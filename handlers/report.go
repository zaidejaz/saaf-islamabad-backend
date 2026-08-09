package handlers

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// CreateReport godoc
// @Summary      Create report
// @Description  Submit a new issue report (citizen). If only a photo (+ location) is provided, AI classifies the issue and fills title, description, category, severity, and priority.
// @Tags         Reports
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateReportRequest  true  "Report data"
// @Success      201   {object}  utils.APIResponse{data=models.Report}
// @Failure      400   {object}  utils.APIResponse
// @Router       /reports [post]
func CreateReport(c *gin.Context) {
	var req CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	if len(req.ImageURLs) > 1 {
		utils.BadRequest(c, "only one image is allowed per report")
		return
	}

	shouldClassify := false
	if req.AutoClassify != nil {
		shouldClassify = *req.AutoClassify
	} else if strings.TrimSpace(req.Title) == "" && req.CategoryID == nil {
		shouldClassify = len(req.ImageURLs) > 0
	}

	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	categoryID := req.CategoryID
	severity := strings.ToLower(strings.TrimSpace(req.SeverityLevel))
	priority := strings.ToLower(strings.TrimSpace(req.PriorityLevel))
	confidence := req.AIConfidenceScore

	if shouldClassify {
		if len(req.ImageURLs) == 0 {
			utils.BadRequest(c, "image_urls is required for AI classification")
			return
		}
		lat, lng := req.Latitude, req.Longitude
		result, err := runClassification(c.Request.Context(), req.ImageURLs, &lat, &lng, req.Address)
		if err != nil {
			writeClassifyError(c, err)
			return
		}
		if !result.IsValidIssue {
			c.JSON(400, gin.H{
				"success": false,
				"message": result.Message,
				"data":    result,
			})
			return
		}
		title = result.Title
		description = result.Description
		categoryID = result.CategoryID
		severity = result.SeverityLevel
		priority = result.PriorityLevel
		confidence = result.AIConfidenceScore
	}

	report := models.Report{
		UserID:            userID,
		Title:             title,
		Description:       description,
		Latitude:          req.Latitude,
		Longitude:         req.Longitude,
		Address:           req.Address,
		CategoryID:        categoryID,
		AIConfidenceScore: confidence,
		Status:            models.StatusSubmitted,
	}
	if severity != "" {
		report.SeverityLevel = models.Severity(severity)
	}
	if priority != "" {
		report.PriorityLevel = models.Priority(priority)
	}

	// Auto-assign department from AI category mapping.
	applyDepartmentFromCategory(&report, categoryID)
	if req.DepartmentID != nil {
		report.DepartmentID = req.DepartmentID
	}

	if err := database.DB.Create(&report).Error; err != nil {
		utils.InternalError(c, "failed to create report")
		return
	}

	for _, url := range req.ImageURLs {
		img := models.ReportImage{
			ReportID: report.ID,
			ImageURL: url,
		}
		database.DB.Create(&img)
	}

	comment := "Report submitted"
	if shouldClassify {
		comment = "Report submitted (AI classified)"
	}
	database.DB.Create(&models.ReportStatusHistory{
		ReportID:  report.ID,
		ChangedBy: userID,
		NewStatus: string(models.StatusSubmitted),
		Comment:   comment,
	})

	database.DB.Preload("Category").Preload("Department").Preload("Images").First(&report, "id = ?", report.ID)

	// Lifecycle: report submitted — confirm to citizen, and ping the active
	// staff of the auto-routed department (if any).
	notifyReportSubmitted(report)
	notifyOtherCitizensOfNewReport(report)

	utils.Created(c, report)
}

