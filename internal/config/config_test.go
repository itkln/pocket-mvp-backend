package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestPasswordResetProductConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/pocket_test")
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	t.Setenv("DATA_ENCRYPTION_KEY", key)
	t.Setenv("DATA_LOOKUP_KEY", key)
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_BASE_URL", "https://legacy-vendor.example")
	t.Setenv("VENDOR_APP_BASE_URL", "")
	t.Setenv("DISH_APP_BASE_URL", "")
	t.Setenv("SPOTS_APP_BASE_URL", "")
	t.Setenv("EATS_APP_BASE_URL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppBaseURL != "https://legacy-vendor.example" || cfg.SpotsAppBaseURL != "http://localhost:3001" {
		t.Fatalf("unexpected defaults: vendor=%s spots=%s", cfg.AppBaseURL, cfg.SpotsAppBaseURL)
	}
	t.Setenv("VENDOR_APP_BASE_URL", "https://vendor.example")
	t.Setenv("SPOTS_APP_BASE_URL", "https://spots.example")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppBaseURL != "https://vendor.example" || cfg.SpotsAppBaseURL != "https://spots.example" {
		t.Fatal("explicit product origins not used")
	}
	t.Setenv("SPOTS_APP_BASE_URL", "")
	t.Setenv("EATS_APP_BASE_URL", "https://legacy-spots.example")
	cfg, err = Load()
	if err != nil || cfg.SpotsAppBaseURL != "https://legacy-spots.example" {
		t.Fatalf("legacy Eats origin was not migrated: value=%q err=%v", cfg.SpotsAppBaseURL, err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("SMTP_ADDRESS", "smtp.example:587")
	t.Setenv("SPOTS_APP_BASE_URL", "")
	t.Setenv("EATS_APP_BASE_URL", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SPOTS_APP_BASE_URL") {
		t.Fatal("production must explicitly configure the Spots origin")
	}
}

func TestDishOriginTakesPrecedence(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/pocket_test")
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	t.Setenv("DATA_ENCRYPTION_KEY", key)
	t.Setenv("DATA_LOOKUP_KEY", key)
	t.Setenv("APP_ENV", "development")
	t.Setenv("DISH_APP_BASE_URL", "https://dish.example")
	t.Setenv("SPOTS_APP_BASE_URL", "https://old-spots.example")
	cfg, err := Load()
	if err != nil || cfg.SpotsAppBaseURL != "https://dish.example" {
		t.Fatalf("Dish URL: %s %v", cfg.SpotsAppBaseURL, err)
	}
}
