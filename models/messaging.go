package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageChannel identifies the allowed communication lanes. Citizens are
// deliberately excluded from in-app messaging.
type MessageChannel string

const (
	ChannelAdminStaff  MessageChannel = "admin_staff"
	ChannelStaffWorker MessageChannel = "staff_worker"
)

// Conversation is a 1:1 chat between two users. The Channel field constrains
// which roles may participate so that we can quickly index / authorize chats
// without re-reading user roles every time.
type Conversation struct {
	ID            uuid.UUID                  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Channel       MessageChannel             `gorm:"size:30;not null;index" json:"channel"`
	Participants  []ConversationParticipant  `gorm:"foreignKey:ConversationID" json:"participants,omitempty"`
	Messages      []Message                  `gorm:"foreignKey:ConversationID" json:"messages,omitempty"`
	LastMessageAt *time.Time                 `gorm:"index" json:"last_message_at,omitempty"`
	CreatedAt     time.Time                  `json:"created_at"`
}

func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

type ConversationParticipant struct {
	ID             uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null;index:idx_conv_user,unique" json:"conversation_id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index:idx_conv_user,unique" json:"user_id"`
	User           User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	JoinedAt       time.Time `gorm:"autoCreateTime" json:"joined_at"`
	LastReadAt     *time.Time `json:"last_read_at,omitempty"`
}

func (p *ConversationParticipant) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

type Message struct {
	ID             uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null;index" json:"conversation_id"`
	SenderID       uuid.UUID `gorm:"type:uuid;not null;index" json:"sender_id"`
	Sender         User      `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
	ReceiverID     uuid.UUID `gorm:"type:uuid;not null;index" json:"receiver_id"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	IsRead         bool      `gorm:"default:false;index" json:"is_read"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	CreatedAt      time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
