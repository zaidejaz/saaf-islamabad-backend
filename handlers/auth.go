package handlers

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/middleware"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/services/otp"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Register godoc
// @Summary      Register a new user
// @Description  Citizens register with phone + password (Pakistani format). Admin / staff / worker accounts use email + password.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      RegisterRequest  true  "Registration payload"
// @Success      201   {object}  utils.APIResponse{data=AuthResponse}
// @Failure      400   {object}  utils.APIResponse
// @Failure      409   {object}  utils.APIResponse
// @Router       /auth/register [post]
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	role := models.Role(req.Role)

	user := models.User{
		FullName:     strings.TrimSpace(req.FullName),
		Role:         role,
		IsVerified:   role == models.RoleCitizen,
		IsActive:     true,
		DepartmentID: req.DepartmentID,
	}

	switch role {
	case models.RoleCitizen:
		if req.Phone == "" {
			utils.BadRequest(c, "phone is required for citizen accounts")
			return
		}
		phone, err := utils.NormalizePKPhone(req.Phone)
		if err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		if strings.TrimSpace(req.OTP) == "" {
			utils.BadRequest(c, "OTP is required for citizen registration")
			return
		}
		if !otpStore.Verify(phone, otp.PurposeRegister, strings.TrimSpace(req.OTP)) {
			utils.BadRequest(c, "invalid or expired OTP")
			return
		}
		if exists, err := userExistsByPhone(phone); err != nil {
			utils.InternalError(c, "lookup failed")
			return
		} else if exists {
			utils.Error(c, http.StatusConflict, "phone already registered")
			return
		}
		user.Phone = &phone
		// Citizens may optionally provide email — must not collide with worker/staff/admin emails.
		if email := strings.TrimSpace(req.Email); email != "" {
			if _, err := mail.ParseAddress(email); err != nil {
				utils.BadRequest(c, "invalid email format")
				return
			}
			emailLower := strings.ToLower(email)
			if exists, err := userExistsByEmail(emailLower); err != nil {
				utils.InternalError(c, "lookup failed")
				return
			} else if exists {
				utils.Error(c, http.StatusConflict, "email already registered — use a different email or leave it blank")
				return
			}
			user.Email = &emailLower
		}

	case models.RoleAdmin, models.RoleStaff, models.RoleWorker:
		if req.Email == "" {
			utils.BadRequest(c, "email is required for admin/staff/worker accounts")
			return
		}
		if _, err := mail.ParseAddress(req.Email); err != nil {
			utils.BadRequest(c, "invalid email format")
			return
		}
		emailLower := strings.ToLower(strings.TrimSpace(req.Email))
		if exists, err := userExistsByEmail(emailLower); err != nil {
			utils.InternalError(c, "lookup failed")
			return
		} else if exists {
			utils.Error(c, http.StatusConflict, "email already registered")
			return
		}
		user.Email = &emailLower

		if req.Phone != "" {
			phone, err := utils.NormalizePKPhone(req.Phone)
			if err != nil {
				utils.BadRequest(c, err.Error())
				return
			}
			user.Phone = &phone
		}

		// Staff registration: department is mandatory and exclusive — only
		// one active staff per department at a time.
		if role == models.RoleStaff {
			if req.DepartmentID == nil {
				utils.BadRequest(c, "department_id is required for staff accounts")
				return
			}
			if !departmentExists(*req.DepartmentID) {
				utils.BadRequest(c, "department not found")
				return
			}
			if err := ensureStaffDepartmentSlot(req.DepartmentID, nil); err != nil {
				utils.Error(c, http.StatusConflict, err.Error())
				return
			}
		}

		if role == models.RoleWorker {
			utils.BadRequest(c, "workers are created by department staff via POST /api/v1/workers")
			return
		}

	default:
		utils.BadRequest(c, "invalid role")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalError(c, "failed to hash password")
		return
	}
	user.PasswordHash = string(hash)

	if err := database.DB.Create(&user).Error; err != nil {
		utils.InternalError(c, "failed to create user")
		return
	}

	token, err := generateToken(user)
	if err != nil {
		utils.InternalError(c, "failed to generate token")
		return
	}

	utils.Created(c, AuthResponse{
		Token: token,
		User:  toUserSummary(user),
	})
}

