package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IssueCategory struct {
	ID                  uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name                string     `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Description         string     `gorm:"type:text" json:"description,omitempty"`
	DefaultDepartmentID *uuid.UUID `gorm:"type:uuid" json:"default_department_id,omitempty"`
	DefaultDepartment   *Department `gorm:"foreignKey:DefaultDepartmentID" json:"default_department,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

func (c *IssueCategory) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
