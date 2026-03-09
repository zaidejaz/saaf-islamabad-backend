package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SafetyAlert struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ReportID  uuid.UUID `gorm:"type:uuid;not null;index" json:"report_id"`
	Report    Report    `gorm:"foreignKey:ReportID" json:"report,omitempty"`
	RadiusKM  float64   `gorm:"type:decimal(6,2);not null;default:1.0" json:"radius_km"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *SafetyAlert) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
