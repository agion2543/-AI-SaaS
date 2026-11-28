package main

// One-click demo-seed entry: generates the demo merchant dataset
// (demo cafe: products, orders + detail rows, coupons, share campaign,
// leads, promotion, campaign target + postmortem, manual Douyin
// redemption, suggestion executions, rebuilt customer profiles).
//
// Safety: refuses to run unless APP_ENV is explicitly "development"
// (config.DevPasswordResetEnabled, the repo's single explicit-development
// signal). Never run this against a production database. The seed itself
// is idempotent: re-running it creates no duplicate orders, detail rows
// or profile rows.
//
// Usage (after cmd/schema-migrate has built the schema):
//   APP_ENV=development MYSQL_DSN='...' go run ./cmd/seed-demo
import (
	"fmt"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"go-web-gin-health/internal/bootstrap"
	"go-web-gin-health/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed-demo:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.DevPasswordResetEnabled {
		return fmt.Errorf("refuses to run: APP_ENV must be explicitly \"development\" (demo data must never be seeded into a non-development database)")
	}
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open MySQL database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := bootstrap.SeedDemoMerchantDataset(db); err != nil {
		return err
	}
	fmt.Println("demo seed complete: demo cafe (15 products, 16 orders, 6 leads, 4 coupons, 1 share campaign, 1 target + postmortem). All rows are labeled demo data, not real business results.")
	return nil
}
