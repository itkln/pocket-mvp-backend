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
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppBaseURL != "https://legacy-vendor.example" || cfg.DishAppBaseURL != "http://localhost:3001" {
		t.Fatalf("unexpected defaults: vendor=%s dish=%s", cfg.AppBaseURL, cfg.DishAppBaseURL)
	}
	t.Setenv("VENDOR_APP_BASE_URL", "https://vendor.example")
	t.Setenv("DISH_APP_BASE_URL", "https://dish.example")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppBaseURL != "https://vendor.example" || cfg.DishAppBaseURL != "https://dish.example" {
		t.Fatal("explicit product origins not used")
	}
	// The Spots and Eats names are no longer read.
	t.Setenv("DISH_APP_BASE_URL", "")
	t.Setenv("SPOTS_APP_BASE_URL", "https://old-spots.example")
	t.Setenv("EATS_APP_BASE_URL", "https://old-eats.example")
	cfg, err = Load()
	if err != nil || cfg.DishAppBaseURL != "http://localhost:3001" {
		t.Fatalf("legacy origin was read: value=%q err=%v", cfg.DishAppBaseURL, err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("SMTP_ADDRESS", "smtp.example:587")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DISH_APP_BASE_URL") {
		t.Fatal("production must explicitly configure the Dish origin")
	}
}
