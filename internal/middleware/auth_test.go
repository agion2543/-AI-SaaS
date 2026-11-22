package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func TestUserAuthRequiresCustomerRoleAndActiveCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user test schema: %v", err)
	}

	user := model.User{UUID: uuid.NewString(), Email: "auth-boundary@example.invalid", Phone: "13800138000", Role: "customer", Status: "active"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create synthetic user: %v", err)
	}
	cfg := &config.Config{JWTSecret: "test-only-secret"}
	engine := gin.New()
	engine.GET("/private", Auth(cfg, "user", db), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tokenForRole := func(role string) string {
		t.Helper()
		candidate := user
		candidate.Role = role
		token, err := utils.GenerateUserToken(cfg.JWTSecret, candidate, 1)
		if err != nil {
			t.Fatalf("create synthetic %s token: %v", role, err)
		}
		return token
	}
	request := func(token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}

	customerToken := tokenForRole("customer")
	if response := request(customerToken); response.Code != http.StatusNoContent {
		t.Fatalf("active customer token status=%d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
	}
	for _, role := range []string{"user", "merchant_admin", "admin", "super_admin"} {
		if response := request(tokenForRole(role)); response.Code != http.StatusForbidden {
			t.Errorf("%s token status=%d, want %d; body=%s", role, response.Code, http.StatusForbidden, response.Body.String())
		}
	}
	legacyToken, err := utils.GenerateToken(cfg.JWTSecret, user.ID, user.Phone, user.Role, nil, 1)
	if err != nil {
		t.Fatalf("create synthetic legacy token: %v", err)
	}
	if response := request(legacyToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("legacy token without credential version status=%d, want %d; body=%s", response.Code, http.StatusUnauthorized, response.Body.String())
	}

	for _, status := range []string{"disabled", "banned"} {
		t.Run("existing token rejected after "+status, func(t *testing.T) {
			if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("status", status).Error; err != nil {
				t.Fatalf("set synthetic user status to %s: %v", status, err)
			}
			if response := request(customerToken); response.Code != http.StatusForbidden {
				t.Fatalf("existing token for %s user status=%d, want %d; body=%s", status, response.Code, http.StatusForbidden, response.Body.String())
			}
		})
		if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("status", "active").Error; err != nil {
			t.Fatalf("restore synthetic user status: %v", err)
		}
	}

	t.Run("existing token rejected after password change", func(t *testing.T) {
		if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("password_hash", "synthetic-changed-password-hash").Error; err != nil {
			t.Fatalf("change synthetic password hash: %v", err)
		}
		if response := request(customerToken); response.Code != http.StatusUnauthorized {
			t.Fatalf("token after password change status=%d, want %d; body=%s", response.Code, http.StatusUnauthorized, response.Body.String())
		}
		if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("password_hash", user.PasswordHash).Error; err != nil {
			t.Fatalf("restore synthetic password hash: %v", err)
		}
	})
	t.Run("existing token rejected after role change", func(t *testing.T) {
		if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("role", "merchant_admin").Error; err != nil {
			t.Fatalf("change synthetic role: %v", err)
		}
		if response := request(customerToken); response.Code != http.StatusUnauthorized {
			t.Fatalf("token after role change status=%d, want %d; body=%s", response.Code, http.StatusUnauthorized, response.Body.String())
		}
		if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("role", "customer").Error; err != nil {
			t.Fatalf("restore synthetic role: %v", err)
		}
	})
	t.Run("existing token rejected after merchant reassignment", func(t *testing.T) {
		if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("merchant_id", uint(9876)).Error; err != nil {
			t.Fatalf("change synthetic merchant assignment: %v", err)
		}
		if response := request(customerToken); response.Code != http.StatusUnauthorized {
			t.Fatalf("token after merchant reassignment status=%d, want %d; body=%s", response.Code, http.StatusUnauthorized, response.Body.String())
		}
		if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("merchant_id", nil).Error; err != nil {
			t.Fatalf("restore synthetic merchant assignment: %v", err)
		}
	})
}
