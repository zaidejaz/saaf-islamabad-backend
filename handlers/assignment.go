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
// @Summary      Assign report to staff
// @Description  Create an assignment for a report (admin only)
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
	if err := database.DB.First(&staff, "id = ? AND role = ?", req.StaffID, models.RoleStaff).Error; err != nil {
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
		Remarks:    req.Remarks,
	}

	if err := database.DB.Create(&assignment).Error; err != nil {
		utils.InternalError(c, "failed to create assignment")
		return
	}

	report.Status = models.StatusAssigned
	now := time.Now()
	report.UpdatedAt = &now
	database.DB.Save(&report)

	database.DB.Create(&models.ReportStatusHistory{
		ReportID:  report.ID,
		ChangedBy: adminID,
		OldStatus: string(report.Status),
		NewStatus: string(models.StatusAssigned),
		Comment:   "Assigned to staff: " + staff.FullName,
	})

	database.DB.Preload("Staff").Preload("Assigner").First(&assignment, "id = ?", assignment.ID)
	utils.Created(c, assignment)
}

// ListAssignments godoc
// @Summary      List assignments
// @Description  Get paginated assignments (staff sees own, admin sees all)
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
	if role == models.RoleStaff {
		userID := c.MustGet("user_id").(uuid.UUID)
		q = q.Where("staff_id = ?", userID)
	}

	q.Count(&total)
	q.Preload("Report").Preload("Staff").Preload("Assigner").
		Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Order("assigned_at DESC").Find(&assignments)

	utils.Paginated(c, assignments, page, pageSize, total)
}

// CompleteAssignment godoc
// @Summary      Complete assignment
// @Description  Mark an assignment as completed (staff)
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

	database.DB.Preload("Report").Preload("Staff").First(&assignment, "id = ?", assignment.ID)
	utils.OK(c, assignment)
}
