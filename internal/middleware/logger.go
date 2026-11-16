package middleware

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	var mu sync.Mutex
	return func(c *gin.Context) {
		if isGroupBuyRedirectRequest(c) {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		// Log only Gin's route template, never raw URL paths or query strings.
		// Unmatched requests use a placeholder rather than echoing attacker input.
		path := c.FullPath()
		if path == "" {
			path = "-"
		}
		line := fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s %s\n",
			time.Now().Format("2006/01/02 - 15:04:05"),
			c.Writer.Status(),
			time.Since(start),
			c.ClientIP(),
			c.Request.Method,
			path,
		)

		mu.Lock()
		_, _ = gin.DefaultWriter.Write([]byte(line))
		mu.Unlock()
	}
}

func isGroupBuyRedirectRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/api/v1/group-buy/r/")
}
