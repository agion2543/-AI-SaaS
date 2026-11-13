package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName            string
	AppEnv             string
	AppHost            string
	AppPort            string
	AppURL             string
	JWTSecret          string
	AESSecret          string
	InitialAdminUser   string
	InitialAdminPass   string
	RateLimitPerMinute int
	MySQLDSN           string
	DBAutoMigrate      bool
	FrontendURL        string
	AlipaySandbox      bool
	AlipayAppID        string
	AlipayPrivateKey   string
	AlipayPublicKey    string
	AlipayNotifyURL    string
	AlipayReturnURL    string
	AIEnabled          bool
	AIProvider         string
	AIBaseURL          string
	AIAPIKey           string
	AIModel            string
	AITimeoutSeconds   int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppName:            getEnv("APP_NAME", "SaaS Billing Platform"),
		AppEnv:             getEnv("APP_ENV", "development"),
		AppHost:            getEnv("APP_HOST", "0.0.0.0"),
		AppPort:            getEnv("APP_PORT", "8080"),
		AppURL:             getEnv("APP_URL", "http://localhost:8080"),
		JWTSecret:          getEnv("JWT_SECRET", "replace-with-a-very-strong-secret"),
		AESSecret:          getEnv("AES_SECRET", "0123456789abcdef0123456789abcdef"),
		InitialAdminUser:   getEnv("INITIAL_ADMIN_USERNAME", "admin"),
		InitialAdminPass:   getEnv("INITIAL_ADMIN_PASSWORD", "SaasAdmin@2026!"),
		RateLimitPerMinute: getEnvAsInt("RATE_LIMIT_PER_MINUTE", 120),
		MySQLDSN:           getEnv("MYSQL_DSN", "root:123456@tcp(127.0.0.1:3306)/saas_billing?charset=utf8mb4&parseTime=True&loc=Local"),
		DBAutoMigrate:      getEnvAsBool("DB_AUTO_MIGRATE", true),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
		AlipaySandbox:      getEnvAsBool("ALIPAY_SANDBOX", true),
		AlipayAppID:        getEnv("ALIPAY_APP_ID", ""),
		AlipayPrivateKey:   normalizePEM(getEnv("ALIPAY_APP_PRIVATE_KEY", "")),
		AlipayPublicKey:    normalizePEM(getEnv("ALIPAY_PUBLIC_KEY", "")),
		AlipayNotifyURL:    getEnv("ALIPAY_NOTIFY_URL", "http://localhost:8080/api/v1/payments/callback/alipay"),
		AlipayReturnURL:    getEnv("ALIPAY_RETURN_URL", "http://localhost:5173/payment/return"),
		AIEnabled:          getEnvAsBool("AI_ENABLED", false),
		AIProvider:         getEnv("AI_PROVIDER", "template"),
		AIBaseURL:          strings.TrimRight(getEnv("AI_BASE_URL", ""), "/"),
		AIAPIKey:           getEnv("AI_API_KEY", ""),
		AIModel:            getEnv("AI_MODEL", "deepseek-chat"),
		AITimeoutSeconds:   getEnvAsInt("AI_TIMEOUT_SECONDS", 20),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate rejects unsafe development defaults when the service is configured
// to run in production. Development defaults remain available for local use.
func (c *Config) Validate() error {
	if !strings.EqualFold(strings.TrimSpace(c.AppEnv), "production") {
		return nil
	}

	if len(c.JWTSecret) < 32 || isPlaceholderValue(c.JWTSecret) || c.JWTSecret == "replace-with-a-very-strong-secret" {
		return errors.New("unsafe production configuration: JWT_SECRET must be a non-placeholder secret of at least 32 characters")
	}
	if len(c.AESSecret) != 32 || isPlaceholderValue(c.AESSecret) || c.AESSecret == "0123456789abcdef0123456789abcdef" {
		return errors.New("unsafe production configuration: AES_SECRET must be a non-placeholder 32-character secret")
	}
	if len(c.InitialAdminPass) < 12 || isPlaceholderValue(c.InitialAdminPass) || c.InitialAdminPass == "SaasAdmin@2026!" {
		return errors.New("unsafe production configuration: INITIAL_ADMIN_PASSWORD must be a non-placeholder password of at least 12 characters")
	}
	dsn := strings.ToLower(c.MySQLDSN)
	if strings.TrimSpace(c.MySQLDSN) == "" || strings.Contains(dsn, "root:123456@") || isPlaceholderValue(c.MySQLDSN) {
		return errors.New("unsafe production configuration: MYSQL_DSN must be explicitly configured without the local default credentials")
	}

	return nil
}

func isPlaceholderValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, marker := range []string{"replace-with", "your-", "change-me", "changeme", "placeholder"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	num, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return num
}

func getEnvAsBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "TRUE", "True", "yes", "YES":
		return true
	case "0", "false", "FALSE", "False", "no", "NO":
		return false
	default:
		return fallback
	}
}

func normalizePEM(value string) string {
	return strings.ReplaceAll(value, "\\n", "\n")
}
