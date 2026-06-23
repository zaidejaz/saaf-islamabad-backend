package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zaidejaz/saaf-islamabad-backend/services/otp"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

var (
	otpStore   = otp.NewStore()
	otpSender  *otp.SMSSender
	otpDebug   bool
)

func InitOTP(debugMode bool) {
	otpDebug = debugMode
	otpSender = otp.NewSMSSender(debugMode)
}

// SendOTP godoc
// @Summary      Send phone OTP
// @Description  Sends a 6-digit OTP to a Pakistani mobile number for citizen registration.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      SendOTPRequest  true  "OTP request"
// @Success      200   {object}  utils.APIResponse
// @Failure      400   {object}  utils.APIResponse
// @Failure      409   {object}  utils.APIResponse
// @Router       /auth/otp/send [post]
func SendOTP(c *gin.Context) {
	var req SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = otp.PurposeRegister
	}
	if purpose != otp.PurposeRegister {
		utils.BadRequest(c, "unsupported OTP purpose")
		return
	}

	phone, err := utils.NormalizePKPhone(req.Phone)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if exists, err := userExistsByPhone(phone); err != nil {
		utils.InternalError(c, "lookup failed")
		return
	} else if exists {
		utils.Error(c, 409, "phone already registered")
		return
	}

	code, err := otpStore.Create(phone, purpose)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if err := otpSender.Send(phone, code); err != nil {
		utils.InternalError(c, "failed to send OTP")
		return
	}

	payload := gin.H{
		"message": "OTP sent successfully",
		"expires_in_seconds": 300,
	}
	if otpDebug && otpSender != nil && !otpSender.IsConfigured() {
		payload["dev_otp"] = code
	}

	utils.OK(c, payload)
}
