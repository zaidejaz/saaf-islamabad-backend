package database

import (
	"fmt"
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
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.Message{},
	)
	if err != nil {
		log.Fatalf("auto-migration failed: %v", err)
	}

	if err := applySchemaPatches(); err != nil {
		log.Fatalf("schema patch failed: %v", err)
	}

	log.Println("database migrated")

	seedSuperAdmin(cfg)
}

// applySchemaPatches runs idempotent ALTERs that GORM's AutoMigrate doesn't
// reliably emit on its own — namely loosening the previous NOT NULL on
// users.email so soft-delete can null it out, and ensuring users.phone is
// uniquely indexed only when populated.
func applySchemaPatches() error {
	statements := []string{
		`ALTER TABLE users ALTER COLUMN email DROP NOT NULL`,
		// Drop legacy unique index on phone if it exists, then re-create one
		// that's null-safe (Postgres treats NULLs as distinct, so this is
		// effectively partial unique behaviour — but we make it explicit).
		`DROP INDEX IF EXISTS idx_users_phone`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone ON users (phone) WHERE phone IS NOT NULL`,
		`DROP INDEX IF EXISTS idx_users_email`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email) WHERE email IS NOT NULL`,
	}
	for _, sql := range statements {
		if err := DB.Exec(sql).Error; err != nil {
			return fmt.Errorf("apply %q: %w", sql, err)
		}
	}
	return nil
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

	emailLower := cfg.SuperAdminEmail
	admin := models.User{
		FullName:     cfg.SuperAdminName,
		Email:        &emailLower,
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
