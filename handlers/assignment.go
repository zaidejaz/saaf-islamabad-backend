package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// CreateAssignment godoc
// @Summary      Assign report to staff (admin)
// @Description  Creates the staff-level ownership of a report. Optionally a worker can be dispatched immediately by including `worker_id`.
// @Tags         Assignments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateAssignmentRequest  true  "Assignment data"
// @Success      201   {object}  utils.APIResponse{data=models.Assignment}
// @Failure      400   {object}  utils.APIResponse
// @Router       /assignments [post]
func CreateAssignment(c *gin.Context) {
	var req CreateAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	adminID := c.MustGet("user_id").(uuid.UUID)

	var staff models.User
	if err := database.DB.First(&staff, "id = ? AND role = ? AND is_active = true", req.StaffID, models.RoleStaff).Error; err != nil {
		utils.BadRequest(c, "staff member not found")
		return
	}

	var report models.Report
	if err := database.DB.First(&report, "id = ?", req.ReportID).Error; err != nil {
		utils.BadRequest(c, "report not found")
		return
	}

	assignment := models.Assignment{
		ReportID:   req.ReportID,
		StaffID:    req.StaffID,
		AssignedBy: adminID,
		RouteInfo:  req.RouteInfo,
		Remarks:    req.Remarks,
	}

	now := time.Now()

	// Optional immediate dispatch to a worker.
	if req.WorkerID != nil {
		var worker models.User
		if err := database.DB.First(&worker, "id = ? AND role = ? AND is_active = true", *req.WorkerID, models.RoleWorker).Error; err != nil {
			utils.BadRequest(c, "worker not found")
			return
		}
		assignment.WorkerID = req.WorkerID
		assignment.DispatchedAt = &now
		assignment.RouteLatitude = &report.Latitude
		assignment.RouteLongitude = &report.Longitude
	}

	if err := database.DB.Create(&assignment).Error; err != nil {
		utils.InternalError(c, "failed to create assignment")
		return
	}

	oldStatus := string(report.Status)
	report.Status = models.StatusAssigned
	report.UpdatedAt = &now
	database.DB.Save(&report)

	database.DB.Create(&models.ReportStatusHistory{
		ReportID:  report.ID,
		ChangedBy: adminID,
		OldStatus: oldStatus,
		NewStatus: string(models.StatusAssigned),
		Comment:   "Assigned to staff: " + staff.FullName,
	})

	database.DB.Preload("Staff").Preload("Worker").Preload("Assigner").Preload("Report").
		First(&assignment, "id = ?", assignment.ID)
	utils.Created(c, assignment)
}

// AssignWorker godoc
// @Summary      Dispatch worker for an assignment (staff)
// @Description  The staff owner of an assignment dispatches a worker to physically resolve the report on site. Sets the worker, route info and a location pin.
// @Tags         Assignments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                true  "Assignment UUID"
// @Param        body  body  AssignWorkerRequest    true  "Worker dispatch payload"
// @Success      200   {object}  utils.APIResponse{data=models.Assignment}
// @Failure      400   {object}  utils.APIResponse
// @Failure      404   {object}  utils.APIResponse
// @Router       /assignments/{id}/assign-worker [patch]
func AssignWorker(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid assignment id")
		return
	}

	var req AssignWorkerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	staffID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.Role)

	var assignment models.Assignment
	q := database.DB.Where("id = ?", id)
	// Staff can only dispatch their own assignments.
	if role == models.RoleStaff {
		q = q.Where("staff_id = ?", staffID)
	}
	if err := q.First(&assignment).Error; err != nil {
		utils.NotFound(c, "assignment not found")
		return
	}

	var worker models.User
	if err := database.DB.First(&worker, "id = ? AND role = ? AND is_active = true", req.WorkerID, models.RoleWorker).Error; err != nil {
		utils.BadRequest(c, "worker not found")
		return
	}

	var report models.Report
	if err := database.DB.First(&report, "id = ?", assignment.ReportID).Error; err != nil {
		utils.InternalError(c, "report not found for assignment")
		return
	}

	now := time.Now()
	assignment.WorkerID = &req.WorkerID
	assignment.DispatchedAt = &now
	assignment.RouteInfo = req.RouteInfo
	assignment.RouteLatitude = &report.Latitude
	assignment.RouteLongitude = &report.Longitude
	if req.Remarks != "" {
		assignment.Remarks = req.Remarks
	}

	if err := database.DB.Save(&assignment).Error; err != nil {
		utils.InternalError(c, "failed to dispatch worker")
		return
	}

	// Move the report to in_progress when a worker is in the field.
	oldStatus := string(report.Status)
	if report.Status != models.StatusInProgress && report.Status != models.StatusResolved {
		report.Status = models.StatusInProgress
		report.UpdatedAt = &now
		database.DB.Save(&report)

		database.DB.Create(&models.ReportStatusHistory{
			ReportID:  report.ID,
			ChangedBy: staffID,
			OldStatus: oldStatus,
			NewStatus: string(models.StatusInProgress),
			Comment:   "Worker dispatched: " + worker.FullName,
		})
	}

	// Notify the worker so their app surfaces the new assignment instantly.
	_ = database.DB.Create(&models.Notification{
		UserID:   worker.ID,
		ReportID: &report.ID,
		Title:    "New assignment",
		Message:  "A new report has been dispatched to you. Open the worker app to see route details.",
		Type:     models.NotifStatusUpdate,
	}).Error

	database.DB.Preload("Staff").Preload("Worker").Preload("Report").
		First(&assignment, "id = ?", assignment.ID)
	utils.OK(c, assignment)
}

