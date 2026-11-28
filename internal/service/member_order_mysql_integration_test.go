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

const mysqlMemberOrderITDSNEnv = "MYSQL_MEMBER_ORDER_IT_DSN"

// TestMySQLMemberOrderCreatesValidItemsJSON reproduces the fresh-MySQL
// member-order bug, the same root cause as the subscription-order bug fixed
// in 1d62e5b: CreateOrder used to leave Order.Items unset, so GORM wrote an
// empty string into the MySQL JSON column orders.items and MySQL rejected
// it with Error 3140 ("Invalid JSON text: The document is empty.").
// SQLite accepts the empty string, which is why the existing SQLite tests
// never caught it. A member (user_membership) order has no product line
// items, so the stored value must be the empty JSON array.
func TestMySQLMemberOrderCreatesValidItemsJSON(t *testing.T) {
	db := openMySQLMemberOrderIntegrationDB(t)

	user := &model.User{Email: "member-order-mysql-it@example.test", Phone: "13800000001", Status: "active"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	pkg := &model.MembershipPackage{Name: "测试会员套餐", Code: "member-order-mysql-it", DurationDays: 30, Price: 1000, Status: "published"}
	if err := db.Create(pkg).Error; err != nil {
		t.Fatalf("create membership package: %v", err)
	}

	order, err := NewOrderService(db).CreateOrder(user.ID, dto.CreateOrderRequest{PackageID: pkg.ID, PaymentChannel: "alipay"})
	if err != nil {
		t.Fatalf("create member order on MySQL: %v", err)
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

func mysqlMemberOrderTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(mysqlMemberOrderITDSNEnv)
	if dsn == "" {
		t.Skip("set MYSQL_MEMBER_ORDER_IT_DSN to a disposable MySQL 8.0.46 Unix-socket database to run integration tests")
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
	if !filepath.IsAbs(socketPath) || !strings.HasPrefix(socketPath, "/tmp/cue-member-order-mysql-") {
		t.Fatalf("refusing socket outside the dedicated temporary MySQL directory: %q", socketPath)
	}
	dbMarker := strings.LastIndex(dsn, ")/")
	if dbMarker < 0 {
		t.Fatalf("malformed MySQL DSN database component")
	}
	database := strings.SplitN(dsn[dbMarker+2:], "?", 2)[0]
	if !strings.HasPrefix(database, "cue_member_order_it_") {
		t.Fatalf("refusing database without the dedicated disposable prefix cue_member_order_it_: %q", database)
	}
	return dsn
}

func openMySQLMemberOrderIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysqlDriver.Open(mysqlMemberOrderTestDSN(t)), &gorm.Config{
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
		&model.User{},
		&model.MembershipPackage{},
		&model.Order{},
	); err != nil {
		t.Fatalf("GORM AutoMigrate synthetic MySQL schema: %v", err)
	}
	return db
}
