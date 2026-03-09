package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/zaidejaz/saaf-islamabad-backend/config"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	_ "github.com/zaidejaz/saaf-islamabad-backend/docs"
	"github.com/zaidejaz/saaf-islamabad-backend/middleware"
	"github.com/zaidejaz/saaf-islamabad-backend/routes"
)

// @title          Saaf Islamabad API
// @version        1.0
// @description    Backend API for the Saaf Islamabad civic issue reporting platform. Citizens can report issues like garbage, broken streetlights, and road damage. Admins assign issues to department staff who resolve them.

// @contact.name   Zaid Ejaz
// @contact.email  zaid@example.com

// @host           localhost:8080
// @BasePath       /api/v1

// @securityDefinitions.apikey BearerAuth
// @in   header
// @name Authorization
// @description Type "Bearer" followed by a space and your JWT token. Example: **Bearer eyJhbGci...**

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	middleware.InitJWT(cfg.JWTSecret)

	gin.SetMode(cfg.GinMode)

	database.Connect(cfg)

	r := gin.Default()

	r.Use(middleware.CORS())

	// Swagger UI at /swagger/index.html (like FastAPI /docs)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Dedicated OpenAPI JSON endpoint for frontend devs
	r.GET("/openapi.json", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/doc.json")
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "saaf-islamabad-backend"})
	})

	routes.Setup(r)

	addr := ":" + cfg.ServerPort
	log.Printf("server starting on %s", addr)
	log.Printf("swagger docs: http://localhost:%s/swagger/index.html", cfg.ServerPort)
	log.Printf("openapi spec: http://localhost:%s/openapi.json", cfg.ServerPort)

	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
