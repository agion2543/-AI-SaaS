package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-web-gin-health/internal/model"
)

// The frozen valid-trade definition: paid_at is set and total_amount > 0.
// These tests pin the exact SQL fragments so a future edit cannot silently
// fork the definition in one of the three call sites.
func TestValidTradeAmountCondition(t *testing.T) {
	if ValidTradeAmountCondition != "paid_at IS NOT NULL AND total_amount > 0" {
		t.Fatalf("ValidTradeAmountCondition=%q", ValidTradeAmountCondition)
	}
	if got := ValidTradeAmountConditionFor(""); got != ValidTradeAmountCondition {
		t.Fatalf("unqualified condition=%q", got)
	}
	if got := ValidTradeAmountConditionFor("orders"); got != "orders.paid_at IS NOT NULL AND orders.total_amount > 0" {
		t.Fatalf("qualified condition=%q", got)
	}
	if got := ValidTradeAmountSumExpression("orders"); got != "COALESCE(SUM(CASE WHEN orders.paid_at IS NOT NULL AND orders.total_amount > 0 THEN orders.total_amount ELSE 0 END),0)" {
		t.Fatalf("sum expression=%q", got)
	}
	if got := ValidTradeAmountAvgExpression("orders"); got != "COALESCE(AVG(CASE WHEN orders.paid_at IS NOT NULL AND orders.total_amount > 0 THEN orders.total_amount ELSE NULL END),0)" {
		t.Fatalf("avg expression=%q", got)
	}
}

func TestOrderChannelHelpers(t *testing.T) {
	if !ValidOrderChannel("store_qr") || !ValidOrderChannel("table") || !ValidOrderChannel("share") {
		t.Fatal("known order channels must be valid")
	}
	if ValidOrderChannel("") || ValidOrderChannel("alipay") || ValidOrderChannel("TABLE") {
		t.Fatal("unknown order channels must be rejected")
	}
	if got := OrderChannelForShareCode(""); got != "store_qr" {
		t.Fatalf("empty share code channel=%q, want store_qr", got)
	}
	if got := OrderChannelForShareCode("  SHARE-1 "); got != "share" {
		t.Fatalf("share code channel=%q, want share", got)
	}
}

func newTradeAmountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQLite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Merchant{}, &model.Store{}, &model.Order{}, &model.PaymentRecord{}, &model.RefundRecord{}, &model.MerchantSettlement{}, &model.User{}, &model.AuditLog{}, &model.MembershipPackage{}); err != nil {
		t.Fatalf("migrate synthetic trade schema: %v", err)
	}
	return db
}

func insertTradeOrder(t *testing.T, db *gorm.DB, orderNo string, merchantID, storeID uint, orderType string, total int64, paidAt *time.Time, channel string) {
	t.Helper()
	order := model.Order{
		OrderNo:     orderNo,
		OrderType:   orderType,
		MerchantID:  &merchantID,
		StoreID:     &storeID,
		TotalAmount: total,
		PaidAt:      paidAt,
		Status:      "received",
		Channel:     channel,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("insert synthetic order %s: %v", orderNo, err)
	}
}

// SummaryWithRange must count exactly the valid trades: paid_at set and
// total_amount > 0. An unpaid order, a zero-amount paid order and a paid
// subscription order must not move customer_trade_revenue, while a paid
// store order in status closed still counts (status is not part of the
// frozen definition).
func TestDashboardSummaryUsesFrozenTradeDefinition(t *testing.T) {
	db := newTradeAmountTestDB(t)
	now := time.Now()
	merchant := model.Merchant{Name: "trade-merchant", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatal(err)
	}
	insertTradeOrder(t, db, "TRADE-PAID-1", merchant.ID, 0, "store_order", 1000, &now, "store_qr")
	closedPaid := now
	insertTradeOrder(t, db, "TRADE-PAID-CLOSED", merchant.ID, 0, "store_order", 500, &closedPaid, "table")
	insertTradeOrder(t, db, "TRADE-UNPAID", merchant.ID, 0, "store_order", 9000, nil, "store_qr")
	insertTradeOrder(t, db, "TRADE-ZERO", merchant.ID, 0, "store_order", 0, &now, "store_qr")
	insertTradeOrder(t, db, "TRADE-SUBSCRIPTION", merchant.ID, 0, "merchant_subscription", 7000, &now, "store_qr")

	summary, err := NewDashboardService(db).SummaryWithRange("2000-01-01", "2999-01-01")
	if err != nil {
		t.Fatalf("SummaryWithRange: %v", err)
	}
	if got := summary["customer_trade_revenue"]; got != int64(1500) {
		t.Fatalf("customer_trade_revenue=%v (%T), want 1500", got, got)
	}
}
