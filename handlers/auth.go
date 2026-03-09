package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/middleware"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
	"golang.org/x/crypto/bcrypt"
)

// Register godoc
// @Summary      Register a new user
// @Description  Create a citizen, admin, or staff account
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

	var existing models.User
	if err := database.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		utils.Error(c, http.StatusConflict, "email already registered")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalError(c, "failed to hash password")
		return
	}

	user := models.User{
		FullName:     req.FullName,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hash),
		Role:         models.Role(req.Role),
		IsVerified:   false,
		IsActive:     true,
	}

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
// @Description  Authenticate with email and password
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

	var user models.User
	if err := database.DB.Where("email = ? AND is_active = true", req.Email).First(&user).Error; err != nil {
		utils.Unauthorized(c, "invalid credentials")
		return
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
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		utils.NotFound(c, "user not found")
		return
	}
	utils.OK(c, user)
}

func generateToken(user models.User) (string, error) {
	claims := middleware.Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(middleware.JWTSecret)
}

func toUserSummary(u models.User) UserSummary {
	return UserSummary{
		ID:       u.ID,
		FullName: u.FullName,
		Email:    u.Email,
		Role:     string(u.Role),
	}
}
