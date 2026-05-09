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
	"gorm.io/gorm"
)

// ListUsers godoc
// @Summary      List users (admin)
// @Description  Paginated list of users. Includes the linked department object (not just the id) for staff. Filter by role with `?role=`. Soft-deleted accounts are excluded by default; pass `?include_inactive=true` to include them.
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        page              query  int     false  "Page number"           default(1)
// @Param        page_size         query  int     false  "Items per page"        default(20)
// @Param        role              query  string  false  "Filter by role (citizen, admin, staff, worker)"
// @Param        include_inactive  query  bool    false  "Include soft-deleted users"
// @Success      200  {object}  utils.PaginatedResponse
// @Router       /users [get]
func ListUsers(c *gin.Context) {
	page, pageSize := utils.GetPagination(c)
	var total int64
	var users []models.User

	q := database.DB.Model(&models.User{})

	if c.Query("include_inactive") != "true" {
		q = q.Where("is_active = true")
	}
	if role := c.Query("role"); role != "" {
		q = q.Where("role = ?", role)
	}

	q.Count(&total)
	q.Preload("Department").Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Order("created_at DESC").Find(&users)

	utils.Paginated(c, users, page, pageSize, total)
}

// GetUser godoc
// @Summary      Get user by ID (admin)
// @Description  Includes the linked department object (not just the id).
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        id               path   string  true   "User UUID"
// @Param        include_inactive query  bool    false  "Allow fetching soft-deleted users"
// @Success      200  {object}  utils.APIResponse{data=models.User}
// @Failure      404  {object}  utils.APIResponse
// @Router       /users/{id} [get]
func GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}

	q := database.DB.Preload("Department")
	if c.Query("include_inactive") != "true" {
		q = q.Where("is_active = true")
	}

	var user models.User
	if err := q.First(&user, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	utils.OK(c, user)
}

// UpdateUser godoc
// @Summary      Update a user (admin)
// @Description  Edit any subset of `full_name`, `email`, `phone`, `role`, `department_id`, `is_verified`. The "one active staff per department" rule is enforced: changing a user's role to `staff` and/or moving an active staff member to a new department is rejected if that department already has another active staff member. Cannot change a user's `is_active` flag — use the dedicated soft-delete and reactivate endpoints for that.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string             true  "User UUID"
// @Param        body  body  UpdateUserRequest  true  "Fields to update"
// @Success      200   {object}  utils.APIResponse{data=models.User}
// @Failure      400   {object}  utils.APIResponse
// @Failure      404   {object}  utils.APIResponse
// @Failure      409   {object}  utils.APIResponse
// @Router       /users/{id} [patch]
func UpdateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
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
			if err := assertUserFieldUnique("email", emailLower, user.ID); err != nil {
				utils.Error(c, http.StatusConflict, err.Error())
				return
			}
			user.Email = &emailLower
		}
	}

	if req.Phone != nil {
		trimmed := strings.TrimSpace(*req.Phone)
		if trimmed == "" {
			user.Phone = nil
		} else {
			phone, err := utils.NormalizePKPhone(trimmed)
			if err != nil {
				utils.BadRequest(c, err.Error())
				return
			}
			if err := assertUserFieldUnique("phone", phone, user.ID); err != nil {
				utils.Error(c, http.StatusConflict, err.Error())
				return
			}
			user.Phone = &phone
		}
	}

	if req.Role != nil {
		newRole := models.Role(*req.Role)
		switch newRole {
		case models.RoleCitizen, models.RoleAdmin, models.RoleStaff, models.RoleWorker:
			user.Role = newRole
		default:
			utils.BadRequest(c, "invalid role")
			return
		}
	}

	if req.DepartmentID != nil {
		// Empty UUID means "clear the department". Otherwise validate it.
		if *req.DepartmentID == uuid.Nil {
			user.DepartmentID = nil
		} else {
			if !departmentExists(*req.DepartmentID) {
				utils.BadRequest(c, "department not found")
				return
			}
			user.DepartmentID = req.DepartmentID
		}
	}

	if req.IsVerified != nil {
		user.IsVerified = *req.IsVerified
	}

	// "One active staff per department" — applies whenever the user ends
	// up as an *active* staff member after this update.
	if user.Role == models.RoleStaff && user.IsActive {
		if err := ensureStaffDepartmentSlot(user.DepartmentID, &user.ID); err != nil {
			utils.Error(c, http.StatusConflict, err.Error())
			return
		}
	}

	now := time.Now()
	user.UpdatedAt = &now
	if err := database.DB.Save(&user).Error; err != nil {
		utils.InternalError(c, "failed to update user")
		return
	}

	database.DB.Preload("Department").First(&user, "id = ?", user.ID)
	utils.OK(c, user)
}