// ListAssignments godoc
// @Summary      List assignments
// @Description  Get paginated assignments. Staff sees own, worker sees own dispatched, admin sees all.
// @Tags         Assignments
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  int  false  "Page number"  default(1)
// @Param        page_size  query  int  false  "Items per page"  default(20)
// @Success      200  {object}  utils.PaginatedResponse
// @Router       /assignments [get]
func ListAssignments(c *gin.Context) {
	page, pageSize := utils.GetPagination(c)
	var total int64
	var assignments []models.Assignment

	q := database.DB.Model(&models.Assignment{})

	role := c.MustGet("user_role").(models.Role)
	userID := c.MustGet("user_id").(uuid.UUID)

	switch role {
	case models.RoleStaff:
		q = q.Where("staff_id = ?", userID)
	case models.RoleWorker:
		q = q.Where("worker_id = ?", userID)
	}

	q.Count(&total)
	q.Preload("Report").Preload("Staff").Preload("Worker").Preload("Assigner").
		Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Order("assigned_at DESC").Find(&assignments)

	utils.Paginated(c, assignments, page, pageSize, total)
}

// CompleteAssignment godoc
// @Summary      Complete assignment (staff)
// @Description  Mark an assignment as completed without going through the worker upload flow (e.g. desk-resolved).
// @Tags         Assignments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                     true  "Assignment UUID"
// @Param        body  body  CompleteAssignmentRequest   true  "Completion details"
// @Success      200   {object}  utils.APIResponse{data=models.Assignment}
// @Failure      404   {object}  utils.APIResponse
// @Router       /assignments/{id}/complete [patch]
func CompleteAssignment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid assignment id")
		return
	}

	var req CompleteAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var assignment models.Assignment
	if err := database.DB.First(&assignment, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "assignment not found")
		return
	}

	now := time.Now()
	assignment.CompletedAt = &now
	if req.Remarks != "" {
		assignment.Remarks = req.Remarks
	}
	database.DB.Save(&assignment)

	var report models.Report
	if err := database.DB.First(&report, "id = ?", assignment.ReportID).Error; err == nil {
		userID := c.MustGet("user_id").(uuid.UUID)
		oldStatus := string(report.Status)
		report.Status = models.StatusResolved
		report.ResolvedAt = &now
		report.UpdatedAt = &now
		database.DB.Save(&report)

		database.DB.Create(&models.ReportStatusHistory{
			ReportID:  report.ID,
			ChangedBy: userID,
			OldStatus: oldStatus,
			NewStatus: string(models.StatusResolved),
			Comment:   "Assignment completed",
		})
	}

	database.DB.Preload("Report").Preload("Staff").Preload("Worker").First(&assignment, "id = ?", assignment.ID)
	utils.OK(c, assignment)
}
