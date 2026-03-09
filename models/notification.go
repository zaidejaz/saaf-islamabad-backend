package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationType string

const (
	NotifStatusUpdate NotificationType = "status_update"
	NotifSafetyAlert  NotificationType = "safety_alert"
	NotifNearbyIssue  NotificationType = "nearby_issue"
)

type Notification struct {
	ID        uuid.UUID        `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID    uuid.UUID        `gorm:"type:uuid;not null;index" json:"user_id"`
	User      User             `gorm:"foreignKey:UserID" json:"-"`
	ReportID  *uuid.UUID       `gorm:"type:uuid" json:"report_id,omitempty"`
	Title     string           `gorm:"size:150;not null" json:"title"`
	Message   string           `gorm:"type:text" json:"message"`
	IsRead    bool             `gorm:"default:false" json:"is_read"`
	Type      NotificationType `gorm:"size:30;not null" json:"type"`
	CreatedAt time.Time        `json:"created_at"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}
