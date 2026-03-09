package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// CreateSafetyAlert godoc
// @Summary      Create safety alert
// @Description  Create a safety alert for a critical report (admin only)
// @Tags         Safety Alerts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateSafetyAlertRequest  true  "Alert data"
// @Success      201   {object}  utils.APIResponse{data=models.SafetyAlert}
// @Failure      400   {object}  utils.APIResponse
// @Router       /safety-alerts [post]
func CreateSafetyAlert(c *gin.Context) {
	var req CreateSafetyAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var report models.Report
	if err := database.DB.First(&report, "id = ?", req.ReportID).Error; err != nil {
		utils.BadRequest(c, "report not found")
		return
	}

	alert := models.SafetyAlert{
		ReportID:  req.ReportID,
		RadiusKM:  req.RadiusKM,
		ExpiresAt: time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour),
	}

	if err := database.DB.Create(&alert).Error; err != nil {
		utils.InternalError(c, "failed to create safety alert")
		return
	}

	database.DB.Preload("Report").First(&alert, "id = ?", alert.ID)
	utils.Created(c, alert)
}

// ListSafetyAlerts godoc
// @Summary      List active safety alerts
// @Description  Get all currently active (non-expired) safety alerts
// @Tags         Safety Alerts
// @Produce      json
// @Param        page       query  int  false  "Page number"  default(1)
// @Param        page_size  query  int  false  "Items per page"  default(20)
// @Success      200  {object}  utils.PaginatedResponse
// @Router       /safety-alerts [get]
func ListSafetyAlerts(c *gin.Context) {
	page, pageSize := utils.GetPagination(c)
	var total int64
	var alerts []models.SafetyAlert

	q := database.DB.Model(&models.SafetyAlert{}).Where("expires_at > ?", time.Now())
	q.Count(&total)
	q.Preload("Report").
		Offset(utils.GetOffset(page, pageSize)).Limit(pageSize).
		Order("created_at DESC").Find(&alerts)

	utils.Paginated(c, alerts, page, pageSize, total)
}

// GetSafetyAlert godoc
// @Summary      Get safety alert
// @Description  Get a single safety alert by ID
// @Tags         Safety Alerts
// @Produce      json
// @Param        id  path  string  true  "Alert UUID"
// @Success      200  {object}  utils.APIResponse{data=models.SafetyAlert}
// @Failure      404  {object}  utils.APIResponse
// @Router       /safety-alerts/{id} [get]
func GetSafetyAlert(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid alert id")
		return
	}

	var alert models.SafetyAlert
	if err := database.DB.Preload("Report").First(&alert, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "safety alert not found")
		return
	}

	utils.OK(c, alert)
}

// DeleteSafetyAlert godoc
// @Summary      Delete safety alert
// @Description  Remove a safety alert (admin only)
// @Tags         Safety Alerts
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Alert UUID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /safety-alerts/{id} [delete]
func DeleteSafetyAlert(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid alert id")
		return
	}

	result := database.DB.Delete(&models.SafetyAlert{}, "id = ?", id)
	if result.RowsAffected == 0 {
		utils.NotFound(c, "safety alert not found")
		return
	}

	utils.OK(c, gin.H{"message": "safety alert deleted"})
}
