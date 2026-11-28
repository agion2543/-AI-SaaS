package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const mysqlSubscriptionOrderITDSNEnv = "MYSQL_SUBSCRIPTION_ORDER_IT_DSN"

// TestMySQLSubscriptionOrderCreatesValidItemsJSON reproduces the fresh-MySQL
// subscription-order bug: CreateSubscriptionOrder used to leave Order.Items
// unset, so GORM wrote an empty string into the MySQL JSON column
// orders.items and MySQL rejected it with Error 3140 ("Invalid JSON text:
// The document is empty."). SQLite accepts the empty string, which is why
// the existing SQLite tests never caught it. A subscription order has no
// product line items, so the stored value must be the empty JSON array.
func TestMySQLSubscriptionOrderCreatesValidItemsJSON(t *testing.T) {
	db := openMySQLSubscriptionOrderIntegrationDB(t)

	merchant := &model.Merchant{Name: "订阅下单 MySQL 测试商家", Status: "active", SubscriptionStatus: "inactive"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	plan := &model.MerchantPlan{Name: "测试月付", PriceCents: 19900, DurationDays: 30}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create merchant plan: %v", err)
	}
	fallbackPackage := &model.MembershipPackage{Name: "测试占位套餐", Code: "subscription-order-mysql-it", DurationDays: 30, Price: 1000, Status: "published"}
	if err := db.Create(fallbackPackage).Error; err != nil {
		t.Fatalf("create fallback package: %v", err)
	}

	order, err := NewMerchantService(nil, db).CreateSubscriptionOrder(0, merchant.ID, dto.CreateMerchantSubscriptionOrderRequest{PlanID: plan.ID})
	if err != nil {
		t.Fatalf("create subscription order on MySQL: %v", err)
	}

	// The column must really be JSON on this server, otherwise the test
	// would not exercise the constraint that failed in production.
	var dataType string
	if err := db.Raw("SELECT DATA_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'items'").Scan(&dataType).Error; err != nil {
		t.Fatalf("read orders.items column type: %v", err)
	}
	if dataType != "json" {
		t.Fatalf("orders.items column type = %q, want json (test would not reproduce Error 3140)", dataType)
	}

	var storedItems string
	if err := db.Raw("SELECT items FROM orders WHERE id = ?", order.ID).Scan(&storedItems).Error; err != nil {
		t.Fatalf("read stored items: %v", err)
	}
	if storedItems != "[]" {
		t.Fatalf("stored items = %q, want empty JSON array \"[]\"", storedItems)
	}
	var parsed []any
	if err := json.Unmarshal([]byte(storedItems), &parsed); err != nil {
		t.Fatalf("stored items %q is not valid JSON: %v", storedItems, err)
	}
	if len(parsed) != 0 {
		t.Fatalf("stored items parsed to %d entries, want 0", len(parsed))
	}

	var reloaded model.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Items != "[]" {
		t.Fatalf("reloaded order items = %q, want \"[]\"", reloaded.Items)
	}
}

func mysqlSubscriptionOrderTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(mysqlSubscriptionOrderITDSNEnv)
	if dsn == "" {
		t.Skip("set MYSQL_SUBSCRIPTION_ORDER_IT_DSN to a disposable MySQL 8.0.46 Unix-socket database to run integration tests")
	}
	unixStart := strings.Index(dsn, "@unix(")
	if unixStart < 0 {
		t.Fatalf("refusing integration DSN without a local Unix socket: %q", dsn)
	}
	socketStart := unixStart + len("@unix(")
	socketEndRelative := strings.Index(dsn[socketStart:], ")")
	if socketEndRelative < 0 {
		t.Fatalf("malformed Unix-socket DSN")
	}
	socketPath := dsn[socketStart : socketStart+socketEndRelative]
	if !filepath.IsAbs(socketPath) || !strings.HasPrefix(socketPath, "/tmp/cue-subscription-order-mysql-") {
		t.Fatalf("refusing socket outside the dedicated temporary MySQL directory: %q", socketPath)
	}
	dbMarker := strings.LastIndex(dsn, ")/")
	if dbMarker < 0 {
		t.Fatalf("malformed MySQL DSN database component")
	}
	database := strings.SplitN(dsn[dbMarker+2:], "?", 2)[0]
	if !strings.HasPrefix(database, "cue_subscription_order_it_") {
		t.Fatalf("refusing database without the dedicated disposable prefix cue_subscription_order_it_: %q", database)
	}
	return dsn
}

func openMySQLSubscriptionOrderIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysqlDriver.Open(mysqlSubscriptionOrderTestDSN(t)), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open disposable MySQL database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(32)
	t.Cleanup(func() { _ = sqlDB.Close() })

	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.0.46") {
		t.Fatalf("integration tests require MySQL 8.0.46, got %q", version)
	}
	var skipNetworking int
	if err := db.Raw("SELECT @@skip_networking").Scan(&skipNetworking).Error; err != nil {
		t.Fatal(err)
	}
	if skipNetworking != 1 {
		t.Fatalf("integration server must have TCP networking disabled, @@skip_networking=%d", skipNetworking)
	}

	if err := db.AutoMigrate(
		&model.Merchant{},
		&model.MerchantPlan{},
		&model.MembershipPackage{},
		&model.Order{},
	); err != nil {
		t.Fatalf("GORM AutoMigrate synthetic MySQL schema: %v", err)
	}
	return db
}
