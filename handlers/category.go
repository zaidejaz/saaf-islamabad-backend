package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// CreateCategory godoc
// @Summary      Create issue category
// @Description  Add a new issue category (admin only)
// @Tags         Categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateCategoryRequest  true  "Category data"
// @Success      201   {object}  utils.APIResponse{data=models.IssueCategory}
// @Failure      400   {object}  utils.APIResponse
// @Router       /categories [post]
func CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	cat := models.IssueCategory{
		Name:                req.Name,
		Description:         req.Description,
		DefaultDepartmentID: req.DefaultDepartmentID,
	}

	if err := database.DB.Create(&cat).Error; err != nil {
		utils.InternalError(c, "failed to create category")
		return
	}

	database.DB.Preload("DefaultDepartment").First(&cat, "id = ?", cat.ID)
	utils.Created(c, cat)
}

// ListCategories godoc
// @Summary      List categories
// @Description  Get all issue categories
// @Tags         Categories
// @Produce      json
// @Success      200  {object}  utils.APIResponse{data=[]models.IssueCategory}
// @Router       /categories [get]
func ListCategories(c *gin.Context) {
	var categories []models.IssueCategory
	database.DB.Preload("DefaultDepartment").Order("name ASC").Find(&categories)
	utils.OK(c, categories)
}

// GetCategory godoc
// @Summary      Get category
// @Description  Get issue category by ID
// @Tags         Categories
// @Produce      json
// @Param        id  path  string  true  "Category UUID"
// @Success      200  {object}  utils.APIResponse{data=models.IssueCategory}
// @Failure      404  {object}  utils.APIResponse
// @Router       /categories/{id} [get]
func GetCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid category id")
		return
	}

	var cat models.IssueCategory
	if err := database.DB.Preload("DefaultDepartment").First(&cat, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "category not found")
		return
	}

	utils.OK(c, cat)
}

// UpdateCategory godoc
// @Summary      Update category
// @Description  Update issue category (admin only)
// @Tags         Categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                 true  "Category UUID"
// @Param        body  body  UpdateCategoryRequest   true  "Fields to update"
// @Success      200   {object}  utils.APIResponse{data=models.IssueCategory}
// @Failure      404   {object}  utils.APIResponse
// @Router       /categories/{id} [put]
func UpdateCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid category id")
		return
	}

	var cat models.IssueCategory
	if err := database.DB.First(&cat, "id = ?", id).Error; err != nil {
		utils.NotFound(c, "category not found")
		return
	}

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if req.Name != nil {
		cat.Name = *req.Name
	}
	if req.Description != nil {
		cat.Description = *req.Description
	}
	if req.DefaultDepartmentID != nil {
		cat.DefaultDepartmentID = req.DefaultDepartmentID
	}

	database.DB.Save(&cat)
	database.DB.Preload("DefaultDepartment").First(&cat, "id = ?", cat.ID)
	utils.OK(c, cat)
}

// DeleteCategory godoc
// @Summary      Delete category
// @Description  Remove an issue category (admin only)
// @Tags         Categories
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Category UUID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /categories/{id} [delete]
func DeleteCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid category id")
		return
	}

	result := database.DB.Delete(&models.IssueCategory{}, "id = ?", id)
	if result.RowsAffected == 0 {
		utils.NotFound(c, "category not found")
		return
	}

	utils.OK(c, gin.H{"message": "category deleted"})
}
