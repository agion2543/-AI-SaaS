package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/adminaccess"
	"go-web-gin-health/internal/utils"
)

// AdminAuthorization must be placed after Auth(cfg, "admin", db). Every
// privileged route is explicitly mapped by method and Gin route pattern.
func AdminAuthorization() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role == "super_admin" {
			c.Next()
			return
		}
		if role != "admin" {
			utils.Error(c, http.StatusForbidden, "permission denied")
			c.Abort()
			return
		}

		access, ok := adminaccess.ResolveAdminRoute(c.Request.Method, c.FullPath())
		if !ok {
			utils.Error(c, http.StatusForbidden, "permission denied")
			c.Abort()
			return
		}
		if access.SuperAdmin {
			utils.Error(c, http.StatusForbidden, "permission denied")
			c.Abort()
			return
		}
		if access.SessionOnly {
			c.Next()
			return
		}

		permissions, _ := c.Get("admin_permissions")
		stored, _ := permissions.(string)
		for _, required := range access.AllPermissions {
			if !adminaccess.HasPermission(stored, required) {
				utils.Error(c, http.StatusForbidden, "permission denied")
				c.Abort()
				return
			}
		}
		if len(access.AnyPermission) == 0 {
			c.Next()
			return
		}
		for _, required := range access.AnyPermission {
			if adminaccess.HasPermission(stored, required) {
				c.Next()
				return
			}
		}
		utils.Error(c, http.StatusForbidden, "permission denied")
		c.Abort()
	}
}
