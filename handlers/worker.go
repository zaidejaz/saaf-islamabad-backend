package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// allowed image content types for resolution photos.
var allowedImageMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// ListMyWorkerAssignments godoc
// @Summary      List my assignments (worker)
// @Description  Returns all reports dispatched to the authenticated worker, including the report's location and any route info supplied by the dispatching staff.
// @Tags         Worker
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  int     false  "Page number"  default(1)
// @Param        page_size  query  int     false  "Items per page"  default(20)
// @Param        status     query  string  false  "Filter: pending | in_progress | resolved"
// @Success      200  {object}  utils.PaginatedResponse
// @Router       /workers/me/assignments [get]
func ListMyWorkerAssignments(c *gin.Context) {
	workerID := c.MustGet("user_id").(uuid.UUID)
	page, pageSize := utils.GetPagination(c)

	q := database.DB.Model(&models.Assignment{}).Where("worker_id = ?", workerID)

	switch c.Query("status") {
	case "pending":
		q = q.Where("completed_at IS NULL")
	case "in_progress":
		q = q.Where("completed_at IS NULL AND dispatched_at IS NOT NULL")
	case "resolved":
		q = q.Where("completed_at IS NOT NULL")
	}

	var total int64
	q.Count(&total)

	var assignments []models.Assignment
	q.Preload("Report").Preload("Report.Category").Preload("Report.Department").
		Preload("Report.Images").Preload("Staff").
		Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Order("assigned_at DESC").Find(&assignments)

	utils.Paginated(c, assignments, page, pageSize, total)
}

// GetMyWorkerAssignment godoc
// @Summary      Get a worker assignment
// @Description  Fetches a single assignment that belongs to the authenticated worker.
// @Tags         Worker
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Assignment UUID"
// @Success      200  {object}  utils.APIResponse{data=models.Assignment}
// @Failure      404  {object}  utils.APIResponse
// @Router       /workers/me/assignments/{id} [get]
func GetMyWorkerAssignment(c *gin.Context) {
	workerID := c.MustGet("user_id").(uuid.UUID)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid assignment id")
		return
	}

	var a models.Assignment
	if err := database.DB.
		Preload("Report").Preload("Report.Category").Preload("Report.Department").
		Preload("Report.Images").Preload("Staff").
		First(&a, "id = ? AND worker_id = ?", id, workerID).Error; err != nil {
		utils.NotFound(c, "assignment not found")
		return
	}
	utils.OK(c, a)
}

// ResolveWorkerAssignment godoc
// @Summary      Upload resolution image and mark resolved
// @Description  Worker uploads a multipart/form-data image proving the issue is fixed. The endpoint stores the image, attaches it to the report, transitions the report status to `resolved`, and stamps the assignment as completed.
// @Tags         Worker
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string  true   "Assignment UUID"
// @Param        image    formData  file  true   "Resolution image (jpg/png/webp/gif, max 10MB)"
// @Param        remarks  formData  string  false  "Optional notes from the worker"
// @Success      200  {object}  utils.APIResponse{data=models.Assignment}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /workers/me/assignments/{id}/resolve [post]
func ResolveWorkerAssignment(c *gin.Context) {
	workerID := c.MustGet("user_id").(uuid.UUID)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid assignment id")
		return
	}

	fileHeader, err := c.FormFile("image")
	if err != nil {
		utils.BadRequest(c, "image file is required (form field: image)")
		return
	}

	if fileHeader.Size > 10*1024*1024 {
		utils.BadRequest(c, "image must be 10MB or smaller")
		return
	}

	contentType := strings.ToLower(fileHeader.Header.Get("Content-Type"))
	if _, ok := allowedImageMIME[contentType]; !ok {
		if _, _, err := parseImageUpload(fileHeader); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
	}

	var assignment models.Assignment
	if err := database.DB.First(&assignment, "id = ? AND worker_id = ?", id, workerID).Error; err != nil {
		utils.NotFound(c, "assignment not found")
		return
	}
	if assignment.CompletedAt != nil {
		utils.BadRequest(c, "assignment is already resolved")
		return
	}

	filename := fmt.Sprintf("resolutions/resolution_%s_%s", assignment.ID.String(), uuid.NewString())
	publicURL, err := saveUploadedImage(c, fileHeader, filename)
	if badImageUpload(c, err) {
		return
	}
	now := time.Now()
	remarks := strings.TrimSpace(c.PostForm("remarks"))

	tx := database.DB.Begin()

	assignment.ResolutionImageURL = publicURL
	assignment.CompletedAt = &now
	if remarks != "" {
		assignment.Remarks = remarks
	}
	if err := tx.Save(&assignment).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to update assignment")
		return
	}

	// Attach image to the report so it surfaces both on the staff dashboard
	// (via the report.images relation) and on the citizen's report-detail
	// view.
	resolutionImage := models.ReportImage{
		ReportID:     assignment.ReportID,
		ImageURL:     publicURL,
		IsResolution: true,
		UploadedBy:   &workerID,
	}
	if err := tx.Create(&resolutionImage).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to attach resolution image")
		return
	}

	// Transition the report to resolved.
	var report models.Report
	if err := tx.First(&report, "id = ?", assignment.ReportID).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "report not found for assignment")
		return
	}
	oldStatus := string(report.Status)
	report.Status = models.StatusResolved
	report.ResolvedAt = &now
	report.UpdatedAt = &now
	if err := tx.Save(&report).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to update report status")
		return
	}

	if err := tx.Create(&models.ReportStatusHistory{
		ReportID:  report.ID,
		ChangedBy: workerID,
		OldStatus: oldStatus,
		NewStatus: string(models.StatusResolved),
		Comment:   "Resolved on-site by worker",
	}).Error; err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to log status change")
		return
	}

	// Notify the citizen who reported the issue so they can see the fix
	// surface in their app immediately.
	_ = tx.Create(&models.Notification{
		UserID:   report.UserID,
		ReportID: &report.ID,
		Title:    "Your report has been resolved",
		Message:  "A field worker has resolved the issue and uploaded a photo. Open the report to see the resolution.",
		Type:     models.NotifStatusUpdate,
	}).Error

	// Notify the dispatching staff so the staff dashboard reflects the
	// resolution + image immediately.
	_ = tx.Create(&models.Notification{
		UserID:   assignment.StaffID,
		ReportID: &report.ID,
		Title:    "Worker completed an assignment",
		Message:  "The dispatched worker uploaded a resolution image and marked the report resolved.",
		Type:     models.NotifStatusUpdate,
	}).Error

	if err := tx.Commit().Error; err != nil {
		utils.InternalError(c, "failed to commit resolution")
		return
	}

	database.DB.Preload("Report").Preload("Report.Images").Preload("Staff").
		First(&assignment, "id = ?", assignment.ID)

	utils.OK(c, assignment)
}

