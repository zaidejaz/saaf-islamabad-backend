package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// ListMessagingAdmins godoc
// @Summary      List admins available for staff messaging
// @Description  Returns active admin accounts so department staff can start a 1:1 admin_staff chat. Each staff member gets their own separate conversation with the same admin.
// @Tags         Messaging
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  utils.APIResponse{data=[]models.User}
// @Router       /contacts/admins [get]
func ListMessagingAdmins(c *gin.Context) {
	var admins []models.User
	if err := database.DB.
		Where("role = ? AND is_active = true", models.RoleAdmin).
		Order("full_name ASC").
		Find(&admins).Error; err != nil {
		utils.InternalError(c, "failed to load admins")
		return
	}
	utils.OK(c, admins)
}

// ListMessagingStaff godoc
// @Summary      List staff a worker can message
// @Description  Returns the worker's managing staff member, or the active staff for their department.
// @Tags         Messaging
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  utils.APIResponse{data=[]models.User}
// @Router       /contacts/staff [get]
func ListMessagingStaff(c *gin.Context) {
	workerID := c.MustGet("user_id").(uuid.UUID)

	var worker models.User
	if err := database.DB.First(&worker, "id = ? AND role = ? AND is_active = true", workerID, models.RoleWorker).Error; err != nil {
		utils.Forbidden(c, "worker profile not found")
		return
	}

	if worker.ManagedByStaffID != nil {
		var staff models.User
		if err := database.DB.Preload("Department").
			First(&staff, "id = ? AND role = ? AND is_active = true", *worker.ManagedByStaffID, models.RoleStaff).Error; err != nil {
			utils.OK(c, []models.User{})
			return
		}
		utils.OK(c, []models.User{staff})
		return
	}

	if worker.DepartmentID != nil {
		if staff := activeStaffForDepartment(*worker.DepartmentID); staff != nil {
			database.DB.Preload("Department").First(staff, "id = ?", staff.ID)
			utils.OK(c, []models.User{*staff})
			return
		}
	}

	utils.OK(c, []models.User{})
}
