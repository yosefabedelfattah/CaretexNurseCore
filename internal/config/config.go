package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv     string
	AppPort    string
	AppBaseURL string
	LogLevel   string

	AllowedOrigins []string

	DBHost         string
	DBPort         string
	DBName         string
	DBUser         string
	DBPassword     string
	DBSSLMode      string
	DBMaxOpenConns int
	DBMaxIdleConns int

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration

	// Caretex external API integration
	// Auth: Authorization: Bearer <KeyID>:<Secret>
	// Base URL example: https://caretex.me:8080/api/v1/external
	CaretxBaseURL     string
	CaretxKeyID       string
	CaretxSecret      string
	CaretxTimeout     time.Duration

	RateLimitRPS int
}

func Load() (*Config, error) {
	_ = godotenv.Load() // optional .env

	c := &Config{
		AppEnv:     getEnv("APP_ENV", "development"),
		AppPort:    getEnv("APP_PORT", "8080"),
		AppBaseURL: getEnv("APP_BASE_URL", "http://localhost:8080"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),

		AllowedOrigins: splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:4200")),

		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBName:         getEnv("DB_NAME", "caretexnurse"),
		DBUser:         getEnv("DB_USER", "dev"),
		DBPassword:     getEnv("DB_PASSWORD", "dev"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		DBMaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 50),
		DBMaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 10),

		JWTAccessSecret:  getEnv("JWT_ACCESS_SECRET", ""),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", ""),
		JWTAccessTTL:     getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:    getEnvDuration("JWT_REFRESH_TTL", 720*time.Hour),

		CaretxBaseURL:      getEnv("CARETX_BASE_URL", ""),
		CaretxKeyID:        getEnv("CARETX_KEY_ID", ""),
		CaretxSecret:       getEnv("CARETX_SECRET", ""),
		CaretxTimeout:      getEnvDuration("CARETX_TIMEOUT", 15*time.Second),

		RateLimitRPS: getEnvInt("RATE_LIMIT_RPS", 20),
	}

	if c.JWTAccessSecret == "" || c.JWTRefreshSecret == "" {
		if c.AppEnv == "production" {
			return nil, errors.New("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must be set in production")
		}
		// Dev fallback (NEVER use in prod)
		c.JWTAccessSecret = "dev-access-secret-change-me"
		c.JWTRefreshSecret = "dev-refresh-secret-change-me"
	}
	return c, nil
}

func (c *Config) DatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
