package controller

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type UserController struct {
	users *repository.UserRepository
	order *service.OrderService
	auth  *service.AuthService
}

func NewUserController(db *gorm.DB, auth *service.AuthService) *UserController {
	return &UserController{
		users: repository.NewUserRepository(db),
		order: service.NewOrderService(db),
		auth:  auth,
	}
}

func (ctl *UserController) Profile(c *gin.Context) {
	userID := c.GetUint("user_id")
	user, err := ctl.users.FindByID(userID)
	if err != nil {
		utils.Error(c, 404, "user not found")
		return
	}

	usages, _ := ctl.order.UsageRecords(userID)
	utils.Success(c, gin.H{
		"profile":       sanitizeUserForClient(user),
		"usage_records": usages,
	})
}

func (ctl *UserController) BindDevice(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req dto.BindDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	if err := ctl.order.BindDevice(userID, req.DeviceID); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.Success(c, gin.H{"bound": true})
}

func (ctl *UserController) SendBindPhoneCode(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req dto.SendSMSCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	if err := ctl.auth.SendBindPhoneCode(userID, req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.Success(c, gin.H{"sent": true, "scene": "bind_phone"})
}

func (ctl *UserController) BindPhone(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req dto.BindPhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	user, err := ctl.auth.BindPhone(userID, req)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.Success(c, gin.H{"profile": sanitizeUserForClient(user), "bound": true})
}

func (ctl *UserController) SendBindEmailCode(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req dto.SendEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	if err := ctl.auth.SendBindEmailCode(userID, req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.Success(c, gin.H{"sent": true, "scene": "bind_email"})
}

func (ctl *UserController) BindEmail(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req dto.BindEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	user, err := ctl.auth.BindEmail(userID, req)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.Success(c, gin.H{"profile": sanitizeUserForClient(user), "bound": true})
}

func sanitizeUserForClient(user *model.User) gin.H {
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
