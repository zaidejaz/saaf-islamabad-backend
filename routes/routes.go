package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/zaidejaz/saaf-islamabad-backend/handlers"
	"github.com/zaidejaz/saaf-islamabad-backend/middleware"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
)

func Setup(r *gin.Engine) {
	api := r.Group("/api/v1")

	// ── Public routes ───────────────────────────────
	auth := api.Group("/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
	}

	// Public read-only
	api.GET("/departments", handlers.ListDepartments)
	api.GET("/departments/:id", handlers.GetDepartment)
	api.GET("/categories", handlers.ListCategories)
	api.GET("/categories/:id", handlers.GetCategory)
	api.GET("/safety-alerts", handlers.ListSafetyAlerts)
	api.GET("/safety-alerts/:id", handlers.GetSafetyAlert)

	// ── Authenticated routes ────────────────────────
	protected := api.Group("")
	protected.Use(middleware.AuthRequired())
	{
		// Auth
		protected.GET("/auth/me", handlers.GetMe)

		// Reports (any authenticated user)
		protected.POST("/reports", handlers.CreateReport)
		protected.GET("/reports", handlers.ListReports)
		protected.GET("/reports/:id", handlers.GetReport)

		// Notifications (own)
		protected.GET("/notifications", handlers.ListMyNotifications)
		protected.PATCH("/notifications/:id/read", handlers.MarkNotificationRead)
		protected.PATCH("/notifications/read-all", handlers.MarkAllRead)
	}

	// ── Admin + Staff routes ────────────────────────
	adminStaff := api.Group("")
	adminStaff.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleAdmin, models.RoleStaff))
	{
		adminStaff.PATCH("/reports/:id/status", handlers.UpdateReportStatus)
		adminStaff.GET("/reports/stats", handlers.GetReportStats)

		// Assignments
		adminStaff.GET("/assignments", handlers.ListAssignments)
		adminStaff.PATCH("/assignments/:id/complete", handlers.CompleteAssignment)
	}

	// ── Admin only routes ───────────────────────────
	admin := api.Group("")
	admin.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleAdmin))
	{
		// Users management
		admin.GET("/users", handlers.ListUsers)
		admin.GET("/users/:id", handlers.GetUser)
		admin.DELETE("/users/:id", handlers.DeactivateUser)

		// Departments
		admin.POST("/departments", handlers.CreateDepartment)
		admin.PUT("/departments/:id", handlers.UpdateDepartment)
		admin.DELETE("/departments/:id", handlers.DeleteDepartment)

		// Categories
		admin.POST("/categories", handlers.CreateCategory)
		admin.PUT("/categories/:id", handlers.UpdateCategory)
		admin.DELETE("/categories/:id", handlers.DeleteCategory)

		// Assignments
		admin.POST("/assignments", handlers.CreateAssignment)

		// Notifications (create)
		admin.POST("/notifications", handlers.CreateNotification)

		// Safety Alerts
		admin.POST("/safety-alerts", handlers.CreateSafetyAlert)
		admin.DELETE("/safety-alerts/:id", handlers.DeleteSafetyAlert)
	}
}
