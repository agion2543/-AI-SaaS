package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func Auth(cfg *config.Config, requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			utils.Error(c, http.StatusUnauthorized, "missing token")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := utils.ParseToken(cfg.JWTSecret, tokenString)
		if err != nil {
			utils.Error(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}

		if requiredRole == "user" && claims.Role == "super_admin" {
			utils.Error(c, http.StatusForbidden, "permission denied")
			c.Abort()
			return
		}
		if requiredRole == "merchant" && claims.Role != "merchant_admin" {
			utils.Error(c, http.StatusForbidden, "permission denied")
			c.Abort()
			return
		}
		if requiredRole == "admin" && claims.Role != "super_admin" && claims.Role != "admin" {
			utils.Error(c, http.StatusForbidden, "permission denied")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("merchant_id", claims.MerchantID)
		c.Set("identity", claims.Identity)
		c.Next()
	}
}

func MerchantSubscriptionMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "merchant_admin" {
			c.Next()
			return
		}

		value, exists := c.Get("merchant_id")
		if !exists || value == nil {
			utils.Error(c, http.StatusForbidden, "merchant context missing")
			c.Abort()
			return
		}

		merchantID, ok := value.(*uint)
		if !ok || merchantID == nil {
			utils.Error(c, http.StatusForbidden, "merchant context missing")
			c.Abort()
			return
		}

		var merchant model.Merchant
		if err := db.First(&merchant, *merchantID).Error; err != nil {
			utils.Error(c, http.StatusForbidden, "merchant not found")
			c.Abort()
			return
		}

		expireAt := merchant.SubscriptionExpireAt
		if expireAt == nil {
			expireAt = merchant.SubscriptionExpiredAt
		}
		if expireAt == nil || expireAt.Before(time.Now()) {
			utils.Error(c, http.StatusForbidden, "订阅已过期，请续费")
			c.Abort()
			return
		}

		c.Next()
	}
}
