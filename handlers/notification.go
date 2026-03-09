package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// CreateNotification godoc
// @Summary      Create notification
// @Description  Send a notification to a user (admin/system)
// @Tags         Notifications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateNotificationRequest  true  "Notification data"
// @Success      201   {object}  utils.APIResponse{data=models.Notification}
// @Failure      400   {object}  utils.APIResponse
// @Router       /notifications [post]
func CreateNotification(c *gin.Context) {
	var req CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	notif := models.Notification{
		UserID:   req.UserID,
		ReportID: req.ReportID,
		Title:    req.Title,
		Message:  req.Message,
		Type:     models.NotificationType(req.Type),
	}

	if err := database.DB.Create(&notif).Error; err != nil {
		utils.InternalError(c, "failed to create notification")
		return
	}

	utils.Created(c, notif)
}

// ListMyNotifications godoc
// @Summary      List my notifications
// @Description  Get notifications for the authenticated user
// @Tags         Notifications
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  int   false  "Page number"  default(1)
// @Param        page_size  query  int   false  "Items per page"  default(20)
// @Param        unread     query  bool  false  "Only unread"
// @Success      200  {object}  utils.PaginatedResponse
// @Router       /notifications [get]
func ListMyNotifications(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	page, pageSize := utils.GetPagination(c)
	var total int64
	var notifs []models.Notification

	q := database.DB.Model(&models.Notification{}).Where("user_id = ?", userID)
	if c.Query("unread") == "true" {
		q = q.Where("is_read = false")
	}

	q.Count(&total)
	q.Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Order("created_at DESC").Find(&notifs)

	utils.Paginated(c, notifs, page, pageSize, total)
}

// MarkNotificationRead godoc
// @Summary      Mark notification as read
// @Description  Mark a single notification as read
// @Tags         Notifications
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Notification UUID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /notifications/{id}/read [patch]
func MarkNotificationRead(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid notification id")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	result := database.DB.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true)

	if result.RowsAffected == 0 {
		utils.NotFound(c, "notification not found")
		return
	}

	utils.OK(c, gin.H{"message": "marked as read"})
}

// MarkAllRead godoc
// @Summary      Mark all notifications as read
// @Description  Mark all notifications as read for the authenticated user
// @Tags         Notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  utils.APIResponse
// @Router       /notifications/read-all [patch]
func MarkAllRead(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	database.DB.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Update("is_read", true)

	utils.OK(c, gin.H{"message": "all notifications marked as read"})
}
