package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Assignment represents the chain admin -> staff -> worker.
//
// StaffID is the department staff who owns the report. WorkerID is the field
// worker dispatched by that staff to physically resolve the issue. WorkerID
// is nullable because admins may assign reports to staff before a worker is
// chosen.
type Assignment struct {
	ID          uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ReportID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"report_id"`
	Report      Report     `gorm:"foreignKey:ReportID" json:"report,omitempty"`
	StaffID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"staff_id"`
	Staff       User       `gorm:"foreignKey:StaffID" json:"staff,omitempty"`
	WorkerID    *uuid.UUID `gorm:"type:uuid;index" json:"worker_id,omitempty"`
	Worker      *User      `gorm:"foreignKey:WorkerID" json:"worker,omitempty"`
	AssignedBy  uuid.UUID  `gorm:"type:uuid;not null" json:"assigned_by"`
	Assigner    User       `gorm:"foreignKey:AssignedBy" json:"assigner,omitempty"`
	AssignedAt  time.Time  `gorm:"autoCreateTime" json:"assigned_at"`
	DispatchedAt *time.Time `json:"dispatched_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Route / location info delivered to the worker. Free-form so staff can
	// drop a Google Maps link, written directions, or a meeting point.
	RouteInfo string `gorm:"type:text" json:"route_info,omitempty"`

	// Latitude/Longitude duplicated from Report at dispatch time so the
	// worker has an authoritative pin even if the report is later edited.
	RouteLatitude  *float64 `gorm:"type:decimal(9,6)" json:"route_latitude,omitempty"`
	RouteLongitude *float64 `gorm:"type:decimal(9,6)" json:"route_longitude,omitempty"`

	// URL of the resolution image uploaded by the worker once the issue
	// is fixed on-site. Mirrored onto the report's image set as well.
	ResolutionImageURL string `gorm:"type:text" json:"resolution_image_url,omitempty"`

	Remarks string `gorm:"type:text" json:"remarks,omitempty"`
}

func (a *Assignment) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
