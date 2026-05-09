package handlers

import (
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"gorm.io/gorm"
)

// ensureStaffDepartmentSlot guarantees that a staff user can occupy the
// given department only if no other active staff currently does.
//
// Pass excludeUserID = nil for new staff (registration). Pass the user's
// own UUID for updates / reactivations so the user doesn't conflict with
// themselves.
//
// Returns nil when the slot is free (or the same user already owns it),
// or a user-facing error when the slot is taken.
func ensureStaffDepartmentSlot(deptID *uuid.UUID, excludeUserID *uuid.UUID) error {
	if deptID == nil {
		return errors.New("department_id is required for staff accounts")
	}

	q := database.DB.Model(&models.User{}).
		Where("role = ? AND is_active = true AND department_id = ?", models.RoleStaff, *deptID)
	if excludeUserID != nil {
		q = q.Where("id <> ?", *excludeUserID)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("this department already has an active staff member")
	}
	return nil
}

// notifyOnce inserts a notification, swallowing errors so a notification
// failure never blocks the underlying business action. We log loudly so
// they're easy to investigate.
func notifyOnce(userID uuid.UUID, reportID *uuid.UUID, title, message string, kind models.NotificationType) {
	n := models.Notification{
		UserID:   userID,
		ReportID: reportID,
		Title:    title,
		Message:  message,
		Type:     kind,
	}
	if err := database.DB.Create(&n).Error; err != nil {
		log.Printf("notification failed (user=%s, title=%q): %v", userID, title, err)
	}
}

// activeStaffForDepartment returns the currently-active staff member for a
// department, or nil if there isn't one. Used to fan out notifications
// when a report is auto-routed to a department.
func activeStaffForDepartment(deptID uuid.UUID) *models.User {
	var staff models.User
	err := database.DB.Where("role = ? AND is_active = true AND department_id = ?", models.RoleStaff, deptID).
		First(&staff).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("active staff lookup failed (dept=%s): %v", deptID, err)
		}
		return nil
	}
	return &staff
}

// notifyReportSubmitted notifies the citizen that their report was
// received, plus the auto-routed department's active staff member (if
// any).
func notifyReportSubmitted(report models.Report) {
	notifyOnce(report.UserID, &report.ID,
		"Report received",
		fmt.Sprintf("We've received your report%s. We'll keep you posted as it moves through the queue.",
			titleSuffix(report.Title)),
		models.NotifStatusUpdate)

	if report.DepartmentID != nil {
		if staff := activeStaffForDepartment(*report.DepartmentID); staff != nil {
			notifyOnce(staff.ID, &report.ID,
				"New report in your department",
				fmt.Sprintf("A citizen submitted a new report%s. Open the dashboard to triage.",
					titleSuffix(report.Title)),
				models.NotifStatusUpdate)
		}
	}
}

// notifyReportAssigned notifies the citizen and the assigned staff member
// when an admin formally assigns a report to a department/staff.
func notifyReportAssigned(report models.Report, staff models.User, departmentName string) {
	deptPart := ""
	if departmentName != "" {
		deptPart = " " + departmentName + " department"
	}
	notifyOnce(report.UserID, &report.ID,
		"Report assigned",
		fmt.Sprintf("Your report%s has been assigned to the%s.",
			titleSuffix(report.Title), deptPart),
		models.NotifStatusUpdate)

	notifyOnce(staff.ID, &report.ID,
		"New assignment",
		fmt.Sprintf("You've been assigned a new report%s.",
			titleSuffix(report.Title)),
		models.NotifStatusUpdate)
}

// notifyReportInProgress notifies the citizen that work has begun.
func notifyReportInProgress(report models.Report) {
	notifyOnce(report.UserID, &report.ID,
		"Work started on your report",
		fmt.Sprintf("Your report%s is now in progress. We'll let you know once it's resolved.",
			titleSuffix(report.Title)),
		models.NotifStatusUpdate)
}

// notifyReportResolved notifies the citizen that the issue is fixed.
func notifyReportResolved(report models.Report) {
	notifyOnce(report.UserID, &report.ID,
		"Your report has been resolved",
		fmt.Sprintf("Your report%s has been marked as resolved. Thanks for helping keep Islamabad clean.",
			titleSuffix(report.Title)),
		models.NotifStatusUpdate)
}

// notifyReportRejected notifies the citizen that their report was closed
// without action.
func notifyReportRejected(report models.Report, comment string) {
	msg := "Your report has been reviewed and closed without action."
	if comment != "" {
		msg = comment
	}
	notifyOnce(report.UserID, &report.ID,
		"Report closed",
		msg,
		models.NotifStatusUpdate)
}

// departmentExists reports whether the given department row exists.
func departmentExists(id uuid.UUID) bool {
	var count int64
	database.DB.Model(&models.Department{}).Where("id = ?", id).Count(&count)
	return count > 0
}

func titleSuffix(title string) string {
	if title == "" {
		return ""
	}
	return " \"" + title + "\""
}
