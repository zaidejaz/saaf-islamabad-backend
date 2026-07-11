package config

import (
	"os"
	"strconv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	JWTSecret      string
	JWTExpiryHours int

	ServerPort string
	GinMode    string
	BaseURL    string

	SuperAdminName     string
	SuperAdminEmail    string
	SuperAdminPassword string

	// AI image classification (Groq by default; provider/model are swappable).
	AIProvider    string
	AIModel       string
	AIAPIKey      string
	AIBaseURL     string
	AITimeoutSecs int
	AIEnabled     bool

	// Twilio SMS OTP.
	OTPDailyLimit         int
	OTPPerPhoneDailyLimit int
	TwilioAccountSID      string
	TwilioAuthToken       string
	TwilioFromNumber      string
}

func Load() *Config {
	expiry, _ := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "72"))
	aiTimeout, _ := strconv.Atoi(getEnv("AI_TIMEOUT_SECS", "8"))
	aiEnabled := getEnv("AI_ENABLED", "true") != "false"
	otpDaily, _ := strconv.Atoi(getEnv("OTP_DAILY_LIMIT", "100"))
	otpPerPhone, _ := strconv.Atoi(getEnv("OTP_PER_PHONE_DAILY", "10"))

	return &Config{
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "saaf_user"),
		DBPassword:         getEnv("DB_PASSWORD", "saaf_secret"),
		DBName:             getEnv("DB_NAME", "saaf_islamabad"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		JWTSecret:          getEnv("JWT_SECRET", "change-me"),
		JWTExpiryHours:     expiry,
		ServerPort:         getEnv("SERVER_PORT", "8080"),
		GinMode:            getEnv("GIN_MODE", "debug"),
		BaseURL:            getEnv("BASE_URL", "localhost:8080"),
		SuperAdminName:     getEnv("SUPER_ADMIN_NAME", "Super Admin"),
		SuperAdminEmail:    getEnv("SUPER_ADMIN_EMAIL", "admin@saafislamabad.pk"),
		SuperAdminPassword: getEnv("SUPER_ADMIN_PASSWORD", ""),
		AIProvider:         getEnv("AI_PROVIDER", "groq"),
		AIModel:            getEnv("AI_MODEL", "meta-llama/llama-4-scout-17b-16e-instruct"),
		AIAPIKey:           getEnv("GROQ_API_KEY", getEnv("AI_API_KEY", "")),
		AIBaseURL:          getEnv("AI_BASE_URL", "https://api.groq.com/openai/v1"),
		AITimeoutSecs:      aiTimeout,
		AIEnabled:          aiEnabled,

		OTPDailyLimit:         otpDaily,
		OTPPerPhoneDailyLimit: otpPerPhone,
		TwilioAccountSID:      getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:       getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFromNumber:      getEnv("TWILIO_FROM_NUMBER", ""),
	}
}

func (c *Config) DSN() string {
	return "host=" + c.DBHost +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" port=" + c.DBPort +
		" sslmode=" + c.DBSSLMode +
		" TimeZone=Asia/Karachi"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
