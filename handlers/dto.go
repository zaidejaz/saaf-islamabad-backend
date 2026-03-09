package handlers

import "github.com/google/uuid"

// ── Auth ────────────────────────────────────────────

type RegisterRequest struct {
	FullName string `json:"full_name" binding:"required" example:"Ali Khan"`
	Email    string `json:"email" binding:"required,email" example:"ali@example.com"`
	Phone    string `json:"phone" example:"+923001234567"`
	Password string `json:"password" binding:"required,min=6" example:"secret123"`
	Role     string `json:"role" binding:"required,oneof=citizen admin staff" example:"citizen"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"ali@example.com"`
	Password string `json:"password" binding:"required" example:"secret123"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  UserSummary `json:"user"`
}

type UserSummary struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"full_name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
}

// ── Department ──────────────────────────────────────

type CreateDepartmentRequest struct {
	Name         string `json:"name" binding:"required" example:"Waste Management"`
	Description  string `json:"description" example:"Handles all waste-related issues"`
	ContactEmail string `json:"contact_email" example:"waste@islamabad.gov"`
}

type UpdateDepartmentRequest struct {
	Name         *string `json:"name" example:"Waste Management"`
	Description  *string `json:"description"`
	ContactEmail *string `json:"contact_email"`
}

// ── Category ────────────────────────────────────────

type CreateCategoryRequest struct {
	Name                string     `json:"name" binding:"required" example:"Broken Street Light"`
	Description         string     `json:"description" example:"Streetlight not working"`
	DefaultDepartmentID *uuid.UUID `json:"default_department_id"`
}

type UpdateCategoryRequest struct {
	Name                *string    `json:"name"`
	Description         *string    `json:"description"`
	DefaultDepartmentID *uuid.UUID `json:"default_department_id"`
}

// ── Report ──────────────────────────────────────────

type CreateReportRequest struct {
	Title       string     `json:"title" example:"Garbage pile on Street 5"`
	Description string     `json:"description" example:"Large pile of uncollected garbage"`
	Latitude    float64    `json:"latitude" binding:"required" example:"33.6844"`
	Longitude   float64    `json:"longitude" binding:"required" example:"73.0479"`
	Address     string     `json:"address" example:"Street 5, G-9, Islamabad"`
	CategoryID  *uuid.UUID `json:"category_id"`
	ImageURLs   []string   `json:"image_urls"`
}

type UpdateReportStatusRequest struct {
	Status  string `json:"status" binding:"required,oneof=submitted in_review assigned in_progress resolved rejected" example:"in_review"`
	Comment string `json:"comment" example:"Issue is being reviewed"`
}

// ── Assignment ──────────────────────────────────────

type CreateAssignmentRequest struct {
	ReportID uuid.UUID `json:"report_id" binding:"required"`
	StaffID  uuid.UUID `json:"staff_id" binding:"required"`
	Remarks  string    `json:"remarks" example:"Please resolve within 48 hours"`
}

type CompleteAssignmentRequest struct {
	Remarks string `json:"remarks" example:"Resolved on site"`
}

// ── Notification ────────────────────────────────────

type CreateNotificationRequest struct {
	UserID   uuid.UUID `json:"user_id" binding:"required"`
	ReportID *uuid.UUID `json:"report_id"`
	Title    string    `json:"title" binding:"required" example:"Status Update"`
	Message  string    `json:"message" binding:"required" example:"Your report is now in review"`
	Type     string    `json:"type" binding:"required,oneof=status_update safety_alert nearby_issue" example:"status_update"`
}

// ── Safety Alert ────────────────────────────────────

type CreateSafetyAlertRequest struct {
	ReportID  uuid.UUID `json:"report_id" binding:"required"`
	RadiusKM  float64   `json:"radius_km" binding:"required,gt=0" example:"2.5"`
	ExpiresIn int       `json:"expires_in_hours" binding:"required,gt=0" example:"24"`
}
