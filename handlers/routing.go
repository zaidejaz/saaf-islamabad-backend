package handlers

import (
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/services/ai"
)

func departmentForCategory(categoryID *uuid.UUID) (deptID *uuid.UUID, deptName string) {
	if categoryID == nil {
		return nil, ""
	}

	var cat models.IssueCategory
	if err := database.DB.Preload("DefaultDepartment").First(&cat, "id = ?", categoryID).Error; err != nil {
		return nil, ""
	}
	if cat.DefaultDepartmentID != nil {
		deptID = cat.DefaultDepartmentID
	}
	if cat.DefaultDepartment != nil {
		deptName = cat.DefaultDepartment.Name
	}
	return deptID, deptName
}

func enrichClassificationResult(result *ai.ClassifyResult) {
	if result == nil || result.CategoryID == nil {
		return
	}
	deptID, deptName := departmentForCategory(result.CategoryID)
	result.DepartmentID = deptID
	result.DepartmentName = deptName
}

func applyDepartmentFromCategory(report *models.Report, categoryID *uuid.UUID) {
	deptID, _ := departmentForCategory(categoryID)
	report.DepartmentID = deptID
}
