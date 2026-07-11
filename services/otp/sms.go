package otp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ProviderSMS = "sms"

// TwilioSMSSender delivers OTPs via Twilio SMS to the phone number.
type TwilioSMSSender struct {
	accountSID string
	authToken  string
	fromNumber string
	httpClient *http.Client
}

func NewTwilioSMSSender(cfg Config) *TwilioSMSSender {
	return &TwilioSMSSender{
		accountSID: strings.TrimSpace(cfg.TwilioAccountSID),
		authToken:  strings.TrimSpace(cfg.TwilioAuthToken),
		fromNumber: strings.TrimSpace(cfg.TwilioFromNumber),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *TwilioSMSSender) Name() string { return ProviderSMS }
func (s *TwilioSMSSender) IsConfigured() bool {
	return s.accountSID != "" && s.authToken != "" && s.fromNumber != ""
}

func (s *TwilioSMSSender) Send(phone, code string) error {
	message := fmt.Sprintf("Your Saaf Islamabad verification code is %s. Valid for 5 minutes.", code)

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

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
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

	log.Printf("[OTP:sms] sent to %s", phone)
	return nil
}
