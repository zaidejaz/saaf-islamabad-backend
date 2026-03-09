package handlers

import (
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
// @Param        role       query  string  false  "Filter by role (citizen, admin, staff)"
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
	q.Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).Order("created_at DESC").Find(&users)

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
	if err := database.DB.First(&user, "id = ? AND is_active = true", id).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	utils.OK(c, user)
}

// DeactivateUser godoc
// @Summary      Deactivate user (soft delete)
// @Description  Set user is_active to false (admin only)
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

	result := database.DB.Model(&models.User{}).Where("id = ?", id).Update("is_active", false)
	if result.RowsAffected == 0 {
		utils.NotFound(c, "user not found")
		return
	}

	utils.OK(c, gin.H{"message": "user deactivated"})
}
