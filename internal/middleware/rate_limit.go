package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/utils"
)

type visitor struct {
	Count     int
	ExpiresAt time.Time
}

type rateLimitKey func(*gin.Context) (string, bool)

func RateLimit(limit int) gin.HandlerFunc {
	return rateLimit(limit, func(c *gin.Context) (string, bool) {
		if isGroupBuyRedirectRequest(c) {
			// Anonymous per-entry protection: do not retain customer IPs.
			return "group-buy:" + c.Param("code"), true
		}
		if c.Request.Method == http.MethodGet && (c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready") {
			// Keep probes bounded, but do not let business traffic exhaust their bucket.
			return "probe:" + c.Request.URL.Path + ":" + c.ClientIP(), true
		}
		return c.ClientIP(), true
	})
}

// OTPSendRateLimit shares a per-IP bucket across all POST routes ending in
// /send-code. It is intentionally process-local, like RateLimit.
func OTPSendRateLimit(limit int) gin.HandlerFunc {
	return rateLimit(limit, func(c *gin.Context) (string, bool) {
		if c.Request.Method != http.MethodPost || !strings.HasSuffix(c.Request.URL.Path, "/send-code") {
			return "", false
		}
		return c.ClientIP(), true
	})
}

func rateLimit(limit int, keyFor rateLimitKey) gin.HandlerFunc {
	var mu sync.Mutex
	visitors := map[string]*visitor{}

	return func(c *gin.Context) {
		key, limited := keyFor(c)
		if !limited {
			c.Next()
			return
		}
		now := time.Now()

		mu.Lock()
		if len(visitors) >= 10000 {
			for oldKey, oldItem := range visitors {
				if oldItem.ExpiresAt.Before(now) {
					delete(visitors, oldKey)
				}
			}
		}
		item, ok := visitors[key]
		if !ok || item.ExpiresAt.Before(now) {
			if !ok && len(visitors) >= 10000 {
				mu.Unlock()
				utils.Error(c, http.StatusTooManyRequests, "too many requests")
				c.Abort()
				return
			}
			item = &visitor{Count: 0, ExpiresAt: now.Add(time.Minute)}
			visitors[key] = item
		}
		item.Count++
		allowed := item.Count <= limit
		mu.Unlock()

		if !allowed {
			utils.Error(c, http.StatusTooManyRequests, "too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}
