package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	logger := gin.Logger()
	return func(c *gin.Context) {
		if isGroupBuyRedirectRequest(c) {
			c.Next()
			return
		}
		logger(c)
	}
}

func isGroupBuyRedirectRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/api/v1/group-buy/r/")
}
