package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OfflineSyncLog struct {
	ID               uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID           uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	User             User      `gorm:"foreignKey:UserID" json:"-"`
	LocalReferenceID string    `gorm:"size:255" json:"local_reference_id"`
	SyncedAt         time.Time `gorm:"autoCreateTime" json:"synced_at"`
	Status           string    `gorm:"size:30;not null;default:'pending'" json:"status"`
}

func (o *OfflineSyncLog) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}
