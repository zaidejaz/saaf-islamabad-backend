package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// CreateReport godoc
// @Summary      Create report
// @Description  Submit a new issue report (citizen)
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

	report := models.Report{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		Address:     req.Address,
		CategoryID:  req.CategoryID,
		Status:      models.StatusSubmitted,
	}

	// Auto-assign department from category default
	if req.CategoryID != nil {
		var cat models.IssueCategory
		if err := database.DB.First(&cat, "id = ?", req.CategoryID).Error; err == nil && cat.DefaultDepartmentID != nil {
			report.DepartmentID = cat.DefaultDepartmentID
		}
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

	// Record initial status history
	database.DB.Create(&models.ReportStatusHistory{
		ReportID:  report.ID,
		ChangedBy: userID,
		NewStatus: string(models.StatusSubmitted),
		Comment:   "Report submitted",
	})

	database.DB.Preload("Category").Preload("Department").Preload("Images").First(&report, "id = ?", report.ID)
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

	// Citizens only see their own reports
	role := c.MustGet("user_role").(models.Role)
	if role == models.RoleCitizen {
		userID := c.MustGet("user_id").(uuid.UUID)
		q = q.Where("user_id = ?", userID)
	}

	q.Count(&total)
	q.Preload("Category").Preload("Department").Preload("Images").
		Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Order("created_at DESC").Find(&reports)

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
	utils.OK(c, report)
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