// GetMyWorkerProfile godoc
// @Summary      Get worker profile
// @Tags         Worker
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  utils.APIResponse{data=models.User}
// @Router       /workers/me/profile [get]
func GetMyWorkerProfile(c *gin.Context) {
	workerID := c.MustGet("user_id").(uuid.UUID)
	var user models.User
	if err := database.DB.Preload("Department").First(&user, "id = ?", workerID).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}
	utils.OK(c, user)
}

// UpdateMyWorkerProfile godoc
// @Summary      Update worker profile
// @Description  Worker self-updates their name / phone / email.
// @Tags         Worker
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      UpdateWorkerProfileRequest  true  "Profile fields"
// @Success      200   {object}  utils.APIResponse{data=models.User}
// @Failure      400   {object}  utils.APIResponse
// @Router       /workers/me/profile [patch]
func UpdateMyWorkerProfile(c *gin.Context) {
	workerID := c.MustGet("user_id").(uuid.UUID)

	var req UpdateWorkerProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", workerID).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	if req.FullName != nil {
		trimmed := strings.TrimSpace(*req.FullName)
		if trimmed == "" {
			utils.BadRequest(c, "full_name cannot be empty")
			return
		}
		user.FullName = trimmed
	}

	if req.Phone != nil {
		if strings.TrimSpace(*req.Phone) == "" {
			user.Phone = nil
		} else {
			phone, err := utils.NormalizePKPhone(*req.Phone)
			if err != nil {
				utils.BadRequest(c, err.Error())
				return
			}
			// Make sure no other user owns this phone.
			var existing models.User
			if err := database.DB.Where("phone = ? AND id <> ?", phone, workerID).First(&existing).Error; err == nil {
				utils.BadRequest(c, "phone already in use")
				return
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				utils.InternalError(c, "failed to check phone uniqueness")
				return
			}
			user.Phone = &phone
		}
	}

	if req.Email != nil {
		trimmed := strings.TrimSpace(*req.Email)
		if trimmed == "" {
			user.Email = nil
		} else {
			if _, err := mail.ParseAddress(trimmed); err != nil {
				utils.BadRequest(c, "invalid email format")
				return
			}
			emailLower := strings.ToLower(trimmed)
			var existing models.User
			if err := database.DB.Where("email = ? AND id <> ?", emailLower, workerID).First(&existing).Error; err == nil {
				utils.BadRequest(c, "email already in use")
				return
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				utils.InternalError(c, "failed to check email uniqueness")
				return
			}
			user.Email = &emailLower
		}
	}

	now := time.Now()
	user.UpdatedAt = &now

	if err := database.DB.Save(&user).Error; err != nil {
		utils.InternalError(c, "failed to update profile")
		return
	}

	utils.OK(c, user)
}

