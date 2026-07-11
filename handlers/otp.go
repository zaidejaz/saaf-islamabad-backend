package handlers

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zaidejaz/saaf-islamabad-backend/config"
	"github.com/zaidejaz/saaf-islamabad-backend/services/otp"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

var (
	otpStore  *otp.Store
	otpSender otp.Sender
)

func InitOTP(cfg *config.Config) {
	otpStore = otp.NewStore(cfg.OTPDailyLimit, cfg.OTPPerPhoneDailyLimit)

	sender, err := otp.NewSender(otp.Config{
		DailyLimit:         cfg.OTPDailyLimit,
		PerPhoneDailyLimit: cfg.OTPPerPhoneDailyLimit,
		TwilioAccountSID:   cfg.TwilioAccountSID,
		TwilioAuthToken:    cfg.TwilioAuthToken,
		TwilioFromNumber:   cfg.TwilioFromNumber,
	})
	if err != nil {
		log.Printf("twilio sms otp not configured: %v", err)
		otpSender = nil
		return
	}
	otpSender = sender
	log.Printf("otp sender ready: twilio sms daily_limit=%d per_phone=%d",
		cfg.OTPDailyLimit, cfg.OTPPerPhoneDailyLimit)
}

// SendOTP godoc
// @Summary      Send phone OTP via SMS
// @Description  Sends a 6-digit OTP by SMS (Twilio) to a Pakistani mobile number for citizen registration.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      SendOTPRequest  true  "OTP request"
// @Success      200   {object}  utils.APIResponse
// @Failure      400   {object}  utils.APIResponse
// @Failure      409   {object}  utils.APIResponse
// @Failure      503   {object}  utils.APIResponse
// @Router       /auth/otp/send [post]
func SendOTP(c *gin.Context) {
	if otpSender == nil {
		utils.Error(c, 503, "SMS OTP is not configured. Set TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, and TWILIO_FROM_NUMBER.")
		return
	}

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
		log.Printf("sms otp send failed (phone=%s): %v", phone, err)
		utils.InternalError(c, "failed to send SMS OTP: "+err.Error())
		return
	}

	utils.OK(c, gin.H{
		"message":            "OTP sent via SMS",
		"channel":            "sms",
		"expires_in_seconds": 300,
	})
}