// DeactivateUser godoc
// @Summary      Soft-delete a user (admin)
// @Description  Anonymises the user's email + name and marks the account inactive. Linked records (reports, assignments, messages, analytics) remain queryable. Admins use this for staff/citizens; staff use DELETE /workers/{id} for workers.
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "User UUID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /users/{id} [delete]
func DeactivateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	if user.Role == models.RoleAdmin {
		utils.Forbidden(c, "cannot soft-delete an admin account via this endpoint")
		return
	}

	if err := softDeleteUser(&user); err != nil {
		utils.InternalError(c, "failed to deactivate user")
		return
	}

	utils.OK(c, gin.H{
		"message": "user deactivated and anonymised",
		"id":      user.ID,
	})
}

// ReactivateUser godoc
// @Summary      Reactivate a soft-deleted user (admin)
// @Description  Restores a previously deactivated account. The "one active staff per department" rule is re-checked at this point — reactivation will fail with 409 if the user is staff and their department already has another active staff. Anonymised email/phone/name fields are not auto-restored; admin should follow up with PATCH /users/{id} to set fresh contact info if needed.
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "User UUID"
// @Success      200  {object}  utils.APIResponse{data=models.User}
// @Failure      404  {object}  utils.APIResponse
// @Failure      409  {object}  utils.APIResponse
// @Router       /users/{id}/reactivate [patch]
func ReactivateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	if user.IsActive {
		utils.BadRequest(c, "user is already active")
		return
	}

	if user.Role == models.RoleStaff {
		if err := ensureStaffDepartmentSlot(user.DepartmentID, &user.ID); err != nil {
			utils.Error(c, http.StatusConflict, err.Error())
			return
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"is_active":  true,
		"deleted_at": nil,
		"updated_at": &now,
	}
	if err := database.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		utils.InternalError(c, "failed to reactivate user")
		return
	}

	database.DB.Preload("Department").First(&user, "id = ?", user.ID)
	utils.OK(c, user)
}

// softDeleteUser scrubs PII from a user record and flips it inactive while
// keeping the row (and its uuid) so that reports, assignments, messages and
// analytics still resolve their foreign keys.
//
// Email is rewritten to `deleted_<id>@removed.local`, the phone is nullified
// (the unique index lets the slot be reclaimed by re-registration), and the
// name is replaced with a placeholder.
func softDeleteUser(user *models.User) error {
	now := time.Now()
	anonymisedEmail := fmt.Sprintf("deleted_%s@removed.local", user.ID.String())

	updates := map[string]interface{}{
		"email":      anonymisedEmail,
		"phone":      nil,
		"full_name":  "[deleted user]",
		"is_active":  false,
		"deleted_at": &now,
		"updated_at": &now,
	}

	return database.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error
}

// assertUserFieldUnique returns an error if any other user already owns the
// given value for a unique column. Used to provide a friendly conflict
// message before relying on the partial unique indexes.
func assertUserFieldUnique(column, value string, excludeID uuid.UUID) error {
	var existing models.User
	err := database.DB.Where(column+" = ? AND id <> ?", value, excludeID).First(&existing).Error
	if err == nil {
		return fmt.Errorf("%s already in use", column)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}
