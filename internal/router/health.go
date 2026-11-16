package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-web-gin-health/internal/utils"
)

const readinessPingTimeout = 2 * time.Second

func registerHealthRoutes(engine *gin.Engine, db *gorm.DB) {
	// Liveness only answers whether the process can serve HTTP. It deliberately
	// does not depend on external services such as the database.
	engine.GET("/health", func(c *gin.Context) {
		utils.Success(c, gin.H{"status": "ok"})
	})

	// Readiness verifies the active database connection but never returns
	// driver errors, DSNs, hostnames, or credentials to the caller.
	engine.GET("/ready", func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    http.StatusServiceUnavailable,
				"message": "not ready",
				"data": gin.H{
					"status": "not_ready",
					"checks": gin.H{"database": "unavailable"},
				},
			})
			return
		}

		sqlDB, err := db.DB()
		if err == nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), readinessPingTimeout)
			err = sqlDB.PingContext(ctx)
			cancel()
		}
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    http.StatusServiceUnavailable,
				"message": "not ready",
				"data": gin.H{
					"status": "not_ready",
					"checks": gin.H{"database": "unavailable"},
				},
			})
			return
		}

		utils.Success(c, gin.H{
			"status": "ready",
			"checks": gin.H{"database": "ready"},
		})
	})
}
