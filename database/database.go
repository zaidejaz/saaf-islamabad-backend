package database

import (
	"log"

	"github.com/zaidejaz/saaf-islamabad-backend/config"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect(cfg *config.Config) {
	var err error

	logLevel := logger.Silent
	if cfg.GinMode == "debug" {
		logLevel = logger.Warn
	}

	DB, err = gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("database connected")

	if err := DB.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error; err != nil {
		log.Printf("warning: could not create uuid-ossp extension: %v", err)
	}

	err = DB.AutoMigrate(
		&models.User{},
		&models.Department{},
		&models.IssueCategory{},
		&models.Report{},
		&models.ReportImage{},
		&models.ReportStatusHistory{},
		&models.Assignment{},
		&models.Notification{},
		&models.SafetyAlert{},
		&models.OfflineSyncLog{},
		&models.UserPoints{},
		&models.Badge{},
		&models.UserBadge{},
	)
	if err != nil {
		log.Fatalf("auto-migration failed: %v", err)
	}
	log.Println("database migrated")

	seedSuperAdmin(cfg)
}

func seedSuperAdmin(cfg *config.Config) {
	if cfg.SuperAdminPassword == "" {
		log.Println("SUPER_ADMIN_PASSWORD not set, skipping super admin seed")
		return
	}

	var count int64
	DB.Model(&models.User{}).Where("email = ?", cfg.SuperAdminEmail).Count(&count)
	if count > 0 {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SuperAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash super admin password: %v", err)
	}

	admin := models.User{
		FullName:     cfg.SuperAdminName,
		Email:        cfg.SuperAdminEmail,
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
		IsVerified:   true,
		IsActive:     true,
	}

	if err := DB.Create(&admin).Error; err != nil {
		log.Fatalf("failed to create super admin: %v", err)
	}

	log.Printf("super admin created: %s", cfg.SuperAdminEmail)
}
