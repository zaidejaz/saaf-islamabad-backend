package otp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type SMSSender struct {
	accountSID string
	authToken  string
	fromNumber string
	httpClient *http.Client
	debugMode  bool
}

func NewSMSSender(debugMode bool) *SMSSender {
	return &SMSSender{
		accountSID: strings.TrimSpace(os.Getenv("TWILIO_ACCOUNT_SID")),
		authToken:  strings.TrimSpace(os.Getenv("TWILIO_AUTH_TOKEN")),
		fromNumber: strings.TrimSpace(os.Getenv("TWILIO_FROM_NUMBER")),
		httpClient: &http.Client{Timeout: 15 * time.Second},
		debugMode:  debugMode,
	}
}

func (s *SMSSender) IsConfigured() bool {
	return s.configured()
}

func (s *SMSSender) configured() bool {
	return s.accountSID != "" && s.authToken != "" && s.fromNumber != ""
}

// Send delivers the OTP via Twilio when configured; otherwise logs it in debug mode.
func (s *SMSSender) Send(phone, code string) error {
	message := fmt.Sprintf("Your Saaf Islamabad verification code is %s. Valid for 5 minutes.", code)

	if !s.configured() {
		if s.debugMode {
			log.Printf("[OTP] phone=%s code=%s (SMS provider not configured — set TWILIO_* env vars for production)", phone, code)
			return nil
		}
		return fmt.Errorf("SMS service is not configured")
	}

	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.accountSID)
	form := url.Values{}
	form.Set("To", phone)
	form.Set("From", s.fromNumber)
	form.Set("Body", message)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.accountSID, s.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		var twilioErr struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &twilioErr)
		if twilioErr.Message != "" {
			return fmt.Errorf("SMS delivery failed: %s", twilioErr.Message)
		}
		return fmt.Errorf("SMS delivery failed with status %d", resp.StatusCode)
	}

	log.Printf("[OTP] SMS sent to %s", phone)
	return nil
}