// DeleteWorker godoc
// @Summary      Soft-delete a worker (staff)
// @Description  Staff can remove a worker from their team. The worker is anonymised (email scrubbed, phone nullified, name placeholdered) so historical assignments & analytics still resolve their FK to this user. Allows the email to be reused for re-registration.
// @Tags         Worker
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Worker UUID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /workers/{id} [delete]
func DeleteWorker(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid worker id")
		return
	}

	var worker models.User
	if err := database.DB.First(&worker, "id = ? AND role = ?", id, models.RoleWorker).Error; err != nil {
		utils.NotFound(c, "worker not found")
		return
	}

	role := c.MustGet("user_role").(models.Role)
	if role == models.RoleStaff {
		staffID := c.MustGet("user_id").(uuid.UUID)
		staffDeptID, err := loadStaffDepartmentID(staffID)
		if err != nil {
			utils.Forbidden(c, "staff profile not found")
			return
		}
		if !staffOwnsWorker(staffID, staffDeptID, worker) {
			utils.Forbidden(c, "worker is not on your team")
			return
		}
	}

	if err := softDeleteUser(&worker); err != nil {
		utils.InternalError(c, "failed to delete worker")
		return
	}

	utils.OK(c, gin.H{
		"message": "worker deactivated and anonymised",
		"id":      worker.ID,
	})
}

// ListWorkers godoc
// @Summary      List workers (staff/admin)
// @Description  Active workers, used by staff to dispatch reports.
// @Tags         Worker
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  int  false  "Page number"  default(1)
// @Param        page_size  query  int  false  "Items per page"  default(20)
// @Success      200  {object}  utils.PaginatedResponse
// @Router       /workers [get]
func ListWorkers(c *gin.Context) {
	page, pageSize := utils.GetPagination(c)
	role := c.MustGet("user_role").(models.Role)
	userID := c.MustGet("user_id").(uuid.UUID)

	q := database.DB.Model(&models.User{}).Where("is_active = true AND role = ?", models.RoleWorker)

	switch role {
	case models.RoleStaff:
		staffDeptID, err := loadStaffDepartmentID(userID)
		if err != nil {
			utils.Forbidden(c, "staff profile not found")
			return
		}
		if staffDeptID == nil {
			utils.Paginated(c, []models.User{}, page, pageSize, 0)
			return
		}
		q = q.Where(
			"managed_by_staff_id = ? OR (managed_by_staff_id IS NULL AND department_id = ?)",
			userID, *staffDeptID,
		)
	case models.RoleAdmin:
		if deptID := strings.TrimSpace(c.Query("department_id")); deptID != "" {
			q = q.Where("department_id = ?", deptID)
		}
	}

	var total int64
	q.Count(&total)

	var users []models.User
	q.Preload("Department").Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Order("created_at DESC").Find(&users)

	utils.Paginated(c, users, page, pageSize, total)
}

// CreateWorker godoc
// @Summary      Create a field worker (staff/admin)
// @Description  Staff creates a worker in their department. The worker is linked to that staff member and only they can dispatch them.
// @Tags         Worker
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateWorkerRequest  true  "Worker account"
// @Success      201   {object}  utils.APIResponse{data=models.User}
// @Failure      400   {object}  utils.APIResponse
// @Failure      409   {object}  utils.APIResponse
// @Router       /workers [post]
func CreateWorker(c *gin.Context) {
	var req CreateWorkerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	creatorID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.Role)

	emailLower := strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(emailLower); err != nil {
		utils.BadRequest(c, "invalid email format")
		return
	}
	if exists, err := userExistsByEmail(emailLower); err != nil {
		utils.InternalError(c, "lookup failed")
		return
	} else if exists {
		utils.Error(c, http.StatusConflict, "email already registered")
		return
	}

	worker := models.User{
		FullName:   strings.TrimSpace(req.FullName),
		Email:      &emailLower,
		Role:       models.RoleWorker,
		IsVerified: true,
		IsActive:   true,
	}

	if req.Phone != "" {
		phone, err := utils.NormalizePKPhone(req.Phone)
		if err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		if exists, err := userExistsByPhone(phone); err != nil {
			utils.InternalError(c, "lookup failed")
			return
		} else if exists {
			utils.Error(c, http.StatusConflict, "phone already registered")
			return
		}
		worker.Phone = &phone
	}

	switch role {
	case models.RoleStaff:
		staffDeptID, err := loadStaffDepartmentID(creatorID)
		if err != nil || staffDeptID == nil {
			utils.Forbidden(c, "staff must belong to a department to create workers")
			return
		}
		worker.DepartmentID = staffDeptID
		worker.ManagedByStaffID = &creatorID
	case models.RoleAdmin:
		if req.DepartmentID == nil {
			utils.BadRequest(c, "department_id is required when admin creates a worker")
			return
		}
		if !departmentExists(*req.DepartmentID) {
			utils.BadRequest(c, "department not found")
			return
		}
		worker.DepartmentID = req.DepartmentID
		if staff := activeStaffForDepartment(*req.DepartmentID); staff != nil {
			worker.ManagedByStaffID = &staff.ID
		}
	default:
		utils.Forbidden(c, "only staff or admin can create workers")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalError(c, "failed to hash password")
		return
	}
	worker.PasswordHash = string(hash)

	if err := database.DB.Create(&worker).Error; err != nil {
		utils.InternalError(c, "failed to create worker")
		return
	}

	database.DB.Preload("Department").First(&worker, "id = ?", worker.ID)
	utils.Created(c, worker)
}
