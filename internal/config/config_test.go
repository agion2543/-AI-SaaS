package config

import (
	"strings"
	"testing"
)

func TestValidateAllowsDevelopmentDefaults(t *testing.T) {
	cfg := &Config{
		AppEnv:           "development",
		JWTSecret:        "replace-with-a-very-strong-secret",
		AESSecret:        "0123456789abcdef0123456789abcdef",
		InitialAdminPass: "SaasAdmin@2026!",
		MySQLDSN:         "root:123456@tcp(127.0.0.1:3306)/saas_billing",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("development defaults should remain available: %v", err)
	}
}

func TestLoadDBAutoMigrateDefaultsEnabledAndCanBeDisabled(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DB_AUTO_MIGRATE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with default DB_AUTO_MIGRATE: %v", err)
	}
	if !cfg.DBAutoMigrate {
		t.Fatal("DB_AUTO_MIGRATE should default to true")
	}

	t.Setenv("DB_AUTO_MIGRATE", "false")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load with DB_AUTO_MIGRATE=false: %v", err)
	}
	if cfg.DBAutoMigrate {
		t.Fatal("DB_AUTO_MIGRATE=false should disable app-side AutoMigrate")
	}
}

func TestValidateProductionRejectsUnsafeConfiguration(t *testing.T) {
	base := validProductionConfig()
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"default JWT secret", func(c *Config) { c.JWTSecret = "replace-with-a-very-strong-secret" }},
		{"short JWT secret", func(c *Config) { c.JWTSecret = "short-secret" }},
		{"placeholder JWT secret", func(c *Config) { c.JWTSecret = "your-production-jwt-secret-that-is-long-enough" }},
		{"default AES secret", func(c *Config) { c.AESSecret = "0123456789abcdef0123456789abcdef" }},
		{"wrong length AES secret", func(c *Config) { c.AESSecret = "too-short" }},
		{"placeholder AES secret", func(c *Config) { c.AESSecret = "replace-with-32-char-production-key" }},
		{"default admin password", func(c *Config) { c.InitialAdminPass = "SaasAdmin@2026!" }},
		{"weak admin password", func(c *Config) { c.InitialAdminPass = "short" }},
		{"placeholder admin password", func(c *Config) { c.InitialAdminPass = "replace-with-initial-admin-password" }},
		{"default database credentials", func(c *Config) { c.MySQLDSN = "root:123456@tcp(mysql:3306)/saas_billing" }},
		{"placeholder database password", func(c *Config) { c.MySQLDSN = "saas_user:replace-with-db-password@tcp(mysql:3306)/saas_billing" }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := *base
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected unsafe production configuration to be rejected")
			}
		})
	}
}

func TestValidateProductionAcceptsConfiguredSecrets(t *testing.T) {
	if err := validProductionConfig().Validate(); err != nil {
		t.Fatalf("valid production configuration was rejected: %v", err)
	}
}

func validProductionConfig() *Config {
	return &Config{
		AppEnv:           "production",
		JWTSecret:        "generated-jwt-secret-with-at-least-32-characters-2026",
		AESSecret:        "1234567890abcdef1234567890abcdef",
		InitialAdminPass: "distinct-Strong-Admin-Password-2026!",
		MySQLDSN:         "saas_user:randomized-database-password@tcp(mysql:3306)/saas_billing?charset=utf8mb4",
	}
}

func TestPlaceholderDetectionIsCaseInsensitiveAndTrimsWhitespace(t *testing.T) {
	if !isPlaceholderValue("  Replace-With-a-random-secret  ") {
		t.Fatal("expected placeholder marker to be detected")
	}
	if isPlaceholderValue(strings.Repeat("a", 40)) {
		t.Fatal("unexpectedly detected an ordinary secret as a placeholder")
	}
}
