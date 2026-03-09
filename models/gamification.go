package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserPoints struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"-"`
	Points    int       `gorm:"not null;default:0" json:"points"`
	Reason    string    `gorm:"type:text" json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (p *UserPoints) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

type Badge struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name        string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
}

func (b *Badge) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

type UserBadge struct {
	ID       uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID   uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	User     User      `gorm:"foreignKey:UserID" json:"-"`
	BadgeID  uuid.UUID `gorm:"type:uuid;not null" json:"badge_id"`
	Badge    Badge     `gorm:"foreignKey:BadgeID" json:"badge,omitempty"`
	EarnedAt time.Time `gorm:"autoCreateTime" json:"earned_at"`
}

func (ub *UserBadge) BeforeCreate(tx *gorm.DB) error {
	if ub.ID == uuid.Nil {
		ub.ID = uuid.New()
	}
	return nil
}
