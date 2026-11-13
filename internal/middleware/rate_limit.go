package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/utils"
)

type visitor struct {
	Count     int
	ExpiresAt time.Time
}

func RateLimit(limit int) gin.HandlerFunc {
	var mu sync.Mutex
	visitors := map[string]*visitor{}

	return func(c *gin.Context) {
		key := ""
		if isGroupBuyRedirectRequest(c) {
			// Anonymous per-entry protection: do not retain customer IPs.
			key = "group-buy:" + c.Param("code")
		} else {
			key = c.ClientIP()
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
