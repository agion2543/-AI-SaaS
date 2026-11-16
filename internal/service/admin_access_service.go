package service

import (
	"errors"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/adminaccess"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

var (
	adminUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,60}$`)
	adminRoleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,59}$`)
)

type AdminAccessService struct {
	db *gorm.DB
}

func NewAdminAccessService(db *gorm.DB) *AdminAccessService {
	return &AdminAccessService{db: db}
}

type AdminRoleView struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Permissions []string `json:"permissions"`
}

type AdminAccountView struct {
	ID          uint     `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Status      string   `json:"status"`
	RoleID      uint     `json:"role_id"`
	RoleName    string   `json:"role_name"`
	RoleCode    string   `json:"role_code"`
	Permissions []string `json:"permissions"`
}

type SaveAdminRoleInput struct {
	Name        string
	Code        string
	Permissions []string
}

type CreateAdminAccountInput struct {
	Username    string
	DisplayName string
	Password    string
	RoleID      uint
}

type UpdateAdminAccountInput struct {
	DisplayName string
	Status      string
	RoleID      uint
}

func (s *AdminAccessService) ListRoles() ([]AdminRoleView, error) {
	var roles []model.AdminRole
	if err := s.db.Where("code <> ?", "super_admin").Order("name asc, id asc").Find(&roles).Error; err != nil {
		return nil, err
	}
	result := make([]AdminRoleView, 0, len(roles))
	for _, role := range roles {
		result = append(result, AdminRoleView{ID: role.ID, Name: role.Name, Code: role.Code, Permissions: adminaccess.ParsePermissions(role.Permissions)})
	}
	return result, nil
}

func (s *AdminAccessService) CreateRole(input SaveAdminRoleInput, actor AuditActor) (*AdminRoleView, error) {
	if err := requireAdminAccessSuperAdmin(actor); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	code := strings.TrimSpace(input.Code)
	if name == "" || len([]rune(name)) > 60 || !adminRoleCodePattern.MatchString(code) || isReservedAdminRoleCode(code) {
		return nil, errors.New("角色名称或代码无效")
	}
	permissions, err := adminaccess.NormalizePermissions(input.Permissions)
	if err != nil {
		return nil, err
	}
	role := model.AdminRole{Name: name, Code: code, Permissions: permissions}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			if isUniqueConstraintError(err) {
				return errors.New("角色名称或代码已存在")
			}
			return errors.New("创建角色失败")
		}
		if err := appendAdminAccessAuditTx(tx, actor, "admin_role_create", "admin_role", role.ID, nil, roleAdminAccessSnapshot(role)); err != nil {
			return errors.New("记录角色创建审计失败")
		}
		return nil
	}); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("角色名称或代码已存在")
		}
		return nil, err
	}
	return &AdminRoleView{ID: role.ID, Name: role.Name, Code: role.Code, Permissions: adminaccess.ParsePermissions(role.Permissions)}, nil
}

func (s *AdminAccessService) UpdateRole(id uint, input SaveAdminRoleInput, actor AuditActor) (*AdminRoleView, error) {
	if err := requireAdminAccessSuperAdmin(actor); err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, errors.New("角色不存在")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 60 {
		return nil, errors.New("角色名称无效")
	}
	permissions, err := adminaccess.NormalizePermissions(input.Permissions)
	if err != nil {
		return nil, err
	}
	var role model.AdminRole
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&role, id).Error; err != nil {
			return errors.New("角色不存在")
		}
		if role.Code == "super_admin" {
			return errors.New("不能修改超管角色")
		}
		before := roleAdminAccessSnapshot(role)
		permissionsChanged := role.Permissions != permissions
		role.Name = name
		role.Permissions = permissions
		if err := tx.Model(&role).Select("name", "permissions").Updates(&role).Error; err != nil {
			if isUniqueConstraintError(err) {
				return errors.New("角色名称已存在")
			}
			return errors.New("更新角色失败")
		}
		if permissionsChanged {
			if err := tx.Model(&model.AdminUser{}).Where("role_id = ?", role.ID).UpdateColumn("auth_version", gorm.Expr("auth_version + ?", 1)).Error; err != nil {
				return errors.New("更新管理员凭证版本失败")
			}
		}
		if err := appendAdminAccessAuditTx(tx, actor, "admin_role_update", "admin_role", role.ID, before, roleAdminAccessSnapshot(role)); err != nil {
			return errors.New("记录角色更新审计失败")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &AdminRoleView{ID: role.ID, Name: role.Name, Code: role.Code, Permissions: adminaccess.ParsePermissions(role.Permissions)}, nil
}

func (s *AdminAccessService) ListAdmins() ([]AdminAccountView, error) {
	var admins []model.AdminUser
	if err := s.db.Preload("Role").Joins("JOIN admin_roles ON admin_roles.id = admin_users.role_id").Where("admin_roles.code <> ?", "super_admin").Order("admin_users.id asc").Find(&admins).Error; err != nil {
		return nil, err
	}
	result := make([]AdminAccountView, 0, len(admins))
	for _, admin := range admins {
		result = append(result, adminAccountView(admin))
	}
	return result, nil
}

func (s *AdminAccessService) CreateAdmin(input CreateAdminAccountInput, actor AuditActor) (*AdminAccountView, error) {
	if err := requireAdminAccessSuperAdmin(actor); err != nil {
		return nil, err
	}
	username := strings.TrimSpace(input.Username)
	displayName := strings.TrimSpace(input.DisplayName)
	if !adminUsernamePattern.MatchString(username) {
		return nil, errors.New("账号名需为 3–60 位字母、数字、点、下划线或短横线")
	}
	if displayName == "" || len([]rune(displayName)) > 80 {
		return nil, errors.New("显示名称无效")
	}
	if len([]rune(input.Password)) < 12 {
		return nil, errors.New("初始密码至少需要 12 个字符")
	}
	passwordHash, err := utils.HashPassword(input.Password)
	if err != nil {
		return nil, errors.New("生成管理员密码失败")
	}
	admin := model.AdminUser{
		Username: username, DisplayName: displayName, PasswordHash: passwordHash,
		Status: "active", RoleID: input.RoleID,
	}
	var role model.AdminRole
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&role, input.RoleID).Error; err != nil || role.Code == "super_admin" {
			return errors.New("请选择有效的普通管理员角色")
		}
		admin.RoleID = role.ID
		if err := tx.Create(&admin).Error; err != nil {
			if isUniqueConstraintError(err) {
				return errors.New("管理员账号已存在")
			}
			return errors.New("创建管理员账号失败")
		}
		if err := appendAdminAccessAuditTx(tx, actor, "admin_account_create", "admin_account", admin.ID, nil, adminAdminAccessSnapshot(admin, role)); err != nil {
			return errors.New("记录管理员账号创建审计失败")
		}
		return nil
	}); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("管理员账号已存在")
		}
		return nil, err
	}
	admin.Role = role
	return ptrAdminAccountView(admin), nil
}

func (s *AdminAccessService) UpdateAdmin(id uint, input UpdateAdminAccountInput, actor AuditActor) (*AdminAccountView, error) {
	if err := requireAdminAccessSuperAdmin(actor); err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, errors.New("管理员账号不存在")
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" || len([]rune(displayName)) > 80 {
		return nil, errors.New("显示名称无效")
	}
	if input.Status != "active" && input.Status != "disabled" {
		return nil, errors.New("账号状态无效")
	}
	var updated model.AdminUser
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var admin model.AdminUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Role").First(&admin, id).Error; err != nil {
			return errors.New("管理员账号不存在")
		}
		if admin.Role.Code == "super_admin" {
			return errors.New("不能修改超管账号")
		}
		before := adminAdminAccessSnapshot(admin, admin.Role)
		var role model.AdminRole
		if err := tx.First(&role, input.RoleID).Error; err != nil || role.Code == "super_admin" {
			return errors.New("请选择有效的普通管理员角色")
		}
		if err := tx.Model(&model.AdminUser{}).Where("id = ?", admin.ID).Updates(map[string]interface{}{
			"display_name": displayName,
			"status":       input.Status,
			"role_id":      role.ID,
			"auth_version": gorm.Expr("auth_version + ?", 1),
		}).Error; err != nil {
			return errors.New("更新管理员账号失败")
		}
		admin.DisplayName = displayName
		admin.Status = input.Status
		admin.RoleID = role.ID
		admin.AuthVersion++
		admin.Role = role
		updated = admin
		if err := appendAdminAccessAuditTx(tx, actor, "admin_account_update", "admin_account", admin.ID, before, adminAdminAccessSnapshot(admin, role)); err != nil {
			return errors.New("记录管理员账号更新审计失败")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ptrAdminAccountView(updated), nil
}

func adminAccountView(admin model.AdminUser) AdminAccountView {
	return AdminAccountView{
		ID: admin.ID, Username: admin.Username, DisplayName: admin.DisplayName,
		Status: admin.Status, RoleID: admin.RoleID, RoleName: admin.Role.Name,
		RoleCode: admin.Role.Code, Permissions: adminaccess.ParsePermissions(admin.Role.Permissions),
	}
}

func ptrAdminAccountView(admin model.AdminUser) *AdminAccountView {
	view := adminAccountView(admin)
	return &view
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate entry")
}

func isReservedAdminRoleCode(code string) bool {
	switch code {
	case "admin", "super_admin", "merchant_admin", "customer", "user":
		return true
	default:
		return false
	}
}
