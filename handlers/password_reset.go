package handlers

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
	"golang.org/x/crypto/bcrypt"
)

var errPasswordTooShort = errors.New("password must be at least 8 characters")

func resolveResetPassword(req ResetPasswordRequest) (string, error) {
	pw := strings.TrimSpace(req.Password)
	if len(pw) < 8 {
		return "", errPasswordTooShort
	}
	return pw, nil
}

// applyPasswordReset hashes plaintext, overwrites password_hash, bumps token_version,
// writes an audit row, and returns nil on success. Plaintext is never stored.
func applyPasswordReset(actorID, targetID uuid.UUID, plaintext string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now()
	tx := database.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Exec(
		`UPDATE users SET password_hash = ?, token_version = token_version + 1, updated_at = ? WHERE id = ?`,
		string(hash), now, targetID,
	).Error; err != nil {
		tx.Rollback()
		return err
	}

	audit := models.PasswordResetAudit{
		ActorID:  actorID,
		TargetID: targetID,
	}
	if err := tx.Create(&audit).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func respondPasswordReset(c *gin.Context, targetID uuid.UUID, plaintext string) {
	utils.OK(c, gin.H{
		"temporary_password": plaintext,
		"user_id":            targetID,
		"message":            "password reset",
	})
}
