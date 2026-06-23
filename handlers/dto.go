package handlers

import "github.com/google/uuid"

// ── Auth ────────────────────────────────────────────

// RegisterRequest is the payload for POST /auth/register.
//
// Citizens must register with phone + password (Pakistani format).
// Admin / Staff / Worker accounts use email + password.
type RegisterRequest struct {
	FullName string `json:"full_name" binding:"required" example:"Ali Khan"`
	Email    string `json:"email,omitempty" example:"ali@example.com"`
	Phone    string `json:"phone,omitempty" example:"+923001234567"`
	Password string `json:"password" binding:"required,min=6" example:"secret123"`
	Role     string `json:"role" binding:"required,oneof=citizen admin staff worker" example:"citizen"`
	OTP      string `json:"otp,omitempty" example:"123456"`
	DepartmentID *uuid.UUID `json:"department_id,omitempty"`
}

// SendOTPRequest is the payload for POST /auth/otp/send.
type SendOTPRequest struct {
	Phone   string `json:"phone" binding:"required" example:"+923001234567"`
	Purpose string `json:"purpose,omitempty" example:"register"`
}

// LoginRequest accepts either a phone number or an email address. Citizens
// sign in with phone; staff / admin / worker accounts sign in with email.
type LoginRequest struct {
	Identifier string `json:"identifier,omitempty" example:"+923001234567"`
	Email      string `json:"email,omitempty" example:"admin@saafislamabad.pk"`
	Phone      string `json:"phone,omitempty" example:"+923001234567"`
	Password   string `json:"password" binding:"required" example:"secret123"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  UserSummary `json:"user"`
}

type UserSummary struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"full_name"`
	Email    string    `json:"email,omitempty"`
	Phone    string    `json:"phone,omitempty"`
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

// CreateAssignmentRequest is used by admin (assign report -> staff) and
// optionally extended by staff to dispatch the report to a specific worker.
type CreateAssignmentRequest struct {
	ReportID  uuid.UUID  `json:"report_id" binding:"required"`
	StaffID   uuid.UUID  `json:"staff_id" binding:"required"`
	WorkerID  *uuid.UUID `json:"worker_id,omitempty"`
	RouteInfo string     `json:"route_info,omitempty" example:"Approach from Street 7, look for the blue gate"`
	Remarks   string     `json:"remarks" example:"Please resolve within 48 hours"`
}

type CompleteAssignmentRequest struct {
	Remarks string `json:"remarks" example:"Resolved on site"`
}

// AssignWorkerRequest is sent by staff to dispatch a report (already
// assigned to them) to a field worker.
type AssignWorkerRequest struct {
	WorkerID  uuid.UUID `json:"worker_id" binding:"required"`
	RouteInfo string    `json:"route_info,omitempty" example:"Use the back entrance off Margalla Rd"`
	Remarks   string    `json:"remarks,omitempty"`
}

// ── Users (admin) ───────────────────────────────────

// UpdateUserRequest is the admin payload for editing a user. All fields
// are optional; only the supplied ones are mutated.
type UpdateUserRequest struct {
	FullName     *string    `json:"full_name,omitempty" example:"Ali Khan"`
	Email        *string    `json:"email,omitempty" example:"ali@saafislamabad.pk"`
	Phone        *string    `json:"phone,omitempty" example:"+923001234567"`
	Role         *string    `json:"role,omitempty" example:"staff"`
	DepartmentID *uuid.UUID `json:"department_id,omitempty"`
	IsVerified   *bool      `json:"is_verified,omitempty"`
}

// ── Worker ──────────────────────────────────────────

type UpdateWorkerProfileRequest struct {
	FullName *string `json:"full_name,omitempty" example:"Ahmed Raza"`
	Phone    *string `json:"phone,omitempty" example:"+923001234567"`
	Email    *string `json:"email,omitempty" example:"ahmed@saafislamabad.pk"`
}

// ── Notification ────────────────────────────────────

type CreateNotificationRequest struct {
	UserID   uuid.UUID  `json:"user_id" binding:"required"`
	ReportID *uuid.UUID `json:"report_id"`
	Title    string     `json:"title" binding:"required" example:"Status Update"`
	Message  string     `json:"message" binding:"required" example:"Your report is now in review"`
	Type     string     `json:"type" binding:"required,oneof=status_update safety_alert nearby_issue" example:"status_update"`
}

// ── Safety Alert ────────────────────────────────────

type CreateSafetyAlertRequest struct {
	ReportID  uuid.UUID `json:"report_id" binding:"required"`
	RadiusKM  float64   `json:"radius_km" binding:"required,gt=0" example:"2.5"`
	ExpiresIn int       `json:"expires_in_hours" binding:"required,gt=0" example:"24"`
}

// ── Messaging ───────────────────────────────────────

// CreateConversationRequest opens (or returns the existing) 1:1 chat with
// another user. The server validates that the channel pair is allowed.
type CreateConversationRequest struct {
	ParticipantID uuid.UUID `json:"participant_id" binding:"required"`
}

type SendMessageRequest struct {
	Content string `json:"content" binding:"required,min=1,max=5000" example:"Heading to site now."`
}
