package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Assignment struct {
	ID          uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ReportID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"report_id"`
	Report      Report     `gorm:"foreignKey:ReportID" json:"report,omitempty"`
	StaffID     uuid.UUID  `gorm:"type:uuid;not null" json:"staff_id"`
	Staff       User       `gorm:"foreignKey:StaffID" json:"staff,omitempty"`
	AssignedBy  uuid.UUID  `gorm:"type:uuid;not null" json:"assigned_by"`
	Assigner    User       `gorm:"foreignKey:AssignedBy" json:"assigner,omitempty"`
	AssignedAt  time.Time  `gorm:"autoCreateTime" json:"assigned_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Remarks     string     `gorm:"type:text" json:"remarks,omitempty"`
}

func (a *Assignment) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
