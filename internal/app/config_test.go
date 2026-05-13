package app_test

import (
	"strings"
	"testing"

	"github.com/Zhaba1337228/max-university-event-bot/internal/app"
)

// setEnv упрощает выставление и автоматическую очистку env в тестах.
func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

// minEnv возвращает обязательный минимум для успешной загрузки.
func minEnv() map[string]string {
	return map[string]string{
		"MAX_BOT_TOKEN": "stub-token-just-for-tests",
		"DATABASE_URL":  "postgres://app:app@localhost:5432/db?sslmode=disable",
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	setEnv(t, minEnv())

	cfg, err := app.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Max.Mode != "longpoll" {
		t.Errorf("Max.Mode default: want longpoll, got %s", cfg.Max.Mode)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("HTTP.Addr default: want :8080, got %s", cfg.HTTP.Addr)
	}
	if cfg.Admin.APIAddr != ":8081" {
		t.Errorf("Admin.APIAddr default: want :8081, got %s", cfg.Admin.APIAddr)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level default: want info, got %s", cfg.Log.Level)
	}
	if !cfg.Business.WaitlistEnabled {
		t.Error("Waitlist must be enabled by default")
	}
}

func TestLoadConfigMissingToken(t *testing.T) {
	setEnv(t, map[string]string{
		"DATABASE_URL": "postgres://app:app@localhost:5432/db?sslmode=disable",
	})

	_, err := app.LoadConfig()
	if err == nil {
		t.Fatal("want error when MAX_BOT_TOKEN missing")
	}
}

func TestLoadConfigMissingDB(t *testing.T) {
	setEnv(t, map[string]string{
		"MAX_BOT_TOKEN": "stub",
	})

	_, err := app.LoadConfig()
	if err == nil {
		t.Fatal("want error when DATABASE_URL missing")
	}
}

func TestValidateInvalidMode(t *testing.T) {
	env := minEnv()
	env["MAX_BOT_MODE"] = "weird"
	setEnv(t, env)

	_, err := app.LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "MAX_BOT_MODE") {
		t.Fatalf("want MAX_BOT_MODE error, got %v", err)
	}
}

func TestValidateWebhookRequiresSecretAndURL(t *testing.T) {
	env := minEnv()
	env["MAX_BOT_MODE"] = "webhook"
	setEnv(t, env)

	_, err := app.LoadConfig()
	if err == nil {
		t.Fatal("want error: webhook mode without URL")
	}

	env["MAX_BOT_WEBHOOK_URL"] = "https://example.com/webhook/max"
	setEnv(t, env)

	_, err = app.LoadConfig()
	if err == nil {
		t.Fatal("want error: webhook mode without secret")
	}

	env["MAX_BOT_WEBHOOK_SECRET"] = "short"
	setEnv(t, env)
	if _, err := app.LoadConfig(); err == nil {
		t.Fatal("want error: webhook secret < 16 chars")
	}

	env["MAX_BOT_WEBHOOK_SECRET"] = "this-secret-is-long-enough"
	setEnv(t, env)
	if _, err := app.LoadConfig(); err != nil {
		t.Fatalf("ok webhook config must validate: %v", err)
	}
}

func TestValidateAdminSessionKey(t *testing.T) {
	env := minEnv()
	env["ADMIN_SESSION_KEY"] = "tooshort"
	setEnv(t, env)

	if _, err := app.LoadConfig(); err == nil {
		t.Fatal("want error: admin session key < 32 chars")
	}

	env["ADMIN_SESSION_KEY"] = strings.Repeat("a", 32)
	setEnv(t, env)
	if _, err := app.LoadConfig(); err != nil {
		t.Fatalf("ok admin session key must validate: %v", err)
	}
}

func TestValidateRateLimit(t *testing.T) {
	env := minEnv()
	env["NOTIFICATION_RATE_LIMIT_RPS"] = "0"
	setEnv(t, env)

	if _, err := app.LoadConfig(); err == nil {
		t.Fatal("want error: rate limit = 0")
	}

	env["NOTIFICATION_RATE_LIMIT_RPS"] = "31"
	setEnv(t, env)
	if _, err := app.LoadConfig(); err == nil {
		t.Fatal("want error: rate limit > 30")
	}
}

func TestConfigStringMasksSecrets(t *testing.T) {
	setEnv(t, map[string]string{
		"MAX_BOT_TOKEN":     "REAL_SUPER_SECRET_TOKEN_VALUE",
		"DATABASE_URL":      "postgres://user:my-real-password@host:5432/db",
		"ADMIN_SESSION_KEY": strings.Repeat("k", 64),
		"GIGACHAT_AUTH_KEY": "GIGA_SECRET_AUTHKEY_VALUE_LONG_ENOUGH",
	})

	cfg, err := app.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	s := cfg.String()
	leak := []string{
		"REAL_SUPER_SECRET_TOKEN_VALUE",
		"my-real-password",
		"GIGA_SECRET_AUTHKEY_VALUE_LONG_ENOUGH",
		strings.Repeat("k", 64),
	}
	for _, secret := range leak {
		if strings.Contains(s, secret) {
			t.Errorf("Config.String leaks secret %q: %s", secret, s)
		}
	}
	if !strings.Contains(s, "***") {
		t.Errorf("Config.String should contain masked '***': %s", s)
	}
}
