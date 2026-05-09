package config

import (
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
	RateLimitPerMinute int
	MySQLDSN           string
	FrontendURL        string
	AlipaySandbox      bool
	AlipayAppID        string
	AlipayPrivateKey   string
	AlipayPublicKey    string
	AlipayNotifyURL    string
	AlipayReturnURL    string
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
		RateLimitPerMinute: getEnvAsInt("RATE_LIMIT_PER_MINUTE", 120),
		MySQLDSN:           getEnv("MYSQL_DSN", "root:123456@tcp(127.0.0.1:3306)/saas_billing?charset=utf8mb4&parseTime=True&loc=Local"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
		AlipaySandbox:      getEnvAsBool("ALIPAY_SANDBOX", true),
		AlipayAppID:        getEnv("ALIPAY_APP_ID", ""),
		AlipayPrivateKey:   normalizePEM(getEnv("ALIPAY_APP_PRIVATE_KEY", "")),
		AlipayPublicKey:    normalizePEM(getEnv("ALIPAY_PUBLIC_KEY", "")),
		AlipayNotifyURL:    getEnv("ALIPAY_NOTIFY_URL", "http://localhost:8080/api/v1/payments/callback/alipay"),
		AlipayReturnURL:    getEnv("ALIPAY_RETURN_URL", "http://localhost:5173/payment/return"),
	}

	return cfg, nil
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
