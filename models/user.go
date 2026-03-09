package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	RoleCitizen Role = "citizen"
	RoleAdmin   Role = "admin"
	RoleStaff   Role = "staff"
)

type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	FullName     string     `gorm:"size:100;not null" json:"full_name"`
	Email        string     `gorm:"size:150;uniqueIndex;not null" json:"email"`
	Phone        string     `gorm:"size:20" json:"phone,omitempty"`
	PasswordHash string     `gorm:"type:text;not null" json:"-"`
	Role         Role       `gorm:"size:20;not null;default:'citizen'" json:"role"`
	IsVerified   bool       `gorm:"default:false" json:"is_verified"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
