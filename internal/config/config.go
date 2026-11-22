package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const DefaultOTPSendRateLimitPerIPPerMinute = 10

type Config struct {
	AppName string
	AppEnv  string
	// DevPasswordResetEnabled is true only when APP_ENV explicitly selects development.
	DevPasswordResetEnabled        bool
	AppHost                        string
	AppPort                        string
	AppURL                         string
	JWTSecret                      string
	AESSecret                      string
	InitialAdminUser               string
	InitialAdminPass               string
	RateLimitPerMinute             int
	MySQLDSN                       string
	DBAutoMigrate                  bool
	InitialDataSeedEnabled         bool
	FrontendURL                    string
	AlipaySandbox                  bool
	AlipayAppID                    string
	AlipayPrivateKey               string
	AlipayPublicKey                string
	AlipayNotifyURL                string
	AlipayReturnURL                string
	AIEnabled                      bool
	AIProvider                     string
	AIBaseURL                      string
	AIAPIKey                       string
	AIModel                        string
	AITimeoutSeconds               int
	OTPSendRateLimitPerIPPerMinute int
	TrustedProxies                 []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppName:                        getEnv("APP_NAME", "SaaS Billing Platform"),
		AppEnv:                         getEnv("APP_ENV", "development"),
		DevPasswordResetEnabled:        strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "development"),
		AppHost:                        getEnv("APP_HOST", "0.0.0.0"),
		AppPort:                        getEnv("APP_PORT", "8080"),
		AppURL:                         getEnv("APP_URL", "http://localhost:8080"),
		JWTSecret:                      getEnv("JWT_SECRET", "replace-with-a-very-strong-secret"),
		AESSecret:                      getEnv("AES_SECRET", "0123456789abcdef0123456789abcdef"),
		InitialAdminUser:               getEnv("INITIAL_ADMIN_USERNAME", "admin"),
		InitialAdminPass:               getEnv("INITIAL_ADMIN_PASSWORD", ""),
		RateLimitPerMinute:             getEnvAsInt("RATE_LIMIT_PER_MINUTE", 120),
		OTPSendRateLimitPerIPPerMinute: getEnvAsInt("OTP_SEND_RATE_LIMIT_PER_IP_PER_MINUTE", DefaultOTPSendRateLimitPerIPPerMinute),
		TrustedProxies:                 parseTrustedProxyList(getEnv("TRUSTED_PROXIES", "")),
		MySQLDSN:                       getEnv("MYSQL_DSN", "root:123456@tcp(127.0.0.1:3306)/saas_billing?charset=utf8mb4&parseTime=True&loc=Local"),
		DBAutoMigrate:                  getEnvAsBool("DB_AUTO_MIGRATE", true),
		InitialDataSeedEnabled:         getEnvAsBool("INITIAL_DATA_SEED_ENABLED", true),
		FrontendURL:                    getEnv("FRONTEND_URL", "http://localhost:5173"),
		AlipaySandbox:                  getEnvAsBool("ALIPAY_SANDBOX", true),
		AlipayAppID:                    getEnv("ALIPAY_APP_ID", ""),
		AlipayPrivateKey:               normalizePEM(getEnv("ALIPAY_APP_PRIVATE_KEY", "")),
		AlipayPublicKey:                normalizePEM(getEnv("ALIPAY_PUBLIC_KEY", "")),
		AlipayNotifyURL:                getEnv("ALIPAY_NOTIFY_URL", "http://localhost:8080/api/v1/payments/callback/alipay"),
		AlipayReturnURL:                getEnv("ALIPAY_RETURN_URL", "http://localhost:5173/payment/return"),
		AIEnabled:                      getEnvAsBool("AI_ENABLED", false),
		AIProvider:                     getEnv("AI_PROVIDER", "template"),
		AIBaseURL:                      strings.TrimRight(getEnv("AI_BASE_URL", ""), "/"),
		AIAPIKey:                       getEnv("AI_API_KEY", ""),
		AIModel:                        getEnv("AI_MODEL", "deepseek-chat"),
		AITimeoutSeconds:               getEnvAsInt("AI_TIMEOUT_SECONDS", 20),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate rejects unsafe production defaults and weak explicitly configured
// initial admin passwords. An empty initial password is handled by bootstrap:
// it is allowed for an existing installation but cannot create a new admin.
func (c *Config) Validate() error {
	if len(c.AESSecret) != 32 {
		return errors.New("AES_SECRET must be exactly 32 bytes")
	}
	if c.OTPSendRateLimitPerIPPerMinute < 1 {
		return errors.New("OTP_SEND_RATE_LIMIT_PER_IP_PER_MINUTE must be at least 1")
	}
	for _, proxy := range c.TrustedProxies {
		proxy = strings.TrimSpace(proxy)
		if strings.Contains(proxy, "/") {
			_, network, err := net.ParseCIDR(proxy)
			if err != nil {
				return fmt.Errorf("TRUSTED_PROXIES contains invalid IP/CIDR %q", proxy)
			}
			prefix, _ := network.Mask.Size()
			if prefix == 0 {
				return fmt.Errorf("TRUSTED_PROXIES must not contain wildcard CIDR %q", proxy)
			}
		} else if net.ParseIP(proxy) == nil {
			return fmt.Errorf("TRUSTED_PROXIES contains invalid IP/CIDR %q", proxy)
		}
	}
	initialAdminPass := strings.TrimSpace(c.InitialAdminPass)
	if initialAdminPass != "" && (len(initialAdminPass) < 12 || isPlaceholderValue(initialAdminPass)) {
		return errors.New("INITIAL_ADMIN_PASSWORD must be a non-placeholder password of at least 12 characters when creating an initial admin account")
	}

	if !strings.EqualFold(strings.TrimSpace(c.AppEnv), "production") {
		return nil
	}

	if len(c.JWTSecret) < 32 || isPlaceholderValue(c.JWTSecret) || c.JWTSecret == "replace-with-a-very-strong-secret" {
		return errors.New("unsafe production configuration: JWT_SECRET must be a non-placeholder secret of at least 32 characters")
	}
	if isPlaceholderValue(c.AESSecret) || c.AESSecret == "0123456789abcdef0123456789abcdef" {
		return errors.New("unsafe production configuration: AES_SECRET must be a non-placeholder 32-byte secret")
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

func parseTrustedProxyList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var proxies []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			proxies = append(proxies, item)
		}
	}
	return proxies
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
