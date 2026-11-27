package config_test

import (
	"testing"

	"go-web-gin-health/internal/config"
)

// 初始管理员密码必须走环境变量，不能再有写死默认值。
// 未设置 INITIAL_ADMIN_PASSWORD 时，Load 必须返回空串，
// 由 bootstrap 拒绝创建初始管理员账号并退出（见 internal/bootstrap/app.go）。
func TestInitialAdminPasswordHasNoDefault(t *testing.T) {
	t.Setenv("INITIAL_ADMIN_PASSWORD", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.InitialAdminPass != "" {
		t.Fatalf("InitialAdminPass = %q, want empty when INITIAL_ADMIN_PASSWORD is unset", cfg.InitialAdminPass)
	}
}

func TestInitialAdminPasswordFromEnv(t *testing.T) {
	t.Setenv("INITIAL_ADMIN_PASSWORD", "test-only-strong-password")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.InitialAdminPass != "test-only-strong-password" {
		t.Fatalf("InitialAdminPass = %q, want the env value", cfg.InitialAdminPass)
	}
}
