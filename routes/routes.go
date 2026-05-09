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
		protected.GET("/auth/me", handlers.GetMe)

		protected.POST("/reports", handlers.CreateReport)
		protected.GET("/reports", handlers.ListReports)
		protected.GET("/reports/:id", handlers.GetReport)

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

		// Assignments (also accessible by workers via dedicated worker routes below).
		adminStaff.PATCH("/assignments/:id/complete", handlers.CompleteAssignment)
		adminStaff.PATCH("/assignments/:id/assign-worker", handlers.AssignWorker)

		// Workers directory (staff dispatches to workers).
		adminStaff.GET("/workers", handlers.ListWorkers)
		// Staff can soft-delete workers in their team.
		adminStaff.DELETE("/workers/:id", handlers.DeleteWorker)
	}

	// ── Assignments listing for any non-citizen role ─
	assignments := api.Group("")
	assignments.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleAdmin, models.RoleStaff, models.RoleWorker))
	{
		assignments.GET("/assignments", handlers.ListAssignments)
	}

	// ── Worker-only routes ──────────────────────────
	workers := api.Group("/workers/me")
	workers.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleWorker))
	{
		workers.GET("/assignments", handlers.ListMyWorkerAssignments)
		workers.GET("/assignments/:id", handlers.GetMyWorkerAssignment)
		workers.POST("/assignments/:id/resolve", handlers.ResolveWorkerAssignment)
		workers.GET("/profile", handlers.GetMyWorkerProfile)
		workers.PATCH("/profile", handlers.UpdateMyWorkerProfile)
	}

	// ── In-app messaging (admin/staff/worker only) ──
	messaging := api.Group("")
	messaging.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleAdmin, models.RoleStaff, models.RoleWorker))
	{
		messaging.POST("/conversations", handlers.CreateConversation)
		messaging.GET("/conversations", handlers.ListConversations)
		messaging.POST("/conversations/:id/messages", handlers.SendMessage)
		messaging.GET("/conversations/:id/messages", handlers.ListMessages)
		messaging.PATCH("/conversations/:id/read", handlers.MarkConversationRead)
	}

	// WebSocket gateway (auth performed inside the handler so query-token
	// browsers can connect).
	api.GET("/ws/messages", handlers.WSMessages)

	// ── Admin only routes ───────────────────────────
	admin := api.Group("")
	admin.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleAdmin))
	{
		admin.GET("/users", handlers.ListUsers)
		admin.GET("/users/:id", handlers.GetUser)
		admin.PATCH("/users/:id", handlers.UpdateUser)
		admin.PATCH("/users/:id/reactivate", handlers.ReactivateUser)
		admin.DELETE("/users/:id", handlers.DeactivateUser)

		admin.POST("/departments", handlers.CreateDepartment)
		admin.PUT("/departments/:id", handlers.UpdateDepartment)
		admin.DELETE("/departments/:id", handlers.DeleteDepartment)

		admin.POST("/categories", handlers.CreateCategory)
		admin.PUT("/categories/:id", handlers.UpdateCategory)
		admin.DELETE("/categories/:id", handlers.DeleteCategory)

		admin.POST("/assignments", handlers.CreateAssignment)

		admin.POST("/notifications", handlers.CreateNotification)

		admin.POST("/safety-alerts", handlers.CreateSafetyAlert)
		admin.DELETE("/safety-alerts/:id", handlers.DeleteSafetyAlert)
	}
}
