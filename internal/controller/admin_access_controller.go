package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/adminaccess"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type AdminAccessController struct {
	access *service.AdminAccessService
}

func NewAdminAccessController(access *service.AdminAccessService) *AdminAccessController {
	return &AdminAccessController{access: access}
}

func (ctl *AdminAccessController) Permissions(c *gin.Context) {
	utils.Success(c, gin.H{"list": adminaccess.PermissionCatalog()})
}

func (ctl *AdminAccessController) Roles(c *gin.Context) {
	roles, err := ctl.access.ListRoles()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "读取管理员角色失败")
		return
	}
	utils.Success(c, gin.H{"list": roles})
}

func (ctl *AdminAccessController) CreateRole(c *gin.Context) {
	var req struct {
		Name        string   `json:"name"`
		Code        string   `json:"code"`
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "角色信息格式无效")
		return
	}
	role, err := ctl.access.CreateRole(service.SaveAdminRoleInput{Name: req.Name, Code: req.Code, Permissions: req.Permissions}, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"role": role})
}

func (ctl *AdminAccessController) UpdateRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		utils.Error(c, http.StatusBadRequest, "角色 ID 无效")
		return
	}
	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "角色信息格式无效")
		return
	}
	role, err := ctl.access.UpdateRole(uint(id), service.SaveAdminRoleInput{Name: req.Name, Permissions: req.Permissions}, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"role": role})
}

func (ctl *AdminAccessController) Admins(c *gin.Context) {
	admins, err := ctl.access.ListAdmins()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "读取管理员账号失败")
		return
	}
	utils.Success(c, gin.H{"list": admins})
}

func (ctl *AdminAccessController) CreateAdmin(c *gin.Context) {
	var req struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		RoleID      uint   `json:"role_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "管理员账号信息格式无效")
		return
	}
	admin, err := ctl.access.CreateAdmin(service.CreateAdminAccountInput{
		Username: req.Username, DisplayName: req.DisplayName, Password: req.Password, RoleID: req.RoleID,
	}, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"admin": admin})
}

func (ctl *AdminAccessController) UpdateAdmin(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		utils.Error(c, http.StatusBadRequest, "管理员账号 ID 无效")
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
		Status      string `json:"status"`
		RoleID      uint   `json:"role_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "管理员账号信息格式无效")
		return
	}
	admin, err := ctl.access.UpdateAdmin(uint(id), service.UpdateAdminAccountInput{
		DisplayName: req.DisplayName, Status: req.Status, RoleID: req.RoleID,
	}, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"admin": admin})
}
