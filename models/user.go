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
	RoleWorker  Role = "worker"
)

// User represents any account on the platform: citizen, admin, staff, or worker.
//
// Email and Phone are both nullable so that:
//   - Citizens can register with phone-only.
//   - Soft-deleted accounts can have their email/phone scrubbed without
//     destroying historical references (assignments, reports, messages).
//
// Phone is unique only when populated, allowing nullification on soft-delete.
type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	FullName     string     `gorm:"size:100;not null" json:"full_name"`
	Email        *string    `gorm:"size:150;uniqueIndex" json:"email,omitempty"`
	Phone        *string    `gorm:"size:20;uniqueIndex" json:"phone,omitempty"`
	PasswordHash string     `gorm:"type:text;not null" json:"-"`
	Role         Role       `gorm:"size:20;not null;default:'citizen'" json:"role"`
	DepartmentID *uuid.UUID `gorm:"type:uuid" json:"department_id,omitempty"`
	Department   *Department `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	// Workers created by a department staff member; used to scope worker lists and dispatch.
	ManagedByStaffID *uuid.UUID `gorm:"type:uuid;index" json:"managed_by_staff_id,omitempty"`
	// TokenVersion is bumped on admin/staff password reset to invalidate existing JWTs.
	TokenVersion int        `gorm:"not null;default:0" json:"-"`
	IsVerified   bool       `gorm:"default:false" json:"is_verified"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
