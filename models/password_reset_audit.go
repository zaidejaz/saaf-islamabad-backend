package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PasswordResetAudit records who reset whose password. Never stores password values.
type PasswordResetAudit struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ActorID   uuid.UUID `gorm:"type:uuid;not null;index" json:"actor_id"`
	TargetID  uuid.UUID `gorm:"type:uuid;not null;index" json:"target_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *PasswordResetAudit) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
