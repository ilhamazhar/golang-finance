package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerPort          string
	AppEnv              string // "development" | "production"; gates dev-only conveniences
	AppBaseURL          string // public base URL, used to build email links
	FrontendURL         string // SPA base URL; email verify link redirects here
	DatabaseURL         string
	RedisURL            string
	JWTSecret           string
	JWTRefreshSecret    string
	JWTExpiry           time.Duration
	JWTRefreshExpiry    time.Duration
	EmailVerifyExpiry   time.Duration // how long an email-verification token is valid
	PasswordResetExpiry time.Duration // how long a password-reset token is valid
	SMTPHost            string        // if empty, emails are logged instead of sent
	SMTPPort            string
	SMTPUsername        string
	SMTPPassword        string
	SMTPFrom            string // From address, e.g. "Azhar Finance <no-reply@azhar.test>"
	XenditAPIKey        string
	XenditCallbackToken string
	RateLimitAuth       int           // max requests per period for /auth/* (IP-based)
	RateLimitAuthPeriod time.Duration // time window for auth rate limit
	RateLimitAPI        int           // max requests per period for /api/* (user-based)
	RateLimitAPIPeriod  time.Duration // time window for api rate limit
	CORSAllowedOrigins  []string      // comma-separated list, e.g. https://app.com,https://admin.com
	AdminEmail          string        // if set, this user is promoted to admin on startup (bootstrap)
}

func Load() (Config, error) {
	jwtExpiry, err := strconv.ParseFloat(getEnv("JWT_EXPIRY_HOURS", "24"), 64)
	if err != nil {
		return Config{}, errors.New("invalid JWT_EXPIRY_HOURS")
	}
	jwtRefreshExpiry, err := strconv.ParseFloat(getEnv("JWT_REFRESH_EXPIRY_HOURS", "168"), 64)
	if err != nil {
		return Config{}, errors.New("invalid JWT_REFRESH_EXPIRY_HOURS")
	}
	emailVerifyExpiry, err := strconv.ParseFloat(getEnv("EMAIL_VERIFY_EXPIRY_HOURS", "24"), 64)
	if err != nil {
		return Config{}, errors.New("invalid EMAIL_VERIFY_EXPIRY_HOURS")
	}
	passwordResetExpiry, err := strconv.ParseFloat(getEnv("PASSWORD_RESET_EXPIRY_HOURS", "1"), 64)
	if err != nil {
		return Config{}, errors.New("invalid PASSWORD_RESET_EXPIRY_HOURS")
	}

	rateLimitAuth, err := strconv.Atoi(getEnv("RATE_LIMIT_AUTH_MAX", "10"))
	if err != nil || rateLimitAuth < 1 {
		return Config{}, errors.New("invalid RATE_LIMIT_AUTH_MAX: must be a positive integer")
	}
	rateLimitAuthPeriod, err := parsePeriod(getEnv("RATE_LIMIT_AUTH_PERIOD", "minute"))
	if err != nil {
		return Config{}, errors.New("invalid RATE_LIMIT_AUTH_PERIOD: must be second, minute, hour, or day")
	}
	rateLimitAPI, err := strconv.Atoi(getEnv("RATE_LIMIT_API_MAX", "100"))
	if err != nil || rateLimitAPI < 1 {
		return Config{}, errors.New("invalid RATE_LIMIT_API_MAX: must be a positive integer")
	}
	rateLimitAPIPeriod, err := parsePeriod(getEnv("RATE_LIMIT_API_PERIOD", "minute"))
	if err != nil {
		return Config{}, errors.New("invalid RATE_LIMIT_API_PERIOD: must be second, minute, hour, or day")
	}

	cfg := Config{
		ServerPort:          getEnv("SERVER_PORT", "8080"),
		AppEnv:              getEnv("APP_ENV", "development"),
		AppBaseURL:          getEnv("APP_BASE_URL", "http://localhost:8080"),
		FrontendURL:         getEnv("FRONTEND_URL", "http://localhost:3000"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		JWTRefreshSecret:    os.Getenv("JWT_REFRESH_SECRET"),
		JWTExpiry:           time.Duration(jwtExpiry * float64(time.Hour)),
		JWTRefreshExpiry:    time.Duration(jwtRefreshExpiry * float64(time.Hour)),
		EmailVerifyExpiry:   time.Duration(emailVerifyExpiry * float64(time.Hour)),
		PasswordResetExpiry: time.Duration(passwordResetExpiry * float64(time.Hour)),
		SMTPHost:            os.Getenv("SMTP_HOST"),
		SMTPPort:            getEnv("SMTP_PORT", "587"),
		SMTPUsername:        os.Getenv("SMTP_USERNAME"),
		SMTPPassword:        os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:            getEnv("SMTP_FROM", "no-reply@azhar.test"),
		XenditAPIKey:        os.Getenv("XENDIT_API_KEY"),
		XenditCallbackToken: os.Getenv("XENDIT_CALLBACK_TOKEN"),
		RateLimitAuth:       rateLimitAuth,
		RateLimitAuthPeriod: rateLimitAuthPeriod,
		RateLimitAPI:        rateLimitAPI,
		RateLimitAPIPeriod:  rateLimitAPIPeriod,
		CORSAllowedOrigins:  strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "*"), ","),
		AdminEmail:          os.Getenv("ADMIN_EMAIL"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}
	if cfg.JWTRefreshSecret == "" {
		return Config{}, errors.New("JWT_REFRESH_SECRET is required")
	}
	if cfg.XenditAPIKey == "" {
		return Config{}, errors.New("XENDIT_API_KEY is required")
	}
	if cfg.XenditCallbackToken == "" {
		return Config{}, errors.New("XENDIT_CALLBACK_TOKEN is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parsePeriod(s string) (time.Duration, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "second":
		return time.Second, nil
	case "minute":
		return time.Minute, nil
	case "hour":
		return time.Hour, nil
	case "day":
		return 24 * time.Hour, nil
	default:
		return 0, errors.New("unknown period")
	}
}
