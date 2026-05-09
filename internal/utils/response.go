package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-web-gin-health/internal/dto"
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, dto.APIResponse{
		Code:    status,
		Message: message,
	})
}
