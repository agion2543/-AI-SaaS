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

var expiredMerchantReadOnlyRoutes = map[string]struct{}{
	"/leads":                    {},
	"/leads/stats":              {},
	"/payment-config":           {},
	"/stores":                   {},
	"/stores/:storeId/products": {},
	"/orders":                   {},
	"/orders/:id":               {},
	"/refunds":                  {},
	"/settlements":              {},
	"/promotions":               {},
	"/group-buy-links":          {},
}

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
	return merchantSubscriptionMiddlewareAt(db, time.Now)
}

func merchantSubscriptionMiddlewareAt(db *gorm.DB, now func() time.Time) gin.HandlerFunc {
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

		if merchant.Status == "suspended" {
			utils.Error(c, http.StatusForbidden, "商家已被禁用")
			c.Abort()
			return
		}
		if merchant.SubscriptionStatus == "paused" {
			utils.Error(c, http.StatusForbidden, "订阅已被平台暂停")
			c.Abort()
			return
		}
		currentTime := now()
		if merchant.HasActiveSubscriptionAt(currentTime) {
			c.Next()
			return
		}
		if !isExpiredMerchantSubscription(merchant, currentTime) || !isExpiredMerchantReadOnlyRoute(c) {
			utils.Error(c, http.StatusForbidden, "订阅当前不可用")
			c.Abort()
			return
		}

		c.Set("merchant_read_only", true)
		c.Next()
	}
}

func isExpiredMerchantSubscription(merchant model.Merchant, currentTime time.Time) bool {
	if merchant.SubscriptionStatus != "active" {
		return false
	}
	expiresAt := merchant.SubscriptionExpireAt
	if expiresAt == nil {
		expiresAt = merchant.SubscriptionExpiredAt
	}
	return expiresAt != nil && !expiresAt.After(currentTime)
}

func isExpiredMerchantReadOnlyRoute(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}
	route := c.FullPath()
	for _, prefix := range []string{"/api/v1/merchant", "/api/merchant"} {
		if strings.HasPrefix(route, prefix+"/") {
			route = strings.TrimPrefix(route, prefix)
			break
		}
	}
	_, allowed := expiredMerchantReadOnlyRoutes[route]
	return allowed
}