// ListReports godoc
// @Summary      List reports
// @Description  Get paginated reports with optional filters
// @Tags         Reports
// @Produce      json
// @Security     BearerAuth
// @Param        page          query  int     false  "Page number"  default(1)
// @Param        page_size     query  int     false  "Items per page"  default(20)
// @Param        status        query  string  false  "Filter by status"
// @Param        category_id   query  string  false  "Filter by category UUID"
// @Param        department_id query  string  false  "Filter by department UUID"
// @Param        severity      query  string  false  "Filter by severity (low, moderate, critical)"
// @Success      200  {object}  utils.PaginatedResponse
// @Router       /reports [get]
func ListReports(c *gin.Context) {
	page, pageSize := utils.GetPagination(c)
	var total int64
	var reports []models.Report

	q := database.DB.Model(&models.Report{})

	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if cid := c.Query("category_id"); cid != "" {
		q = q.Where("category_id = ?", cid)
	}
	if did := c.Query("department_id"); did != "" {
		q = q.Where("department_id = ?", did)
	}
	if sev := c.Query("severity"); sev != "" {
		q = q.Where("severity_level = ?", sev)
	}

	// Citizens see their own reports by default. Pass scope=community to list
	// all citizen reports for the city map / awareness feed.
	// Staff see every report routed to their department.
	role := c.MustGet("user_role").(models.Role)
	if role == models.RoleCitizen && c.Query("scope") != "community" {
		userID := c.MustGet("user_id").(uuid.UUID)
		q = q.Where("user_id = ?", userID)
	}
	if role == models.RoleStaff {
		staff, err := loadStaffUser(c.MustGet("user_id").(uuid.UUID))
		if err != nil || staff.DepartmentID == nil {
			q = q.Where("1 = 0")
		} else {
			q = q.Where("department_id = ?", *staff.DepartmentID)
		}
	}

	q.Count(&total)
	q.Preload("Category").Preload("Department").Preload("Images").
		Preload("StatusHistory").Preload("StatusHistory.Changer").
		Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Order("created_at DESC").Find(&reports)

	if role == models.RoleCitizen && c.Query("scope") == "community" {
		userID := c.MustGet("user_id").(uuid.UUID)
		utils.Paginated(c, redactCommunityReportsForCitizen(userID, reports), page, pageSize, total)
		return
	}

	utils.Paginated(c, reports, page, pageSize, total)
}

// GetReport godoc
// @Summary      Get report
// @Description  Get a single report with full details
// @Tags         Reports
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Report UUID"
// @Success      200  {object}  utils.APIResponse{data=models.Report}
// @Failure      404  {object}  utils.APIResponse
// @Router       /reports/{id} [get]
func GetReport(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid report id")
		return
	}

	var report models.Report
	if err := database.DB.
		Preload("User").Preload("Category").Preload("Department").
		Preload("Images").Preload("StatusHistory").Preload("StatusHistory.Changer").
		First(&report, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "report not found")
		return
	}

	role := c.MustGet("user_role").(models.Role)
	userID := c.MustGet("user_id").(uuid.UUID)
	if role == models.RoleStaff {
		staff, err := loadStaffUser(userID)
		if err != nil || !staffCanAccessReport(staff, report) {
			utils.Forbidden(c, "report is not in your department")
			return
		}
	}

	if role == models.RoleCitizen && !citizenOwnsReport(userID, report) {
		utils.OK(c, toCommunityReportView(report))
		return
	}

	utils.OK(c, report)
}

// UpdateReportStatus godoc
// @Summary      Update report status
// @Description  Change report status (admin/staff)
// @Tags         Reports
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                     true  "Report UUID"
// @Param        body  body  UpdateReportStatusRequest   true  "New status"
// @Success      200   {object}  utils.APIResponse{data=models.Report}
// @Failure      400   {object}  utils.APIResponse
// @Failure      404   {object}  utils.APIResponse
// @Router       /reports/{id}/status [patch]
func UpdateReportStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid report id")
		return
	}

	var req UpdateReportStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var report models.Report
	if err := database.DB.First(&report, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "report not found")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	oldStatus := string(report.Status)
	report.Status = models.ReportStatus(req.Status)
	now := time.Now()
	report.UpdatedAt = &now

	if req.Status == string(models.StatusResolved) {
		report.ResolvedAt = &now
	}

	database.DB.Save(&report)

	database.DB.Create(&models.ReportStatusHistory{
		ReportID:  report.ID,
		ChangedBy: userID,
		OldStatus: oldStatus,
		NewStatus: req.Status,
		Comment:   req.Comment,
	})

	database.DB.Preload("Category").Preload("Department").Preload("Images").First(&report, "id = ?", report.ID)

	// Lifecycle: dispatch a notification appropriate to the new status.
	if oldStatus != req.Status {
		switch models.ReportStatus(req.Status) {
		case models.StatusInProgress:
			notifyReportInProgress(report)
		case models.StatusResolved:
			notifyReportResolved(report)
		case models.StatusRejected:
			notifyReportRejected(report, req.Comment)
		}
	}

	utils.OK(c, report)
}

