package otp

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"
)

const (
	PurposeRegister = "register"
	otpLength       = 6
	otpTTL          = 5 * time.Minute
	resendCooldown  = 60 * time.Second
)

type entry struct {
	code      string
	purpose   string
	expiresAt time.Time
	sentAt    time.Time
}

type Store struct {
	mu   sync.RWMutex
	data map[string]entry
}

func NewStore() *Store {
	return &Store{data: make(map[string]entry)}
}

func key(phone, purpose string) string {
	return phone + "|" + purpose
}

func generateCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// Create generates a fresh OTP for the phone/purpose pair.
func (s *Store) Create(phone, purpose string) (code string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key(phone, purpose)
	if existing, ok := s.data[k]; ok {
		if time.Since(existing.sentAt) < resendCooldown {
			return "", fmt.Errorf("please wait before requesting another code")
		}
	}

	code, err = generateCode()
	if err != nil {
		return "", err
	}

	s.data[k] = entry{
		code:      code,
		purpose:   purpose,
		expiresAt: time.Now().Add(otpTTL),
		sentAt:    time.Now(),
	}
	return code, nil
}

// Verify checks the OTP and removes it on success.
func (s *Store) Verify(phone, purpose, code string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key(phone, purpose)
	existing, ok := s.data[k]
	if !ok {
		return false
	}
	if time.Now().After(existing.expiresAt) {
		delete(s.data, k)
		return false
	}
	if existing.code != code {
		return false
	}
	delete(s.data, k)
	return true
}
