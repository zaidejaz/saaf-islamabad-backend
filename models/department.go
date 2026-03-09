package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Department struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name         string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Description  string    `gorm:"type:text" json:"description,omitempty"`
	ContactEmail string    `gorm:"size:150" json:"contact_email,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (d *Department) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}
