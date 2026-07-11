package otp

import "log"

const ProviderConsole = "console"

// ConsoleSender logs OTP codes instead of sending SMS. For local development only.
type ConsoleSender struct{}

func NewConsoleSender() *ConsoleSender {
	return &ConsoleSender{}
}

func (s *ConsoleSender) Name() string { return ProviderConsole }
func (s *ConsoleSender) IsConfigured() bool {
	return true
}

func (s *ConsoleSender) Send(phone, code string) error {
	log.Printf("[OTP:console] phone=%s code=%s (dev mode — no SMS sent)", phone, code)
	return nil
}
