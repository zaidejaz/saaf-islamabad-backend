package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// CreateDepartment godoc
// @Summary      Create department
// @Description  Add a new department (admin only)
// @Tags         Departments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateDepartmentRequest  true  "Department data"
// @Success      201   {object}  utils.APIResponse{data=models.Department}
// @Failure      400   {object}  utils.APIResponse
// @Router       /departments [post]
func CreateDepartment(c *gin.Context) {
	var req CreateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	dept := models.Department{
		Name:         req.Name,
		Description:  req.Description,
		ContactEmail: req.ContactEmail,
	}

	if err := database.DB.Create(&dept).Error; err != nil {
		utils.InternalError(c, "failed to create department")
		return
	}

	utils.Created(c, dept)
}

// ListDepartments godoc
// @Summary      List departments
// @Description  Get all departments
// @Tags         Departments
// @Produce      json
// @Success      200  {object}  utils.APIResponse{data=[]models.Department}
// @Router       /departments [get]
func ListDepartments(c *gin.Context) {
	var departments []models.Department
	database.DB.Order("name ASC").Find(&departments)
	utils.OK(c, departments)
}

// GetDepartment godoc
// @Summary      Get department
// @Description  Get department by ID
// @Tags         Departments
// @Produce      json
// @Param        id  path  string  true  "Department UUID"
// @Success      200  {object}  utils.APIResponse{data=models.Department}
// @Failure      404  {object}  utils.APIResponse
// @Router       /departments/{id} [get]
func GetDepartment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid department id")
		return
	}

	var dept models.Department
	if err := database.DB.First(&dept, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "department not found")
		return
	}

	utils.OK(c, dept)
}

// UpdateDepartment godoc
// @Summary      Update department
// @Description  Update department details (admin only)
// @Tags         Departments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                   true  "Department UUID"
// @Param        body  body  UpdateDepartmentRequest   true  "Fields to update"
// @Success      200   {object}  utils.APIResponse{data=models.Department}
// @Failure      404   {object}  utils.APIResponse
// @Router       /departments/{id} [put]
func UpdateDepartment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid department id")
		return
	}

	var dept models.Department
	if err := database.DB.First(&dept, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "department not found")
		return
	}

	var req UpdateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if req.Name != nil {
		dept.Name = *req.Name
	}
	if req.Description != nil {
		dept.Description = *req.Description
	}
	if req.ContactEmail != nil {
		dept.ContactEmail = *req.ContactEmail
	}

	database.DB.Save(&dept)
	utils.OK(c, dept)
}

// DeleteDepartment godoc
// @Summary      Delete department
// @Description  Remove a department (admin only)
// @Tags         Departments
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Department UUID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /departments/{id} [delete]
func DeleteDepartment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid department id")
		return
	}

	result := database.DB.Delete(&models.Department{}, "id = ?", id)
	if result.RowsAffected == 0 {
		utils.NotFound(c, "department not found")
		return
	}

	utils.OK(c, gin.H{"message": "department deleted"})
}
