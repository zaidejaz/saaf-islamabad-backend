package otp

import "fmt"

// Sender delivers an OTP SMS to a phone number.
type Sender interface {
	Name() string
	IsConfigured() bool
	Send(phone, code string) error
}

// Config holds OTP delivery settings and rate limits.
type Config struct {
	DailyLimit         int
	PerPhoneDailyLimit int
	DevMode            bool

	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFromNumber string
}

// NewSender builds the OTP sender. Dev mode uses console logging; otherwise Twilio SMS.
func NewSender(cfg Config) (Sender, error) {
	if cfg.DevMode {
		return NewConsoleSender(), nil
	}

	s := NewTwilioSMSSender(cfg)
	if !s.IsConfigured() {
		return nil, fmt.Errorf("Twilio SMS OTP requires TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, and TWILIO_FROM_NUMBER")
	}
	return s, nil
}
