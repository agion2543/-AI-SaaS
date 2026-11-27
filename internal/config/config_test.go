package config

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestValidateAllowsDevelopmentDefaults(t *testing.T) {
	cfg := &Config{
		AppEnv:                         "development",
		JWTSecret:                      "replace-with-a-very-strong-secret",
		AESSecret:                      "0123456789abcdef0123456789abcdef",
		MySQLDSN:                       "root:123456@tcp(127.0.0.1:3306)/saas_billing",
		OTPSendRateLimitPerIPPerMinute: DefaultOTPSendRateLimitPerIPPerMinute,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("development defaults should remain available: %v", err)
	}
}

func TestValidateRequiresExactly32AESSecretBytesInEveryEnvironment(t *testing.T) {
	for _, size := range []int{0, 31, 32, 33, 64} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			cfg := &Config{
				AppEnv:                         "development",
				AESSecret:                      strings.Repeat("a", size),
				OTPSendRateLimitPerIPPerMinute: DefaultOTPSendRateLimitPerIPPerMinute,
			}
			err := cfg.Validate()
			if size == 32 && err != nil {
				t.Fatalf("exactly 32 AES bytes should be accepted in development: %v", err)
			}
			if size != 32 && err == nil {
				t.Fatalf("%d AES bytes should be rejected", size)
			}
		})
	}
}

func TestLoadDoesNotSupplyAnInitialAdminPassword(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("INITIAL_ADMIN_PASSWORD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load without an initial admin password: %v", err)
	}
	if cfg.InitialAdminPass != "" {
		t.Fatal("Load supplied an initial admin password when none was configured")
	}
	if !cfg.DevPasswordResetEnabled {
		t.Fatal("explicit APP_ENV=development should enable the development-only reset route")
	}
}

func TestLoadAllowsOmittedInitialAdminPasswordInDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	previous, wasSet := os.LookupEnv("INITIAL_ADMIN_PASSWORD")
	if err := os.Unsetenv("INITIAL_ADMIN_PASSWORD"); err != nil {
		t.Fatalf("unset INITIAL_ADMIN_PASSWORD: %v", err)
	}
	t.Cleanup(func() {
		if wasSet {
			if err := os.Setenv("INITIAL_ADMIN_PASSWORD", previous); err != nil {
				t.Errorf("restore INITIAL_ADMIN_PASSWORD: %v", err)
			}
		} else if err := os.Unsetenv("INITIAL_ADMIN_PASSWORD"); err != nil {
			t.Errorf("restore absent INITIAL_ADMIN_PASSWORD: %v", err)
		}
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load without INITIAL_ADMIN_PASSWORD in development: %v", err)
	}
	if cfg.InitialAdminPass != "" {
		t.Fatal("Load supplied an initial admin password when the variable was omitted")
	}
}

func TestLoadDisablesDevResetWhenAppEnvUsesDevelopmentDefault(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("AES_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("OTP_SEND_RATE_LIMIT_PER_IP_PER_MINUTE", "10")
	t.Setenv("INITIAL_ADMIN_PASSWORD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with omitted APP_ENV: %v", err)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("omitted APP_ENV default=%q, want development", cfg.AppEnv)
	}
	if cfg.DevPasswordResetEnabled {
		t.Fatal("implicit development default must not enable the public development password-reset route")
	}
}

func TestLoadOTPSendRateLimitDefaultsAndCanBeConfigured(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("OTP_SEND_RATE_LIMIT_PER_IP_PER_MINUTE", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with default OTP send IP limit: %v", err)
	}
	if cfg.OTPSendRateLimitPerIPPerMinute != DefaultOTPSendRateLimitPerIPPerMinute {
		t.Fatalf("default OTP send IP limit=%d, want %d", cfg.OTPSendRateLimitPerIPPerMinute, DefaultOTPSendRateLimitPerIPPerMinute)
	}

	t.Setenv("OTP_SEND_RATE_LIMIT_PER_IP_PER_MINUTE", "7")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load with configured OTP send IP limit: %v", err)
	}
	if cfg.OTPSendRateLimitPerIPPerMinute != 7 {
		t.Fatalf("configured OTP send IP limit=%d, want 7", cfg.OTPSendRateLimitPerIPPerMinute)
	}
}

func TestValidateRejectsNonPositiveOTPSendRateLimit(t *testing.T) {
	for _, value := range []int{0, -1} {
		cfg := &Config{AppEnv: "development", AESSecret: "0123456789abcdef0123456789abcdef", OTPSendRateLimitPerIPPerMinute: value}
		if err := cfg.Validate(); err == nil {
			t.Errorf("OTP send IP limit %d must be rejected", value)
		}
	}
}

func TestLoadTrustedProxiesDefaultsDisabledAndParsesList(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with no trusted proxies: %v", err)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Fatalf("trusted proxies must default to empty; got %v", cfg.TrustedProxies)
	}

	t.Setenv("TRUSTED_PROXIES", "10.24.0.0/16, 2001:db8::5, ,")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load with configured trusted proxies: %v", err)
	}
	want := []string{"10.24.0.0/16", "2001:db8::5"}
	if len(cfg.TrustedProxies) != len(want) || cfg.TrustedProxies[0] != want[0] || cfg.TrustedProxies[1] != want[1] {
		t.Fatalf("trusted proxies=%v, want %v", cfg.TrustedProxies, want)
	}
}

