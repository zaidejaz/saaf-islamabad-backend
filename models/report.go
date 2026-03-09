package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityModerate Severity = "moderate"
	SeverityCritical Severity = "critical"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

type ReportStatus string

const (
	StatusSubmitted  ReportStatus = "submitted"
	StatusInReview   ReportStatus = "in_review"
	StatusAssigned   ReportStatus = "assigned"
	StatusInProgress ReportStatus = "in_progress"
	StatusResolved   ReportStatus = "resolved"
	StatusRejected   ReportStatus = "rejected"
)

type Report struct {
	ID                uuid.UUID    `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID            uuid.UUID    `gorm:"type:uuid;not null" json:"user_id"`
	User              User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CategoryID        *uuid.UUID   `gorm:"type:uuid" json:"category_id,omitempty"`
	Category          *IssueCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	DepartmentID      *uuid.UUID   `gorm:"type:uuid" json:"department_id,omitempty"`
	Department        *Department  `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	Title             string       `gorm:"size:150" json:"title,omitempty"`
	Description       string       `gorm:"type:text" json:"description,omitempty"`
	Latitude          float64      `gorm:"type:decimal(9,6);not null" json:"latitude"`
	Longitude         float64      `gorm:"type:decimal(9,6);not null" json:"longitude"`
	Address           string       `gorm:"type:text" json:"address,omitempty"`
	SeverityLevel     Severity     `gorm:"size:20" json:"severity_level,omitempty"`
	PriorityLevel     Priority     `gorm:"size:20" json:"priority_level,omitempty"`
	Status            ReportStatus `gorm:"size:20;not null;default:'submitted'" json:"status"`
	AIConfidenceScore *float64     `gorm:"type:decimal(5,2)" json:"ai_confidence_score,omitempty"`
	IsDuplicate       bool         `gorm:"default:false" json:"is_duplicate"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         *time.Time   `json:"updated_at,omitempty"`
	ResolvedAt        *time.Time   `json:"resolved_at,omitempty"`

	Images        []ReportImage         `gorm:"foreignKey:ReportID" json:"images,omitempty"`
	StatusHistory []ReportStatusHistory  `gorm:"foreignKey:ReportID" json:"status_history,omitempty"`
}

func (r *Report) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

type ReportImage struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ReportID  uuid.UUID `gorm:"type:uuid;not null;index" json:"report_id"`
	ImageURL  string    `gorm:"type:text;not null" json:"image_url"`
	IsPrimary bool      `gorm:"default:false" json:"is_primary"`
	CreatedAt time.Time `json:"created_at"`
}

func (ri *ReportImage) BeforeCreate(tx *gorm.DB) error {
	if ri.ID == uuid.Nil {
		ri.ID = uuid.New()
	}
	return nil
}

type ReportStatusHistory struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ReportID  uuid.UUID `gorm:"type:uuid;not null;index" json:"report_id"`
	ChangedBy uuid.UUID `gorm:"type:uuid;not null" json:"changed_by"`
	Changer   User      `gorm:"foreignKey:ChangedBy" json:"changer,omitempty"`
	OldStatus string    `gorm:"size:30" json:"old_status"`
	NewStatus string    `gorm:"size:30;not null" json:"new_status"`
	Comment   string    `gorm:"type:text" json:"comment,omitempty"`
	ChangedAt time.Time `gorm:"autoCreateTime" json:"changed_at"`
}

func (h *ReportStatusHistory) BeforeCreate(tx *gorm.DB) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	return nil
}
