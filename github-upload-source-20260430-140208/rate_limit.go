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
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		item, ok := visitors[ip]
		if !ok || item.ExpiresAt.Before(now) {
			item = &visitor{Count: 0, ExpiresAt: now.Add(time.Minute)}
			visitors[ip] = item
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