// UpdateReport godoc
// @Summary      Update report details (staff corrections)
// @Description  Staff may correct report fields for reports in their department
// @Tags         Reports
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string               true  "Report UUID"
// @Param        body  body  UpdateReportRequest  true  "Fields to update"
// @Success      200   {object}  utils.APIResponse{data=models.Report}
// @Router       /reports/{id} [patch]
func UpdateReport(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid report id")
		return
	}

	var req UpdateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	staffID := c.MustGet("user_id").(uuid.UUID)
	staff, err := loadStaffUser(staffID)
	if err != nil {
		utils.Forbidden(c, "staff access required")
		return
	}

	var report models.Report
	if err := database.DB.First(&report, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "report not found")
		return
	}
	if !staffCanAccessReport(staff, report) {
		utils.Forbidden(c, "report is not in your department")
		return
	}
	if report.Status == models.StatusResolved || report.Status == models.StatusRejected {
		utils.BadRequest(c, "resolved or rejected reports cannot be edited")
		return
	}

	if req.Title != nil {
		report.Title = *req.Title
	}
	if req.Description != nil {
		report.Description = *req.Description
	}
	if req.Address != nil {
		report.Address = *req.Address
	}
	if req.Latitude != nil {
		report.Latitude = *req.Latitude
	}
	if req.Longitude != nil {
		report.Longitude = *req.Longitude
	}
	if req.SeverityLevel != nil {
		report.SeverityLevel = models.Severity(*req.SeverityLevel)
	}
	if req.PriorityLevel != nil {
		report.PriorityLevel = models.Priority(*req.PriorityLevel)
	}
	now := time.Now()
	report.UpdatedAt = &now

	if err := database.DB.Save(&report).Error; err != nil {
		utils.InternalError(c, "failed to update report")
		return
	}

	database.DB.Create(&models.ReportStatusHistory{
		ReportID:  report.ID,
		ChangedBy: staffID,
		OldStatus: string(report.Status),
		NewStatus: string(report.Status),
		Comment:   "Staff updated report details",
	})

	database.DB.Preload("Category").Preload("Department").Preload("Images").First(&report, "id = ?", report.ID)
	utils.OK(c, report)
}

// ConfirmStaffReport godoc
// @Summary      Confirm department report (staff)
// @Description  Staff reviews a department report, claims ownership, and marks it ready for worker dispatch
// @Tags         Reports
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Report UUID"
// @Success      200  {object}  utils.APIResponse
// @Router       /reports/{id}/staff-confirm [post]
func ConfirmStaffReport(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid report id")
		return
	}

	staffID := c.MustGet("user_id").(uuid.UUID)
	staff, err := loadStaffUser(staffID)
	if err != nil {
		utils.Forbidden(c, "staff access required")
		return
	}

	var report models.Report
	if err := database.DB.First(&report, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "report not found")
		return
	}
	if !staffCanAccessReport(staff, report) {
		utils.Forbidden(c, "report is not in your department")
		return
	}
	if report.Status == models.StatusResolved || report.Status == models.StatusRejected {
		utils.BadRequest(c, "report is already closed")
		return
	}

	var assignment models.Assignment
	assignErr := database.DB.Where("report_id = ? AND staff_id = ?", report.ID, staffID).First(&assignment).Error
	if assignErr != nil {
		assignment = models.Assignment{
			ReportID:   report.ID,
			StaffID:    staffID,
			AssignedBy: staffID,
		}
		if err := database.DB.Create(&assignment).Error; err != nil {
			utils.InternalError(c, "failed to create assignment")
			return
		}
	}

	now := time.Now()
	oldStatus := string(report.Status)
	if report.Status == models.StatusSubmitted || report.Status == models.StatusInReview {
		report.Status = models.StatusAssigned
		report.UpdatedAt = &now
		database.DB.Save(&report)

		database.DB.Create(&models.ReportStatusHistory{
			ReportID:  report.ID,
			ChangedBy: staffID,
			OldStatus: oldStatus,
			NewStatus: string(models.StatusAssigned),
			Comment:   "Reviewed and confirmed by staff",
		})

		if oldStatus != string(models.StatusAssigned) {
			deptName := ""
			if report.DepartmentID != nil {
				var dept models.Department
				if database.DB.First(&dept, "id = ?", *report.DepartmentID).Error == nil {
					deptName = dept.Name
				}
			}
			notifyReportAssigned(report, staff, deptName)
		}
	} else {
		database.DB.Create(&models.ReportStatusHistory{
			ReportID:  report.ID,
			ChangedBy: staffID,
			OldStatus: oldStatus,
			NewStatus: oldStatus,
			Comment:   "Staff confirmed report for worker dispatch",
		})
	}

	database.DB.Preload("Category").Preload("Department").Preload("Images").First(&report, "id = ?", report.ID)
	database.DB.Preload("Staff").Preload("Worker").Preload("Report").First(&assignment, "id = ?", assignment.ID)

	utils.OK(c, gin.H{
		"report":     report,
		"assignment": assignment,
	})
}

