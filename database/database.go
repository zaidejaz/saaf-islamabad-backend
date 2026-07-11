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
	seedIssueCategories()
}

// Approved civic issue categories for AI classification (spec v1.0).
var defaultCategories = []struct {
	Name        string
	Description string
}{
	{"Waste / Garbage", "Uncollected garbage, litter, dump sites, and waste piles"},
	{"Sewerage / Pipes", "Blocked, overflowing, or damaged sewerage and drainage pipes"},
	{"Traffic Pipes", "Traffic-related civic infrastructure and pipe issues"},
	{"Parks & Green Areas", "Damaged parks, neglected green belts, and public landscaping"},
	{"Broken Roads", "Potholes, cracked asphalt, and damaged road surfaces"},
	{"Electric Lines", "Fallen, exposed, or damaged electrical lines and poles"},
	{"Water Supply", "Leaking, broken, or disrupted public water supply"},
	{"Street Lights", "Non-functional, damaged, or missing street lights"},
	{"Bridges / Flyovers", "Damage or hazards on bridges and flyovers"},
	{"Footpaths / Sidewalks", "Broken, blocked, or unsafe footpaths and sidewalks"},
	{"Stray Animals", "Stray animals causing public safety or sanitation concerns"},
	{"Encroachments", "Illegal structures or occupation of public space"},
}

func seedIssueCategories() {
	for _, c := range defaultCategories {
		var count int64
		DB.Model(&models.IssueCategory{}).Where("name = ?", c.Name).Count(&count)
		if count > 0 {
			continue
		}
		cat := models.IssueCategory{
			Name:        c.Name,
			Description: c.Description,
		}
		if err := DB.Create(&cat).Error; err != nil {
			log.Printf("warning: failed to seed category %q: %v", c.Name, err)
			continue
		}
		log.Printf("seeded category: %s", c.Name)
	}
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
		// Hard guarantee at the DB layer that a department can have at most
		// one active staff member at a time. The handler logic still does
		// the up-front check so we can return a friendly error, but this
		// index protects against races and data drift.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_one_active_staff_per_dept
			ON users (department_id)
			WHERE role = 'staff' AND is_active = true AND department_id IS NOT NULL`,
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
