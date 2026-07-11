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
	OTPDevMode            bool
	TwilioAccountSID      string
	TwilioAuthToken       string
	TwilioFromNumber      string

	// Image storage (local disk or Cloudflare R2).
	StorageDriver   string
	LocalUploadsDir string
	R2AccountID     string
	R2AccessKeyID   string
	R2SecretAccessKey string
	R2Bucket        string
	R2PublicBaseURL string
}

func Load() *Config {
	expiry, _ := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "72"))
	aiTimeout, _ := strconv.Atoi(getEnv("AI_TIMEOUT_SECS", "8"))
	aiEnabled := getEnv("AI_ENABLED", "true") != "false"
	otpDaily, _ := strconv.Atoi(getEnv("OTP_DAILY_LIMIT", "100"))
	otpPerPhone, _ := strconv.Atoi(getEnv("OTP_PER_PHONE_DAILY", "10"))
	otpDevMode := getEnv("OTP_DEV_MODE", "") == "true"

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
		OTPDevMode:            otpDevMode,
		TwilioAccountSID:      getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:       getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFromNumber:      getEnv("TWILIO_FROM_NUMBER", ""),

		StorageDriver:     getEnv("STORAGE_DRIVER", "local"),
		LocalUploadsDir:   getEnv("LOCAL_UPLOADS_DIR", "./uploads"),
		R2AccountID:       getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:     getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:          getEnv("R2_BUCKET", ""),
		R2PublicBaseURL:   getEnv("R2_PUBLIC_BASE_URL", ""),
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