func TestValidateRejectsInvalidOrWildcardTrustedProxies(t *testing.T) {
	for _, proxy := range []string{"not-an-ip", "10.24.0.0/99", "0.0.0.0/0", "::/0"} {
		t.Run(proxy, func(t *testing.T) {
			cfg := &Config{
				AppEnv:                         "development",
				AESSecret:                      "0123456789abcdef0123456789abcdef",
				OTPSendRateLimitPerIPPerMinute: DefaultOTPSendRateLimitPerIPPerMinute,
				TrustedProxies:                 []string{proxy},
			}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("invalid or wildcard trusted proxy %q must be rejected", proxy)
			}
		})
	}
}

func TestLoadDBAutoMigrateDefaultsDisabledAndCanBeEnabled(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DB_AUTO_MIGRATE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with default DB_AUTO_MIGRATE: %v", err)
	}
	if cfg.DBAutoMigrate {
		t.Fatal("DB_AUTO_MIGRATE should default to false: the versioned schema migrator is the single schema path")
	}

	t.Setenv("DB_AUTO_MIGRATE", "true")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load with DB_AUTO_MIGRATE=true: %v", err)
	}
	if !cfg.DBAutoMigrate {
		t.Fatal("DB_AUTO_MIGRATE=true should enable app-side AutoMigrate")
	}
}

func TestLoadInitialDataSeedEnabledDefaultsTrueAndCanBeDisabled(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("INITIAL_DATA_SEED_ENABLED", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with default INITIAL_DATA_SEED_ENABLED: %v", err)
	}
	if !cfg.InitialDataSeedEnabled {
		t.Fatal("INITIAL_DATA_SEED_ENABLED should default to true to preserve initial-install behavior")
	}

	t.Setenv("INITIAL_DATA_SEED_ENABLED", "false")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load with INITIAL_DATA_SEED_ENABLED=false: %v", err)
	}
	if cfg.InitialDataSeedEnabled {
		t.Fatal("INITIAL_DATA_SEED_ENABLED=false should disable startup seed writes")
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
		{"deployment guide JWT placeholder", func(c *Config) { c.JWTSecret = "replace-with-a-long-random-jwt-secret" }},
		{"default AES secret", func(c *Config) { c.AESSecret = "0123456789abcdef0123456789abcdef" }},
		{"wrong length AES secret", func(c *Config) { c.AESSecret = "too-short" }},
		{"32-byte placeholder AES secret", func(c *Config) { c.AESSecret = "replace-with-placeholder-aes-key" }},
		{"deployment guide AES placeholder length", func(c *Config) { c.AESSecret = "replace-with-a-32-byte-random-key" }},
		{"weak admin password", func(c *Config) { c.InitialAdminPass = "short" }},
		{"example initial admin placeholder", func(c *Config) { c.InitialAdminPass = "replace-with-a-strong-initial-admin-password" }},
		{"default database credentials", func(c *Config) { c.MySQLDSN = "root:123456@tcp(mysql:3306)/saas_billing" }},
		{"deployment guide database password placeholder", func(c *Config) {
			c.MySQLDSN = "saas_user:replace-with-a-strong-db-password@tcp(mysql:3306)/saas_billing?charset=utf8mb4&parseTime=true&loc=Local"
		}},
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

func TestValidateProductionSecretByteLengthBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		jwtBytes int
		aesBytes int
		wantErr  bool
	}{
		{name: "minimum valid lengths", jwtBytes: 32, aesBytes: 32},
		{name: "JWT one byte short", jwtBytes: 31, aesBytes: 32, wantErr: true},
		{name: "AES one byte short", jwtBytes: 32, aesBytes: 31, wantErr: true},
		{name: "AES one byte long", jwtBytes: 32, aesBytes: 33, wantErr: true},
		{name: "AES 64-byte hex encoding", jwtBytes: 32, aesBytes: 64, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validProductionConfig()
			cfg.JWTSecret = strings.Repeat("j", tc.jwtBytes)
			cfg.AESSecret = strings.Repeat("a", tc.aesBytes)
			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected production configuration to reject out-of-bound secret length")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("valid boundary secret lengths were rejected: %v", err)
			}
		})
	}
}

func TestValidateProductionAllowsMissingInitialPasswordUntilBootstrap(t *testing.T) {
	cfg := validProductionConfig()
	cfg.InitialAdminPass = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production config without an initial password should reach bootstrap: %v", err)
	}
}

func validProductionConfig() *Config {
	return &Config{
		AppEnv:                         "production",
		JWTSecret:                      "generated-jwt-secret-with-at-least-32-characters-2026",
		AESSecret:                      "1234567890abcdef0123456789abcdef",
		InitialAdminPass:               "distinct-Strong-Admin-Password-2026!",
		MySQLDSN:                       "saas_user:randomized-database-password@tcp(mysql:3306)/saas_billing?charset=utf8mb4",
		OTPSendRateLimitPerIPPerMinute: DefaultOTPSendRateLimitPerIPPerMinute,
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
