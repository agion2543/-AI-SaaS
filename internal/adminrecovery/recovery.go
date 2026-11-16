package adminrecovery

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func ValidateLocalTarget(appEnv, mysqlDSN string, allowProduction bool) error {
	if strings.EqualFold(strings.TrimSpace(appEnv), "production") && !allowProduction {
		return errors.New("生产环境恢复需要显式传入 -allow-production")
	}

	if strings.TrimSpace(mysqlDSN) == "" {
		return errors.New("恢复命令只允许连接本机环回地址或 Unix socket 数据库")
	}
	cfg, err := mysqlDriver.ParseDSN(mysqlDSN)
	if err != nil {
		return errors.New("恢复命令只允许连接本机环回地址或 Unix socket 数据库")
	}
	switch cfg.Net {
	case "tcp", "tcp4", "tcp6":
		host, port, err := net.SplitHostPort(cfg.Addr)
		if err != nil {
			return errors.New("恢复命令只允许连接本机环回地址或 Unix socket 数据库")
		}
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return errors.New("恢复命令只允许连接本机环回地址或 Unix socket 数据库")
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() || (cfg.Net == "tcp4" && ip.To4() == nil) || (cfg.Net == "tcp6" && ip.To4() != nil) {
			return errors.New("恢复命令只允许连接本机环回地址或 Unix socket 数据库")
		}
		return nil
	case "unix":
		if cfg.Addr != "" {
			return nil
		}
	}
	return errors.New("恢复命令只允许连接本机环回地址或 Unix socket 数据库")
}

// ResetExistingSuperAdminPassword changes only the password hash and auth version
// on an existing account whose current role remains super_admin. It appends a
// minimal success audit in the same transaction, so a failed audit leaves the
// account unchanged and every previous admin token is invalid after commit. It
// never creates or promotes an account or changes its role or status.
func ResetExistingSuperAdminPassword(db *gorm.DB, username, newPassword string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("必须提供已有超管账号名")
	}
	if len([]rune(newPassword)) < 12 {
		return errors.New("新密码至少需要 12 个字符")
	}
	if db == nil {
		return errors.New("本地数据库不可用")
	}
	passwordHash, err := utils.HashPassword(newPassword)
	if err != nil {
		return errors.New("无法安全处理新密码")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var admin model.AdminUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Role").Where("username = ?", username).First(&admin).Error; err != nil {
			return errors.New("没有找到该现有管理员账号")
		}
		if admin.Role.Code != "super_admin" {
			return errors.New("该账号当前不是 super_admin，未修改任何数据")
		}
		result := tx.Model(&model.AdminUser{}).Where("id = ? AND role_id = ?", admin.ID, admin.RoleID).Updates(map[string]interface{}{
			"password_hash": passwordHash,
			"auth_version":  gorm.Expr("auth_version + ?", 1),
		})
		if result.Error != nil {
			return errors.New("保存超管密码失败")
		}
		if result.RowsAffected != 1 {
			return errors.New("超管账号状态已变化，未修改密码")
		}
		auditAt := time.Now().UTC()
		if err := tx.Table("audit_logs").Create(map[string]interface{}{
			"created_at":  auditAt,
			"updated_at":  auditAt,
			"actor_id":    0,
			"actor_type":  "local_recovery",
			"action":      "admin_password_recovery",
			"target_type": "admin_user",
			"target_id":   admin.ID,
			"detail":      `{"result":"success"}`,
		}).Error; err != nil {
			return errors.New("记录超管恢复审计失败，恢复未完成")
		}
		return nil
	})
}
