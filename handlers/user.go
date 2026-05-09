package handlers

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// ListUsers godoc
// @Summary      List users
// @Description  Get paginated list of users (admin only)
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  int     false  "Page number"  default(1)
// @Param        page_size  query  int     false  "Items per page"  default(20)
// @Param        role       query  string  false  "Filter by role (citizen, admin, staff, worker)"
// @Success      200  {object}  utils.PaginatedResponse
// @Router       /users [get]
func ListUsers(c *gin.Context) {
	page, pageSize := utils.GetPagination(c)
	var total int64
	var users []models.User

	q := database.DB.Model(&models.User{}).Where("is_active = true")

	if role := c.Query("role"); role != "" {
		q = q.Where("role = ?", role)
	}

	q.Count(&total)
	q.Preload("Department").Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).Order("created_at DESC").Find(&users)

	utils.Paginated(c, users, page, pageSize, total)
}

// GetUser godoc
// @Summary      Get user by ID
// @Description  Retrieve a single user
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "User UUID"
// @Success      200  {object}  utils.APIResponse{data=models.User}
// @Failure      404  {object}  utils.APIResponse
// @Router       /users/{id} [get]
func GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}

	var user models.User
	if err := database.DB.Preload("Department").First(&user, "id = ? AND is_active = true", id).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	utils.OK(c, user)
}

// DeactivateUser godoc
// @Summary      Soft-delete a user
// @Description  Anonymises the user's email + name and marks the account inactive. Linked records (reports, assignments, messages, analytics) remain queryable. Admin only — to delete workers, staff use DELETE /workers/{id}.
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

	// Admins can soft-delete staff/workers/citizens but not other admins.
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
