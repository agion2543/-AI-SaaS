package bootstrap

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/router"
	"go-web-gin-health/internal/utils"
)

type Application struct {
	Config *config.Config
	DB     *gorm.DB
	Engine *gin.Engine
}

func NewApplication() (*Application, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := autoMigrate(db); err != nil {
		return nil, err
	}
	if err := seedInitialData(db); err != nil {
		return nil, err
	}

	engine := gin.New()
	app := &Application{
		Config: cfg,
		DB:     db,
		Engine: engine,
	}

	router.Register(engine, db, cfg)
	return app, nil
}

func (a *Application) Run() error {
	return a.Engine.Run(fmt.Sprintf("%s:%s", a.Config.AppHost, a.Config.AppPort))
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Merchant{},
		&model.MerchantPlan{},
		&model.MerchantPaymentConfig{},
		&model.MerchantSettlement{},
		&model.AuditLog{},
		&model.Store{},
		&model.StoreProduct{},
		&model.CustomerLead{},
		&model.Promotion{},
		&model.User{},
		&model.VerificationCode{},
		&model.EmailVerificationCode{},
		&model.AdminRole{},
		&model.AdminUser{},
		&model.MembershipPackage{},
		&model.Order{},
		&model.PaymentRecord{},
		&model.RefundRecord{},
		&model.CardCode{},
		&model.SystemConfig{},
		&model.UsageRecord{},
	)
}

func seedInitialData(db *gorm.DB) error {
	role := model.AdminRole{
		Name:        "Super Admin",
		Code:        "super_admin",
		Permissions: "dashboard,users,packages,orders,payments,cards,configs",
	}
	if err := db.Where(model.AdminRole{Code: role.Code}).Assign(role).FirstOrCreate(&role).Error; err != nil {
		return err
	}

	passwordHash, err := utils.HashPassword("Admin@123456")
	if err != nil {
		return err
	}

	admin := model.AdminUser{
		Username:     "admin",
		PasswordHash: passwordHash,
		DisplayName:  "System Admin",
		Status:       "active",
		RoleID:       role.ID,
	}
	if err := db.Where(model.AdminUser{Username: admin.Username}).Assign(admin).FirstOrCreate(&admin).Error; err != nil {
		return err
	}

	packages := []model.MembershipPackage{
		{Name: "日卡", Code: "day", DurationDays: 1, Price: 990, OriginalPrice: 1290, Quota: 10, Sort: 1, Status: "published", Description: "1 天会员权益"},
		{Name: "周卡", Code: "week", DurationDays: 7, Price: 4990, OriginalPrice: 6990, Quota: 100, Sort: 2, Status: "published", Description: "7 天会员权益"},
		{Name: "月卡", Code: "month", DurationDays: 30, Price: 12900, OriginalPrice: 15900, Quota: 500, Sort: 3, Status: "published", Description: "30 天会员权益"},
		{Name: "年卡", Code: "year", DurationDays: 365, Price: 99900, OriginalPrice: 129900, Quota: 6000, Sort: 4, Status: "published", Description: "365 天会员权益"},
		{Name: "终身套餐", Code: "lifetime", DurationDays: 0, Price: 299900, OriginalPrice: 399900, Quota: 999999, Sort: 5, Status: "published", IsLifetime: true, Description: "永久会员权益"},
	}

	for _, pkg := range packages {
		item := pkg
		if err := db.Where(model.MembershipPackage{Code: item.Code}).Assign(item).FirstOrCreate(&item).Error; err != nil {
			return err
		}
	}

	merchantPlans := []model.MerchantPlan{
		{Name: "月付", PriceCents: 19900, DurationDays: 30, Sort: 1},
		{Name: "年付", PriceCents: 199900, DurationDays: 365, Sort: 2},
	}
	for _, plan := range merchantPlans {
		item := plan
		if err := db.Where(model.MerchantPlan{Name: item.Name}).Assign(item).FirstOrCreate(&item).Error; err != nil {
			return err
		}
	}

	if err := db.Model(&model.Merchant{}).Where("status = ?", "pending").Update("status", "active").Error; err != nil {
		return err
	}

	return nil
}
