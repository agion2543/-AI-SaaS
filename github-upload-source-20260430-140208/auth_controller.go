package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type AuthController struct {
	auth *service.AuthService
}

func NewAuthController(auth *service.AuthService) *AuthController {
	return &AuthController{auth: auth}
}

func (ctl *AuthController) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	user, token, err := ctl.auth.Register(req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"user": sanitizeAuthUser(user), "token": token})
}

func (ctl *AuthController) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	user, token, err := ctl.auth.Login(req)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	utils.Success(c, gin.H{"user": sanitizeAuthUser(user), "token": token})
}

func (ctl *AuthController) LoginByPhone(c *gin.Context) {
	var req dto.PhoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	user, token, err := ctl.auth.LoginByPhone(req)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	utils.Success(c, gin.H{"user": sanitizeAuthUser(user), "token": token})
}

func (ctl *AuthController) SendLoginCode(c *gin.Context) {
	var req dto.SendLoginCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := ctl.auth.SendLoginCode(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"sent": true, "scene": "login"})
}

func (ctl *AuthController) SendRegisterCode(c *gin.Context) {
	var req dto.SendSMSCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := ctl.auth.SendRegisterCode(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"sent": true, "scene": "register"})
}

func (ctl *AuthController) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := ctl.auth.ForgotPassword(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"reset": true})
}

func (ctl *AuthController) SendPasswordResetCode(c *gin.Context) {
	var req dto.SendSMSCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := ctl.auth.SendPasswordResetCode(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"sent": true, "scene": "reset_password"})
}

func (ctl *AuthController) ResetPasswordByPhone(c *gin.Context) {
	var req dto.ResetPasswordByPhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := ctl.auth.ResetPasswordByPhone(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"reset": true})
}

func (ctl *AuthController) DevResetPassword(c *gin.Context) {
	var req dto.DevResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := ctl.auth.DevResetPassword(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"reset": true})
}

func (ctl *AuthController) AdminLogin(c *gin.Context) {
	var req dto.AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	admin, token, err := ctl.auth.AdminLogin(req)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	utils.Success(c, gin.H{"admin": admin, "token": token})
}

func sanitizeAuthUser(user *model.User) gin.H {
	email := user.Email
	if strings.HasSuffix(strings.ToLower(email), "@pending.local") {
		email = ""
	}

	return gin.H{
		"id":                 user.ID,
		"created_at":         user.CreatedAt,
		"updated_at":         user.UpdatedAt,
		"uuid":               user.UUID,
		"email":              email,
		"phone":              user.Phone,
		"merchant_id":        user.MerchantID,
		"role":               user.Role,
		"display_name":       user.DisplayName,
		"status":             user.Status,
		"member_level":       user.MemberLevel,
		"current_package_id": user.CurrentPackageID,
		"expired_at":         user.ExpiredAt,
		"remaining_quota":    user.RemainingQuota,
		"bound_devices":      user.BoundDevices,
		"auto_renew":         user.AutoRenew,
	}
}