// Login godoc
// @Summary      Login
// @Description  Authenticate with phone + password (citizens) or email + password (admin/staff/worker). Provide either `phone`, `email`, or a generic `identifier` along with `password`.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest  true  "Login credentials"
// @Success      200   {object}  utils.APIResponse{data=AuthResponse}
// @Failure      400   {object}  utils.APIResponse
// @Failure      401   {object}  utils.APIResponse
// @Router       /auth/login [post]
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Phone)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" || req.Password == "" {
		utils.BadRequest(c, "identifier (phone/email) and password are required")
		return
	}

	var user models.User
	q := database.DB.Where("is_active = true")

	if _, err := mail.ParseAddress(identifier); err == nil {
		// Looks like an email
		emailLower := strings.ToLower(identifier)
		err = q.Where("email = ?", emailLower).First(&user).Error
		if err != nil {
			utils.Unauthorized(c, "invalid credentials")
			return
		}
	} else {
		// Treat as phone number
		phone, err := utils.NormalizePKPhone(identifier)
		if err != nil {
			utils.Unauthorized(c, "invalid credentials")
			return
		}
		if err := q.Where("phone = ?", phone).First(&user).Error; err != nil {
			utils.Unauthorized(c, "invalid credentials")
			return
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		utils.Unauthorized(c, "invalid credentials")
		return
	}

	token, err := generateToken(user)
	if err != nil {
		utils.InternalError(c, "failed to generate token")
		return
	}

	utils.OK(c, AuthResponse{
		Token: token,
		User:  toUserSummary(user),
	})
}

// GetMe godoc
// @Summary      Get current user
// @Description  Return the authenticated user's profile
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  utils.APIResponse{data=models.User}
// @Failure      401  {object}  utils.APIResponse
// @Router       /auth/me [get]
func GetMe(c *gin.Context) {
	userID := c.MustGet("user_id")
	var user models.User
	if err := database.DB.Preload("Department").First(&user, "id = ?", userID).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}
	utils.OK(c, user)
}

// ChangePassword godoc
// @Summary      Change current user's password
// @Description  Authenticated citizens, workers, staff, and admins can update their own password by providing the current password.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      ChangePasswordRequest  true  "Password update payload"
// @Success      200   {object}  utils.APIResponse
// @Failure      400   {object}  utils.APIResponse
// @Failure      401   {object}  utils.APIResponse
// @Router       /auth/change-password [patch]
func ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID := c.MustGet("user_id")
	var user models.User
	if err := database.DB.First(&user, "id = ? AND is_active = true", userID).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		utils.Unauthorized(c, "current password is incorrect")
		return
	}

	if req.CurrentPassword == req.NewPassword {
		utils.BadRequest(c, "new password must be different from current password")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalError(c, "failed to hash password")
		return
	}

	now := time.Now()
	if err := database.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"password_hash": string(hash),
		"updated_at":    &now,
	}).Error; err != nil {
		utils.InternalError(c, "failed to update password")
		return
	}

	utils.OK(c, gin.H{"message": "password updated"})
}

func generateToken(user models.User) (string, error) {
	claims := middleware.Claims{
		UserID:       user.ID,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(middleware.JWTSecret)
}

func toUserSummary(u models.User) UserSummary {
	s := UserSummary{
		ID:       u.ID,
		FullName: u.FullName,
		Role:     string(u.Role),
	}
	if u.Email != nil {
		s.Email = *u.Email
	}
	if u.Phone != nil {
		s.Phone = *u.Phone
	}
	return s
}

func userExistsByPhone(phone string) (bool, error) {
	var u models.User
	err := database.DB.Where("phone = ?", phone).First(&u).Error
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func userExistsByEmail(email string) (bool, error) {
	var u models.User
	err := database.DB.Where("email = ?", email).First(&u).Error
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}
