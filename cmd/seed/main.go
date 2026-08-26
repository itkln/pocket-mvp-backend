package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"pocket-mvp-backend/internal/config"
	"pocket-mvp-backend/internal/database"
	"pocket-mvp-backend/internal/demodata"
	"pocket-mvp-backend/internal/security"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	if cfg.Environment != "development" && !strings.EqualFold(os.Getenv("ALLOW_DEMO_SEED"), "true") {
		logger.Error("demo seed is disabled outside development", "environment", cfg.Environment)
		os.Exit(1)
	}

	protector, err := security.NewDataProtector(cfg.DataEncryptionKey, cfg.DataLookupKey)
	if err != nil {
		logger.Error("initialize data protection", "error", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConnections)
	if err != nil {
		logger.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	email := envOrDefault("DEMO_EMAIL", demodata.DefaultEmail)
	password := envOrDefault("DEMO_PASSWORD", demodata.DefaultPassword)
	if err = demodata.New(db, protector, email, password).Run(ctx); err != nil {
		logger.Error("seed demo workspace", "error", err)
		os.Exit(1)
	}
	logger.Info("demo workspace is ready", "email", email)
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
