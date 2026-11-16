package main

import (
	"crypto/subtle"
	"flag"
	"fmt"
	"os"

	"golang.org/x/term"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"go-web-gin-health/internal/adminrecovery"
	"go-web-gin-health/internal/config"
)

func main() {
	username := flag.String("username", "", "现有 super_admin 管理员账号名")
	allowProduction := flag.Bool("allow-production", false, "在生产机本地恢复时显式确认；仍仅允许本机数据库目标")
	flag.Parse()
	if *username == "" {
		fmt.Fprintln(os.Stderr, "必须通过 -username 指定现有超管账号")
		os.Exit(2)
	}

	if err := run(*username, *allowProduction); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(username string, allowProduction bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("加载本地配置失败: %w", err)
	}
	if err := adminrecovery.ValidateLocalTarget(cfg.AppEnv, cfg.MySQLDSN, allowProduction); err != nil {
		return err
	}

	fmt.Fprint(os.Stderr, "新密码（输入不可见）: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return fmt.Errorf("无法安全读取密码；请在交互式终端运行: %w", err)
	}
	defer clear(password)

	fmt.Fprint(os.Stderr, "再次输入新密码: ")
	confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return fmt.Errorf("无法安全读取密码确认: %w", err)
	}
	defer clear(confirmation)
	if subtle.ConstantTimeCompare(password, confirmation) != 1 {
		return fmt.Errorf("两次输入不一致，未修改任何数据")
	}

	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("打开本地数据库失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("打开本地数据库连接失败")
	}
	defer sqlDB.Close()
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("无法连接本地数据库: %w", err)
	}

	if err := adminrecovery.ResetExistingSuperAdminPassword(db, username, string(password)); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "现有超管账号密码已重置；账号角色与状态未变。")
	return nil
}