// GetReportStats godoc
// @Summary      Report statistics
// @Description  Get aggregate statistics for the dashboard
// @Tags         Reports
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  utils.APIResponse
// @Router       /reports/stats [get]
func GetReportStats(c *gin.Context) {
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	type SeverityCount struct {
		SeverityLevel string `json:"severity_level"`
		Count         int64  `json:"count"`
	}

	var totalReports int64
	database.DB.Model(&models.Report{}).Count(&totalReports)

	var byStatus []StatusCount
	database.DB.Model(&models.Report{}).Select("status, count(*) as count").Group("status").Scan(&byStatus)

	var bySeverity []SeverityCount
	database.DB.Model(&models.Report{}).Select("severity_level, count(*) as count").
		Where("severity_level IS NOT NULL AND severity_level != ''").Group("severity_level").Scan(&bySeverity)

	var resolvedToday int64
	database.DB.Model(&models.Report{}).Where("status = ? AND resolved_at >= CURRENT_DATE", models.StatusResolved).Count(&resolvedToday)

	utils.OK(c, gin.H{
		"total_reports":  totalReports,
		"by_status":      byStatus,
		"by_severity":    bySeverity,
		"resolved_today": resolvedToday,
	})
}

// DeleteReport godoc
// @Summary      Delete a report (admin)
// @Description  Permanently deletes a report and its related images, status history, assignments, and safety alerts. Notification report links are cleared.
// @Tags         Reports
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  string  true  "Report UUID"
// @Success      200  {object}  utils.APIResponse
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /reports/{id} [delete]
func DeleteReport(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid report id")
		return
	}

	var report models.Report
	if err := database.DB.First(&report, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "report not found")
		return
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		utils.InternalError(c, "failed to delete report")
		return
	}

	if err := tx.Where("report_id = ?", id).Delete(&models.ReportImage{}).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to delete report images")
		return
	}
	if err := tx.Where("report_id = ?", id).Delete(&models.ReportStatusHistory{}).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to delete report history")
		return
	}
	if err := tx.Where("report_id = ?", id).Delete(&models.Assignment{}).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to delete report assignments")
		return
	}
	if err := tx.Where("report_id = ?", id).Delete(&models.SafetyAlert{}).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to delete related safety alerts")
		return
	}
	if err := tx.Model(&models.Notification{}).Where("report_id = ?", id).Update("report_id", nil).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to clear notification links")
		return
	}
	if err := tx.Delete(&report).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to delete report")
		return
	}
	if err := tx.Commit().Error; err != nil {
		utils.InternalError(c, "failed to delete report")
		return
	}

	utils.OK(c, gin.H{
		"message": "report deleted",
		"id":      id,
	})
}
