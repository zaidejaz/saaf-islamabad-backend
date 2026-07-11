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

	defaultDailyLimit    = 100
	defaultPerPhoneDaily = 10
)

type entry struct {
	code      string
	purpose   string
	expiresAt time.Time
	sentAt    time.Time
}

type Store struct {
	mu                 sync.RWMutex
	data               map[string]entry
	dailyGlobal        map[string]int // yyyy-mm-dd -> count
	dailyPhone         map[string]int // yyyy-mm-dd|phone -> count
	dailyLimit         int
	perPhoneDailyLimit int
}

func NewStore(dailyLimit, perPhoneDailyLimit int) *Store {
	if dailyLimit <= 0 {
		dailyLimit = defaultDailyLimit
	}
	if perPhoneDailyLimit <= 0 {
		perPhoneDailyLimit = defaultPerPhoneDaily
	}
	return &Store{
		data:               make(map[string]entry),
		dailyGlobal:        make(map[string]int),
		dailyPhone:         make(map[string]int),
		dailyLimit:         dailyLimit,
		perPhoneDailyLimit: perPhoneDailyLimit,
	}
}

func key(phone, purpose string) string {
	return phone + "|" + purpose
}

func dayKey() string {
	return time.Now().Format("2006-01-02")
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

	day := dayKey()
	if s.dailyGlobal[day] >= s.dailyLimit {
		return "", fmt.Errorf("daily OTP limit reached (%d). Try again tomorrow or raise OTP_DAILY_LIMIT", s.dailyLimit)
	}
	phoneDay := day + "|" + phone
	if s.dailyPhone[phoneDay] >= s.perPhoneDailyLimit {
		return "", fmt.Errorf("too many OTP requests for this number today (limit %d)", s.perPhoneDailyLimit)
	}

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
	s.dailyGlobal[day]++
	s.dailyPhone[phoneDay]++
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
