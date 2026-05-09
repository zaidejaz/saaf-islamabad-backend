package handlers

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/messaging"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
	"gorm.io/gorm"
)

// channelFor returns the allowed messaging channel for a pair of roles, or
// false when the pair is not allowed to chat. Citizens are deliberately
// excluded (the citizen app has no access to this module).
func channelFor(a, b models.Role) (models.MessageChannel, bool) {
	switch {
	case (a == models.RoleAdmin && b == models.RoleStaff) || (a == models.RoleStaff && b == models.RoleAdmin):
		return models.ChannelAdminStaff, true
	case (a == models.RoleStaff && b == models.RoleWorker) || (a == models.RoleWorker && b == models.RoleStaff):
		return models.ChannelStaffWorker, true
	}
	return "", false
}

// CreateConversation godoc
// @Summary      Open or fetch a 1:1 conversation
// @Description  Returns the existing chat with the participant if one exists, otherwise creates a new conversation. Allowed pairs: admin↔staff and staff↔worker. Citizens are not permitted.
// @Tags         Messaging
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateConversationRequest  true  "Participant"
// @Success      200   {object}  utils.APIResponse{data=models.Conversation}
// @Failure      400   {object}  utils.APIResponse
// @Failure      403   {object}  utils.APIResponse
// @Router       /conversations [post]
func CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	meID := c.MustGet("user_id").(uuid.UUID)
	myRole := c.MustGet("user_role").(models.Role)

	if req.ParticipantID == meID {
		utils.BadRequest(c, "cannot start a conversation with yourself")
		return
	}

	var other models.User
	if err := database.DB.First(&other, "id = ? AND is_active = true", req.ParticipantID).Error; err != nil {
		utils.NotFound(c, "participant not found")
		return
	}

	channel, ok := channelFor(myRole, other.Role)
	if !ok {
		utils.Forbidden(c, "messaging is not allowed between these roles")
		return
	}

	conv, err := findOrCreateConversation(meID, other.ID, channel)
	if err != nil {
		utils.InternalError(c, "failed to open conversation")
		return
	}

	database.DB.Preload("Participants").Preload("Participants.User").
		First(&conv, "id = ?", conv.ID)

	utils.OK(c, conv)
}

// findOrCreateConversation returns the unique 1:1 conversation between two
// users for a given channel. Conversations are not duplicated.
func findOrCreateConversation(a, b uuid.UUID, channel models.MessageChannel) (models.Conversation, error) {
	var conv models.Conversation

	// We look for a conversation that contains both users on this channel.
	err := database.DB.
		Joins("JOIN conversation_participants p1 ON p1.conversation_id = conversations.id AND p1.user_id = ?", a).
		Joins("JOIN conversation_participants p2 ON p2.conversation_id = conversations.id AND p2.user_id = ?", b).
		Where("conversations.channel = ?", channel).
		First(&conv).Error
	if err == nil {
		return conv, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return conv, err
	}

	conv = models.Conversation{Channel: channel}
	tx := database.DB.Begin()
	if err := tx.Create(&conv).Error; err != nil {
		tx.Rollback()
		return conv, err
	}
	parts := []models.ConversationParticipant{
		{ConversationID: conv.ID, UserID: a},
		{ConversationID: conv.ID, UserID: b},
	}
	if err := tx.Create(&parts).Error; err != nil {
		tx.Rollback()
		return conv, err
	}
	if err := tx.Commit().Error; err != nil {
		return conv, err
	}
	return conv, nil
}

// ListConversations godoc
// @Summary      List my conversations
// @Description  All chats the authenticated non-citizen user is part of, most recently active first.
// @Tags         Messaging
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  int  false  "Page number"  default(1)
// @Param        page_size  query  int  false  "Items per page"  default(20)
// @Success      200  {object}  utils.PaginatedResponse
// @Router       /conversations [get]
func ListConversations(c *gin.Context) {
	meID := c.MustGet("user_id").(uuid.UUID)
	page, pageSize := utils.GetPagination(c)

	var convIDs []uuid.UUID
	database.DB.Model(&models.ConversationParticipant{}).
		Where("user_id = ?", meID).Pluck("conversation_id", &convIDs)

	if len(convIDs) == 0 {
		utils.Paginated(c, []models.Conversation{}, page, pageSize, 0)
		return
	}

	var total int64
	q := database.DB.Model(&models.Conversation{}).Where("id IN ?", convIDs)
	q.Count(&total)

	var convs []models.Conversation
	q.Preload("Participants").Preload("Participants.User").
		Order("last_message_at DESC NULLS LAST, created_at DESC").
		Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Find(&convs)

	utils.Paginated(c, convs, page, pageSize, total)
}

