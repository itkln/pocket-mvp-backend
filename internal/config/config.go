package config

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment            string
	HTTPAddress            string
	DatabaseURL            string
	DatabaseMaxConnections int32
	DatabaseConnectTimeout time.Duration
	ShutdownTimeout        time.Duration
	AllowedOrigins         []string
	LogLevel               slog.Level
	DataEncryptionKey      []byte
	DataLookupKey          []byte
	SessionCookieName      string
	SessionTTL             time.Duration
	CookieSecure           bool
}

func Load() (Config, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	maxConnections, err := int32Env("DATABASE_MAX_CONNECTIONS", 10)
	if err != nil {
		return Config{}, err
	}

	connectTimeout, err := durationEnv("DATABASE_CONNECT_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := durationEnv("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := durationEnv("SESSION_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	encryptionKey, err := base64KeyEnv("DATA_ENCRYPTION_KEY", 32)
	if err != nil {
		return Config{}, err
	}
	lookupKey, err := base64KeyEnv("DATA_LOOKUP_KEY", 32)
	if err != nil {
		return Config{}, err
	}
	cookieSecure, err := boolEnv("COOKIE_SECURE", false)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Environment:            stringEnv("APP_ENV", "development"),
		HTTPAddress:            stringEnv("HTTP_ADDRESS", ":8080"),
		DatabaseURL:            databaseURL,
		DatabaseMaxConnections: maxConnections,
		DatabaseConnectTimeout: connectTimeout,
		ShutdownTimeout:        shutdownTimeout,
		AllowedOrigins:         csvEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		LogLevel:               logLevelEnv("LOG_LEVEL", slog.LevelInfo),
		DataEncryptionKey:      encryptionKey,
		DataLookupKey:          lookupKey,
		SessionCookieName:      stringEnv("SESSION_COOKIE_NAME", "pocket_session"),
		SessionTTL:             sessionTTL,
		CookieSecure:           cookieSecure,
	}, nil
}

func base64KeyEnv(key string, minimumBytes int) ([]byte, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, errors.New(key + " is required")
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) < minimumBytes {
		return nil, errors.New(key + " must be base64 encoding of at least " + strconv.Itoa(minimumBytes) + " bytes")
	}
	return decoded, nil
}

func boolEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.New(key + " must be true or false")
	}
	return parsed, nil
}

func stringEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func csvEnv(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func int32Env(key string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 1 {
		return 0, errors.New(key + " must be a positive integer")
	}
	return int32(parsed), nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New(key + " must be a positive duration")
	}
	return parsed, nil
}

func logLevelEnv(key string, fallback slog.Level) slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info":
		return slog.LevelInfo
	default:
		return fallback
	}
}
