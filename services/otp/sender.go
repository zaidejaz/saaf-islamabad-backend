package otp

import "fmt"

// Sender delivers an OTP SMS to a phone number.
type Sender interface {
	Name() string
	IsConfigured() bool
	Send(phone, code string) error
}

// Config holds Twilio SMS credentials and rate limits.
type Config struct {
	DailyLimit         int
	PerPhoneDailyLimit int

	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFromNumber string
}

// NewSender builds the Twilio SMS OTP sender.
func NewSender(cfg Config) (Sender, error) {
	s := NewTwilioSMSSender(cfg)
	if !s.IsConfigured() {
		return nil, fmt.Errorf("Twilio SMS OTP requires TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, and TWILIO_FROM_NUMBER")
	}
	return s, nil
}