// SendMessage godoc
// @Summary      Send a message in a conversation
// @Description  Persists the message and pushes it to any websocket subscribers of the receiver in real time. The HTTP endpoint also doubles as the polling fallback when websockets aren't available.
// @Tags         Messaging
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string              true  "Conversation UUID"
// @Param        body  body  SendMessageRequest   true  "Message body"
// @Success      201   {object}  utils.APIResponse{data=models.Message}
// @Failure      400   {object}  utils.APIResponse
// @Failure      403   {object}  utils.APIResponse
// @Router       /conversations/{id}/messages [post]
func SendMessage(c *gin.Context) {
	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid conversation id")
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		utils.BadRequest(c, "content cannot be empty")
		return
	}

	meID := c.MustGet("user_id").(uuid.UUID)

	var conv models.Conversation
	if err := database.DB.Preload("Participants").First(&conv, "id = ?", convID).Error; err != nil {
		utils.NotFound(c, "conversation not found")
		return
	}

	var receiverID uuid.UUID
	isParticipant := false
	for _, p := range conv.Participants {
		if p.UserID == meID {
			isParticipant = true
		} else {
			receiverID = p.UserID
		}
	}
	if !isParticipant {
		utils.Forbidden(c, "you are not part of this conversation")
		return
	}
	if receiverID == uuid.Nil {
		utils.InternalError(c, "conversation is missing a receiver")
		return
	}

	now := time.Now()
	msg := models.Message{
		ConversationID: conv.ID,
		SenderID:       meID,
		ReceiverID:     receiverID,
		Content:        content,
	}
	if err := database.DB.Create(&msg).Error; err != nil {
		utils.InternalError(c, "failed to send message")
		return
	}

	database.DB.Model(&conv).Update("last_message_at", &now)
	database.DB.Preload("Sender").First(&msg, "id = ?", msg.ID)

	// Real-time push to receiver (and echo to sender for multi-device sync).
	messaging.Default.Publish(receiverID, messaging.OutboundEvent{Type: "message", Payload: msg})
	messaging.Default.Publish(meID, messaging.OutboundEvent{Type: "message_sent", Payload: msg})

	utils.Created(c, msg)
}

// ListMessages godoc
// @Summary      Fetch message history (paginated)
// @Description  Returns messages newest-first. Use as the polling fallback when websockets are unavailable.
// @Tags         Messaging
// @Produce      json
// @Security     BearerAuth
// @Param        id         path   string  true   "Conversation UUID"
// @Param        page       query  int     false  "Page number"  default(1)
// @Param        page_size  query  int     false  "Items per page"  default(50)
// @Success      200  {object}  utils.PaginatedResponse
// @Failure      403  {object}  utils.APIResponse
// @Router       /conversations/{id}/messages [get]
func ListMessages(c *gin.Context) {
	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid conversation id")
		return
	}

	meID := c.MustGet("user_id").(uuid.UUID)

	var part models.ConversationParticipant
	if err := database.DB.First(&part, "conversation_id = ? AND user_id = ?", convID, meID).Error; err != nil {
		utils.Forbidden(c, "you are not part of this conversation")
		return
	}

	page, pageSize := utils.GetPagination(c)
	var total int64
	database.DB.Model(&models.Message{}).Where("conversation_id = ?", convID).Count(&total)

	var msgs []models.Message
	database.DB.Where("conversation_id = ?", convID).
		Preload("Sender").
		Order("created_at DESC").
		Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Find(&msgs)

	utils.Paginated(c, msgs, page, pageSize, total)
}

// MarkConversationRead godoc
// @Summary      Mark all messages in a conversation as read
// @Tags         Messaging
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Conversation UUID"
// @Success      200  {object}  utils.APIResponse
// @Router       /conversations/{id}/read [patch]
func MarkConversationRead(c *gin.Context) {
	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid conversation id")
		return
	}

	meID := c.MustGet("user_id").(uuid.UUID)
	now := time.Now()

	res := database.DB.Model(&models.ConversationParticipant{}).
		Where("conversation_id = ? AND user_id = ?", convID, meID).
		Update("last_read_at", &now)
	if res.RowsAffected == 0 {
		utils.Forbidden(c, "you are not part of this conversation")
		return
	}

	database.DB.Model(&models.Message{}).
		Where("conversation_id = ? AND receiver_id = ? AND is_read = false", convID, meID).
		Updates(map[string]interface{}{"is_read": true, "read_at": &now})

	utils.OK(c, gin.H{"message": "marked as read"})
}
