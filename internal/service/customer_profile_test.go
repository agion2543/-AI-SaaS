package service

import (
	"sort"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

// Customer-profile (v16) unit tests. All fixtures are the shared SQLite
// in-memory order fixture (newOrderBoundaryTestDB, which now also migrates
// customer_profiles); nothing here touches MySQL — the GROUP_CONCAT backfill
// equivalence is covered by the v16 MySQL integration test instead.

func newCustomerProfileServices(t *testing.T) (*OrderService, *AdminService, *MerchantService, *gorm.DB) {
	t.Helper()
	db := newOrderBoundaryTestDB(t)
	return NewOrderService(db), NewAdminService(&config.Config{}, db), NewMerchantService(&config.Config{}, db), db
}

func payProfileOrder(t *testing.T, svc *OrderService, order model.Order, txn string) *model.Order {
	t.Helper()
	paid, err := svc.HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo:        order.OrderNo,
		TransactionNo:  txn,
		PaymentChannel: "alipay",
		Amount:         order.TotalAmount,
		Status:         "success",
	})
	if err != nil {
		t.Fatalf("pay order %s: %v", order.OrderNo, err)
	}
	return paid
}

func setProfileOrderFacts(t *testing.T, db *gorm.DB, orderID uint, phone string, createdAt time.Time, discount int64) {
	t.Helper()
	if err := db.Exec("UPDATE orders SET customer_phone = ?, created_at = ?, discount_amount = ? WHERE id = ?", phone, createdAt, discount, orderID).Error; err != nil {
		t.Fatalf("set order %d facts: %v", orderID, err)
	}
}

func readProfile(t *testing.T, db *gorm.DB, merchantID uint, phone string) model.CustomerProfile {
	t.Helper()
	var profile model.CustomerProfile
	if err := db.Where("merchant_id = ? AND customer_phone = ?", merchantID, phone).First(&profile).Error; err != nil {
		t.Fatalf("read profile merchant=%d phone=%s: %v", merchantID, phone, err)
	}
	return profile
}

func countProfiles(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&model.CustomerProfile{}).Count(&count).Error; err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	return count
}

func TestCustomerProfileFirstPaymentCreatesRow(t *testing.T) {
	orderSvc, adminSvc, _, db := newCustomerProfileServices(t)
	merchant, store, _, order := createSyntheticStoreOrder(t, db, "CP-FIRST-001", "pending", 1800, nil)
	// Within the 30-minute checkout window: an older created_at would take
	// the late-payment exception path (status closed) instead of "received".
	createdAt := time.Now().Add(-10 * time.Minute)
	setProfileOrderFacts(t, db, order.ID, "13800000001", createdAt, 0)

	paid := payProfileOrder(t, orderSvc, order, "cp-first-txn-001")
	if paid.Status != "received" || paid.PaidAt == nil {
		t.Fatalf("paid order must be received with PaidAt, got status=%q paid_at=%v", paid.Status, paid.PaidAt)
	}
	profile := readProfile(t, db, merchant.ID, "13800000001")
	if profile.OrderCount != 1 || profile.TotalAmountCents != 1800 || profile.DiscountAmountCents != 0 {
		t.Fatalf("profile counts=%d total=%d discount=%d, want 1/1800/0", profile.OrderCount, profile.TotalAmountCents, profile.DiscountAmountCents)
	}
	if profile.FirstPaidAt == nil || profile.LastPaidAt == nil || !profile.FirstPaidAt.Equal(*profile.LastPaidAt) {
		t.Fatalf("first/last paid must be equal after one payment, got %v/%v", profile.FirstPaidAt, profile.LastPaidAt)
	}
	if profile.LastOrderAt == nil || profile.LastOrderAt.UnixMilli() != createdAt.UnixMilli() {
		t.Fatalf("last_order_at=%v, want order created_at %v", profile.LastOrderAt, createdAt)
	}
	if profile.LastOrderID != order.ID || profile.LastStatus != "received" {
		t.Fatalf("last order=%d status=%q, want %d/received", profile.LastOrderID, profile.LastStatus, order.ID)
	}
	if profile.LastStoreID == nil || *profile.LastStoreID != store.ID || profile.LastStoreName != store.Name {
		t.Fatalf("last store=%v/%q, want %d/%q", profile.LastStoreID, profile.LastStoreName, store.ID, store.Name)
	}

	rows, total, err := adminSvc.ListCustomerProfiles(1, 10, CustomerProfileFilter{})
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].AITag != "新顾客" {
		t.Fatalf("list total=%d rows=%v, want 1 新顾客 row", total, rows)
	}
	top, err := adminSvc.ListMerchantCustomerProfiles(merchant.ID, 10)
	if err != nil {
		t.Fatalf("list merchant profiles: %v", err)
	}
	if len(top) != 1 || top[0].CustomerPhone != "138****0001" || top[0].TotalAmount != 1800 {
		t.Fatalf("merchant top=%v, want 1 masked row total 1800", top)
	}
}

func TestCustomerProfileNonEventsDoNotCreateRow(t *testing.T) {
	orderSvc, _, _, db := newCustomerProfileServices(t)

	// Unpaid order: no callback, no profile.
	_, _, _, unpaid := createSyntheticStoreOrder(t, db, "CP-NONE-UNPAID", "pending", 1800, nil)
	setProfileOrderFacts(t, db, unpaid.ID, "13800000002", time.Now().Add(-time.Hour), 0)
	if countProfiles(t, db) != 0 {
		t.Fatal("unpaid order must not create a profile")
	}

	// Empty phone: payment succeeds for the order, but no profile row.
	_, _, _, emptyPhone := createSyntheticStoreOrder(t, db, "CP-NONE-EMPTY", "pending", 1800, nil)
	setProfileOrderFacts(t, db, emptyPhone.ID, "", time.Now().Add(-time.Hour), 0)
	payProfileOrder(t, orderSvc, emptyPhone, "cp-none-empty-txn")
	if countProfiles(t, db) != 0 {
		t.Fatal("empty-phone order must not create a profile")
	}

	// Zero-amount order: the callback itself refuses (amount must be > 0).
	_, _, _, zero := createSyntheticStoreOrder(t, db, "CP-NONE-ZERO", "pending", 0, nil)
	setProfileOrderFacts(t, db, zero.ID, "13800000003", time.Now().Add(-time.Hour), 0)
	if _, err := orderSvc.HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo: zero.OrderNo, TransactionNo: "cp-none-zero-txn",
		PaymentChannel: "alipay", Amount: 0, Status: "success",
	}); err == nil {
		t.Fatal("zero-amount payment callback must fail")
	}
	if countProfiles(t, db) != 0 {
		t.Fatal("zero-amount order must not create a profile")
	}

	// Non-store orders, unpaid orders and zero amounts never reach the
	// upsert, even when the helper is called directly with a paid order.
	now := time.Now()
	merchantID := uint(1)
	paidAt := now
	for _, tc := range []struct {
		name  string
		order model.Order
	}{
		{"subscription", model.Order{OrderType: "merchant_subscription", MerchantID: &merchantID, CustomerPhone: "13800000004", TotalAmount: 19900, Status: "paid", PaidAt: &paidAt}},
		{"unpaid store order", model.Order{OrderType: "store_order", MerchantID: &merchantID, CustomerPhone: "13800000005", TotalAmount: 1000, Status: "pending"}},
		{"zero store order", model.Order{OrderType: "store_order", MerchantID: &merchantID, CustomerPhone: "13800000006", TotalAmount: 0, Status: "received", PaidAt: &paidAt}},
		{"empty phone paid", model.Order{OrderType: "store_order", MerchantID: &merchantID, CustomerPhone: " ", TotalAmount: 1000, Status: "received", PaidAt: &paidAt}},
	} {
		if err := upsertCustomerProfileForPaidOrderTx(db, tc.order); err != nil {
			t.Fatalf("%s upsert: %v", tc.name, err)
		}
	}
	if countProfiles(t, db) != 0 {
		t.Fatal("non-event orders must not create profiles")
	}
}

func TestCustomerProfileSecondPaymentAccumulatesAndIsolates(t *testing.T) {
	orderSvc, _, _, db := newCustomerProfileServices(t)
	merchant, _, _, first := createSyntheticStoreOrder(t, db, "CP-ACC-001", "pending", 1800, nil)
	// Both orders stay inside the 30-minute checkout window so the normal
	// payment path (status received) is exercised.
	base := time.Now().Add(-20 * time.Minute)
	setProfileOrderFacts(t, db, first.ID, "13800000011", base, 100)
	payProfileOrder(t, orderSvc, first, "cp-acc-txn-001")

	_, _, _, second := createSyntheticStoreOrder(t, db, "CP-ACC-002", "pending", 8200, nil)
	// Same merchant/store as the first order (createSyntheticStoreOrder
	// makes a new merchant per call, so re-point the order at merchant 1).
	if err := db.Exec("UPDATE orders SET merchant_id = ?, store_id = ? WHERE id = ?", merchant.ID, first.StoreID, second.ID).Error; err != nil {
		t.Fatal(err)
	}
	second.StoreID = first.StoreID
	setProfileOrderFacts(t, db, second.ID, "13800000011", base.Add(10*time.Minute), 300)
	payProfileOrder(t, orderSvc, second, "cp-acc-txn-002")

	profile := readProfile(t, db, merchant.ID, "13800000011")
	if profile.OrderCount != 2 || profile.TotalAmountCents != 10000 || profile.DiscountAmountCents != 400 {
		t.Fatalf("accumulated count=%d total=%d discount=%d, want 2/10000/400", profile.OrderCount, profile.TotalAmountCents, profile.DiscountAmountCents)
	}
	if profile.LastOrderID != second.ID || profile.LastStatus != "received" {
		t.Fatalf("last order=%d status=%q, want %d/received", profile.LastOrderID, profile.LastStatus, second.ID)
	}
	if profile.LastOrderAt == nil || profile.LastOrderAt.UnixMilli() != base.Add(10*time.Minute).UnixMilli() {
		t.Fatalf("last_order_at=%v, want second order created_at", profile.LastOrderAt)
	}

	// Same phone, different merchant: a separate profile, first untouched.
	merchant2, _, _, other := createSyntheticStoreOrder(t, db, "CP-ACC-003", "pending", 5000, nil)
	setProfileOrderFacts(t, db, other.ID, "13800000011", base.Add(time.Hour), 0)
	payProfileOrder(t, orderSvc, other, "cp-acc-txn-003")
	if got := readProfile(t, db, merchant2.ID, "13800000011"); got.OrderCount != 1 || got.TotalAmountCents != 5000 {
		t.Fatalf("other-merchant profile=%+v, want count 1 total 5000", got)
	}
	if got := readProfile(t, db, merchant.ID, "13800000011"); got.OrderCount != 2 || got.TotalAmountCents != 10000 {
		t.Fatalf("first-merchant profile changed after other-merchant payment: %+v", got)
	}

	// Different phone, same merchant: a separate profile.
	_, _, _, third := createSyntheticStoreOrder(t, db, "CP-ACC-004", "pending", 700, nil)
	if err := db.Exec("UPDATE orders SET merchant_id = ?, store_id = ? WHERE id = ?", merchant.ID, first.StoreID, third.ID).Error; err != nil {
		t.Fatal(err)
	}
	setProfileOrderFacts(t, db, third.ID, "13800000012", base.Add(time.Hour), 0)
	payProfileOrder(t, orderSvc, third, "cp-acc-txn-004")
	if got := readProfile(t, db, merchant.ID, "13800000012"); got.OrderCount != 1 || got.TotalAmountCents != 700 {
		t.Fatalf("other-phone profile=%+v, want count 1 total 700", got)
	}
	if countProfiles(t, db) != 3 {
		t.Fatalf("profiles=%d, want 3", countProfiles(t, db))
	}
}

func TestCustomerProfilePaymentIdempotency(t *testing.T) {
	orderSvc, _, _, db := newCustomerProfileServices(t)
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "CP-IDEM-001", "pending", 2500, nil)
	setProfileOrderFacts(t, db, order.ID, "13800000021", time.Now().Add(-10*time.Minute), 0)

	payProfileOrder(t, orderSvc, order, "cp-idem-txn-001")
	// Same transaction number again: the callback short-circuits on the
	// existing payment record and must not recount.
	if _, err := orderSvc.HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo: order.OrderNo, TransactionNo: "cp-idem-txn-001",
		PaymentChannel: "alipay", Amount: order.TotalAmount, Status: "success",
	}); err != nil {
		t.Fatalf("repeat callback with same transaction: %v", err)
	}
	// A second, distinct successful payment for the same order is a
	// duplicate-payment exception (PaidAt already set): still no recount.
	if _, err := orderSvc.HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo: order.OrderNo, TransactionNo: "cp-idem-txn-002",
		PaymentChannel: "alipay", Amount: order.TotalAmount, Status: "success",
	}); err != nil {
		t.Fatalf("duplicate success callback: %v", err)
	}
	if got := readProfile(t, db, merchant.ID, "13800000021"); got.OrderCount != 1 || got.TotalAmountCents != 2500 {
		t.Fatalf("profile after duplicate callbacks=%+v, want count 1 total 2500", got)
	}

	// Merchant manual confirmation builds the same row, and a second
	// confirmation (PaidAt already set) is a no-op.
	_, _, _, manual := createSyntheticStoreOrder(t, db, "CP-IDEM-002", "submitted", 1200, nil)
	if err := db.Exec("UPDATE orders SET merchant_id = ? WHERE id = ?", merchant.ID, manual.ID).Error; err != nil {
		t.Fatal(err)
	}
	setProfileOrderFacts(t, db, manual.ID, "13800000022", time.Now().Add(-30*time.Minute), 0)
	if _, err := orderSvc.ConfirmMerchantOrderPayment(merchant.ID, manual.ID); err != nil {
		t.Fatalf("manual payment confirmation: %v", err)
	}
	if _, err := orderSvc.ConfirmMerchantOrderPayment(merchant.ID, manual.ID); err != nil {
		t.Fatalf("repeat manual payment confirmation: %v", err)
	}
	if got := readProfile(t, db, merchant.ID, "13800000022"); got.OrderCount != 1 || got.TotalAmountCents != 1200 || got.LastStatus != "received" {
		t.Fatalf("manual-confirm profile=%+v, want count 1 total 1200 received", got)
	}
}

func TestCustomerProfileLastStatusOnlyForLatestOrder(t *testing.T) {
	orderSvc, _, _, db := newCustomerProfileServices(t)
	merchant, _, _, first := createSyntheticStoreOrder(t, db, "CP-STATUS-001", "pending", 1000, nil)
	base := time.Now().Add(-20 * time.Minute)
	setProfileOrderFacts(t, db, first.ID, "13800000031", base, 0)
	payProfileOrder(t, orderSvc, first, "cp-status-txn-001")

	_, _, _, second := createSyntheticStoreOrder(t, db, "CP-STATUS-002", "pending", 2000, nil)
	if err := db.Exec("UPDATE orders SET merchant_id = ?, store_id = ? WHERE id = ?", merchant.ID, first.StoreID, second.ID).Error; err != nil {
		t.Fatal(err)
	}
	setProfileOrderFacts(t, db, second.ID, "13800000031", base.Add(10*time.Minute), 0)
	payProfileOrder(t, orderSvc, second, "cp-status-txn-002")

	lastStatus := func() string { return readProfile(t, db, merchant.ID, "13800000031").LastStatus }
	if lastStatus() != "received" {
		t.Fatalf("last_status=%q, want received", lastStatus())
	}
	// Accepting the older order must not rewrite the latest status.
	if err := orderSvc.AcceptMerchantOrder(merchant.ID, first.ID); err != nil {
		t.Fatalf("accept older order: %v", err)
	}
	if lastStatus() != "received" {
		t.Fatalf("accepting older order changed last_status to %q", lastStatus())
	}
	// Accepting then completing the latest order moves it.
	if err := orderSvc.AcceptMerchantOrder(merchant.ID, second.ID); err != nil {
		t.Fatalf("accept latest order: %v", err)
	}
	if lastStatus() != "accepted" {
		t.Fatalf("last_status=%q, want accepted", lastStatus())
	}
	if err := orderSvc.CompleteMerchantOrder(merchant.ID, second.ID); err != nil {
		t.Fatalf("complete latest order: %v", err)
	}
	if lastStatus() != "completed" {
		t.Fatalf("last_status=%q, want completed", lastStatus())
	}
	// Closing the older (accepted) order leaves the latest status alone.
	if err := orderSvc.CloseMerchantOrder(merchant.ID, first.ID); err != nil {
		t.Fatalf("close older order: %v", err)
	}
	if lastStatus() != "completed" {
		t.Fatalf("closing older order changed last_status to %q", lastStatus())
	}
	// A third, newer paid order becomes the latest (received); closing it
	// is the close that rewrites last_status. (A completed order cannot be
	// closed at all, so the close assertion needs a newer latest order.)
	_, _, _, third := createSyntheticStoreOrder(t, db, "CP-STATUS-004", "pending", 4000, nil)
	if err := db.Exec("UPDATE orders SET merchant_id = ?, store_id = ? WHERE id = ?", merchant.ID, first.StoreID, third.ID).Error; err != nil {
		t.Fatal(err)
	}
	setProfileOrderFacts(t, db, third.ID, "13800000031", base.Add(15*time.Minute), 0)
	payProfileOrder(t, orderSvc, third, "cp-status-txn-003")
	if lastStatus() != "received" {
		t.Fatalf("last_status=%q after third payment, want received", lastStatus())
	}
	if err := orderSvc.CloseMerchantOrder(merchant.ID, third.ID); err != nil {
		t.Fatalf("close latest order: %v", err)
	}
	if lastStatus() != "closed" {
		t.Fatalf("last_status=%q, want closed", lastStatus())
	}
	// Closing an unpaid order never touches the profile.
	_, _, _, unpaid := createSyntheticStoreOrder(t, db, "CP-STATUS-003", "submitted", 3000, nil)
	if err := db.Exec("UPDATE orders SET merchant_id = ?, store_id = ? WHERE id = ?", merchant.ID, first.StoreID, unpaid.ID).Error; err != nil {
		t.Fatal(err)
	}
	setProfileOrderFacts(t, db, unpaid.ID, "13800000031", base.Add(15*time.Minute), 0)
	if err := orderSvc.CloseMerchantOrder(merchant.ID, unpaid.ID); err != nil {
		t.Fatalf("close unpaid order: %v", err)
	}
	if lastStatus() != "closed" {
		t.Fatalf("closing unpaid order changed last_status to %q", lastStatus())
	}
	if got := readProfile(t, db, merchant.ID, "13800000031"); got.OrderCount != 3 {
		t.Fatalf("order count=%d after status changes, want 3", got.OrderCount)
	}
}

func TestCustomerProfileAITagBoundaries(t *testing.T) {
	_, adminSvc, _, db := newCustomerProfileServices(t)
	merchant, store, _, _ := createSyntheticStoreOrder(t, db, "CP-TAG-SEED", "pending", 100, nil)
	now := time.Now()
	seedPaidOrder := func(phone, orderNo, status string, amount int64, createdAt time.Time) {
		t.Helper()
		_, _, _, order := createSyntheticStoreOrder(t, db, orderNo, status, amount, nil)
		if err := db.Exec("UPDATE orders SET merchant_id = ?, store_id = ? WHERE id = ?", merchant.ID, store.ID, order.ID).Error; err != nil {
			t.Fatal(err)
		}
		paidAt := createdAt
		if err := db.Exec("UPDATE orders SET customer_phone = ?, created_at = ?, paid_at = ?, discount_amount = 0 WHERE id = ?", phone, createdAt, paidAt, order.ID).Error; err != nil {
			t.Fatalf("seed tag order %s: %v", orderNo, err)
		}
	}
	// High value by amount (>= 10000 fen in one order).
	seedPaidOrder("13800000101", "CP-TAG-HIGH-AMT", "received", 10000, now.Add(-time.Hour))
	// High value by count (>= 5 orders).
	for i := 0; i < 5; i++ {
		seedPaidOrder("13800000102", "CP-TAG-HIGH-CNT-0"+string(rune('0'+i)), "received", 100, now.Add(-time.Duration(i+1)*time.Hour))
	}
	// Repeat (>= 2 orders, below the high-value lines).
	seedPaidOrder("13800000103", "CP-TAG-REP-1", "received", 100, now.Add(-2*time.Hour))
	seedPaidOrder("13800000103", "CP-TAG-REP-2", "received", 100, now.Add(-time.Hour))
	// Sleeping: last order 31 days ago (30-day boundary, safely past it).
	seedPaidOrder("13800000104", "CP-TAG-SLEEP", "received", 100, now.AddDate(0, 0, -31))
	// Not sleeping yet: last order 29 days ago.
	seedPaidOrder("13800000105", "CP-TAG-AWAKE", "received", 100, now.AddDate(0, 0, -29))
	// At-risk: latest order closed, recent, single order.
	seedPaidOrder("13800000106", "CP-TAG-RISK", "closed", 100, now.Add(-time.Hour))
	// New customer: one recent received order.
	seedPaidOrder("13800000107", "CP-TAG-NEW", "received", 100, now.Add(-time.Hour))

	if _, err := adminSvc.RebuildCustomerProfiles(0); err != nil {
		t.Fatalf("rebuild for AITag test: %v", err)
	}
	rows, total, err := adminSvc.ListCustomerProfiles(1, 100, CustomerProfileFilter{})
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if total != 7 {
		t.Fatalf("total=%d, want 7", total)
	}
	byPhone := map[string]CustomerProfileRow{}
	for _, row := range rows {
		byPhone[row.CustomerPhone] = row
	}
	want := map[string]string{
		"13800000101": "高价值顾客",
		"13800000102": "高价值顾客",
		"13800000103": "复购顾客",
		"13800000104": "沉睡顾客",
		"13800000105": "新顾客",
		"13800000106": "流失风险",
		"13800000107": "新顾客",
	}
	for phone, tag := range want {
		row, ok := byPhone[phone]
		if !ok {
			t.Fatalf("missing profile for %s", phone)
		}
		if row.AITag != tag {
			t.Fatalf("phone %s tag=%q, want %q (row=%+v)", phone, row.AITag, tag, row)
		}
		if row.AISuggestion == "" {
			t.Fatalf("phone %s must carry a suggestion", phone)
		}
	}
	// The Go-side ai_tag filter must see the same rows.
	filtered, filteredTotal, err := adminSvc.ListCustomerProfiles(1, 100, CustomerProfileFilter{AITag: "沉睡顾客"})
	if err != nil {
		t.Fatalf("list sleeping profiles: %v", err)
	}
	if filteredTotal != 1 || len(filtered) != 1 || filtered[0].CustomerPhone != "13800000104" {
		t.Fatalf("ai_tag filter total=%d rows=%v, want 1 sleeping row", filteredTotal, filtered)
	}
	closed, closedTotal, err := adminSvc.ListCustomerProfiles(1, 100, CustomerProfileFilter{Status: "closed"})
	if err != nil {
		t.Fatalf("list closed profiles: %v", err)
	}
	if closedTotal != 1 || len(closed) != 1 || closed[0].CustomerPhone != "13800000106" {
		t.Fatalf("status filter total=%d rows=%v, want 1 closed row", closedTotal, closed)
	}
}

func TestOpsDormantCustomersTierBoundariesFromProfiles(t *testing.T) {
	_, _, merchantSvc, db := newCustomerProfileServices(t)
	// Reuse the ops fixture seed style: paid_at on Beijing calendar days.
	seed := func(phone string, daysAgo int) {
		t.Helper()
		paid := opsBeijingDay(t, daysAgo)
		order := model.Order{
			OrderNo: "CP-DORM-" + phone, OrderType: "store_order",
			MerchantID: ptrUint(1), CustomerPhone: phone,
			TotalAmount: 1000, Status: "received", PaidAt: &paid,
		}
		order.CreatedAt = paid
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("seed dormant order %s: %v", phone, err)
		}
	}
	seed("13800000230", 30) // not dormant yet
	seed("13800000231", 31) // light lower bound
	seed("13800000260", 60) // light upper bound
	seed("13800000261", 61) // medium lower bound
	seed("13800000290", 90) // medium upper bound
	seed("13800000291", 91) // deep
	if _, err := rebuildCustomerProfiles(db, 0); err != nil {
		t.Fatalf("rebuild for dormant tiers: %v", err)
	}
	dormant, err := merchantSvc.OpsDormantCustomers(1)
	if err != nil {
		t.Fatalf("dormant customers: %v", err)
	}
	byTier := map[string]OpsDormantTier{}
	for _, tier := range dormant.Tiers {
		byTier[tier.Tier] = tier
	}
	if byTier["light"].Count != 2 || byTier["medium"].Count != 2 || byTier["deep"].Count != 1 {
		t.Fatalf("tier counts light/medium/deep=%d/%d/%d, want 2/2/1",
			byTier["light"].Count, byTier["medium"].Count, byTier["deep"].Count)
	}
	for _, tier := range dormant.Tiers {
		for _, phone := range tier.SamplePhones {
			if len(phone) != len("138****0231") || phone[3:7] != "****" {
				t.Fatalf("phone %q must be masked as 138****0231 style", phone)
			}
		}
	}
}

// legacyProfileScanRow mirrors CustomerProfileRow, except last_order_at is
// a string: MAX(created_at) is an expression column, and the SQLite driver
// hands it back as a string that database/sql refuses to scan into
// time.Time (the same P3 lesson as MAX(paid_at) in the ops dashboard).
type legacyProfileScanRow struct {
	CustomerPhone  string `gorm:"column:customer_phone"`
	MerchantID     uint   `gorm:"column:merchant_id"`
	MerchantName   string `gorm:"column:merchant_name"`
	StoreName      string `gorm:"column:store_name"`
	OrderCount     int64  `gorm:"column:order_count"`
	TotalAmount    int64  `gorm:"column:total_amount"`
	DiscountAmount int64  `gorm:"column:discount_amount"`
	LastOrderAt    string `gorm:"column:last_order_at"`
}

func parseSQLiteTime(t *testing.T, value string) time.Time {
	t.Helper()
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	t.Fatalf("parse sqlite time %q", value)
	return time.Time{}
}

func legacyRowsFromScan(t *testing.T, db *gorm.DB, scanned []legacyProfileScanRow) []CustomerProfileRow {
	t.Helper()
	rows := make([]CustomerProfileRow, 0, len(scanned))
	for _, item := range scanned {
		row := CustomerProfileRow{
			CustomerPhone:  item.CustomerPhone,
			MerchantID:     item.MerchantID,
			MerchantName:   item.MerchantName,
			StoreName:      item.StoreName,
			OrderCount:     item.OrderCount,
			TotalAmount:    item.TotalAmount,
			DiscountAmount: item.DiscountAmount,
			LastOrderAt:    parseSQLiteTime(t, item.LastOrderAt),
		}
		var latest model.Order
		if err := db.Where("merchant_id = ? AND customer_phone = ? AND order_type = ?", row.MerchantID, row.CustomerPhone, "store_order").
			Where(ValidTradeAmountCondition).
			Order("created_at DESC, id DESC").First(&latest).Error; err != nil {
			t.Fatalf("legacy latest status for %s: %v", row.CustomerPhone, err)
		}
		row.LastStatus = latest.Status
		rows = append(rows, row)
	}
	return rows
}
// over orders), kept here as the equivalence oracle. It is a verbatim copy
// of the old AdminService.ListCustomerProfiles logic, except for last_status:
// the old SUBSTRING_INDEX(GROUP_CONCAT(...)) expression is MySQL-only, so
// on SQLite the latest status is read from the latest order row instead —
// the same value, since the seed orders have distinct created_at values.
func legacyListCustomerProfiles(t *testing.T, db *gorm.DB, page, pageSize int, filter CustomerProfileFilter) ([]CustomerProfileRow, int64) {
	t.Helper()
	var scanned []legacyProfileScanRow
	query := db.Table("orders").
		Select(`orders.customer_phone AS customer_phone,
			orders.merchant_id AS merchant_id,
			MAX(merchants.name) AS merchant_name,
			MAX(stores.name) AS store_name,
			COUNT(orders.id) AS order_count,
			COALESCE(SUM(orders.total_amount),0) AS total_amount,
			COALESCE(SUM(orders.discount_amount),0) AS discount_amount,
			MAX(orders.created_at) AS last_order_at`).
		Joins("LEFT JOIN merchants ON merchants.id = orders.merchant_id").
		Joins("LEFT JOIN stores ON stores.id = orders.store_id").
		Where("orders.order_type = ? AND orders.customer_phone <> ''", "store_order").
		Where(ValidTradeAmountConditionFor("orders"))
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("orders.customer_phone LIKE ? OR merchants.name LIKE ? OR stores.name LIKE ?", like, like, like)
	}
	query = query.Group("orders.customer_phone, orders.merchant_id")
	if err := query.Order("last_order_at desc").Scan(&scanned).Error; err != nil {
		t.Fatalf("legacy list: %v", err)
	}
	rows := legacyRowsFromScan(t, db, scanned)
	for i := range rows {
		rows[i].AITag, rows[i].AISuggestion = buildCustomerInsight(rows[i])
	}
	aiTag := strings.TrimSpace(filter.AITag)
	status := strings.TrimSpace(filter.Status)
	if aiTag != "" || status != "" {
		filtered := rows[:0]
		for _, row := range rows {
			if aiTag != "" && row.AITag != aiTag {
				continue
			}
			if status != "" && row.LastStatus != status {
				continue
			}
			filtered = append(filtered, row)
		}
		rows = filtered
	}
	sortBy := strings.TrimSpace(filter.SortBy)
	sortOrder := strings.ToLower(strings.TrimSpace(filter.SortOrder))
	desc := sortOrder != "asc"
	sort.SliceStable(rows, func(i, j int) bool {
		var compare int
		switch sortBy {
		case "order_count":
			compare = int(rows[i].OrderCount - rows[j].OrderCount)
		case "total_amount":
			compare = int(rows[i].TotalAmount - rows[j].TotalAmount)
		case "discount_amount":
			compare = int(rows[i].DiscountAmount - rows[j].DiscountAmount)
		case "customer_phone":
			compare = strings.Compare(rows[i].CustomerPhone, rows[j].CustomerPhone)
		default:
			compare = rows[i].LastOrderAt.Compare(rows[j].LastOrderAt)
		}
		if compare == 0 {
			compare = rows[i].LastOrderAt.Compare(rows[j].LastOrderAt)
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
	total := int64(len(rows))
	start := (page - 1) * pageSize
	if start >= len(rows) {
		return []CustomerProfileRow{}, total
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], total
}

func legacyListMerchantCustomerProfiles(t *testing.T, db *gorm.DB, merchantID uint, limit int) []CustomerProfileRow {
	t.Helper()
	if limit <= 0 {
		limit = 10
	}
	var scanned []legacyProfileScanRow
	err := db.Table("orders").
		Select(`orders.customer_phone AS customer_phone,
			orders.merchant_id AS merchant_id,
			MAX(merchants.name) AS merchant_name,
			MAX(stores.name) AS store_name,
			COUNT(orders.id) AS order_count,
			COALESCE(SUM(orders.total_amount),0) AS total_amount,
			COALESCE(SUM(orders.discount_amount),0) AS discount_amount,
			MAX(orders.created_at) AS last_order_at`).
		Joins("LEFT JOIN merchants ON merchants.id = orders.merchant_id").
		Joins("LEFT JOIN stores ON stores.id = orders.store_id").
		Where("orders.merchant_id = ? AND orders.order_type = ? AND orders.customer_phone <> ''", merchantID, "store_order").
		Where(ValidTradeAmountConditionFor("orders")).
		Group("orders.customer_phone, orders.merchant_id").
		Order("total_amount desc, last_order_at desc").
		Limit(limit).
		Scan(&scanned).Error
	if err != nil {
		t.Fatalf("legacy merchant list: %v", err)
	}
	// Same MySQL-only GROUP_CONCAT note as legacyListCustomerProfiles.
	rows := legacyRowsFromScan(t, db, scanned)
	for i := range rows {
		rows[i].AITag, rows[i].AISuggestion = buildCustomerInsight(rows[i])
		rows[i].CustomerPhone = maskPhoneForTest(rows[i].CustomerPhone)
	}
	return rows
}

func maskPhoneForTest(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) <= 7 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

func assertProfileRowsEqual(t *testing.T, label string, want, got []CustomerProfileRow) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s row count=%d, want %d (want=%v got=%v)", label, len(got), len(want), want, got)
	}
	for i := range want {
		w, g := want[i], got[i]
		if w.CustomerPhone != g.CustomerPhone || w.MerchantID != g.MerchantID || w.MerchantName != g.MerchantName ||
			w.StoreName != g.StoreName || w.OrderCount != g.OrderCount || w.TotalAmount != g.TotalAmount ||
			w.DiscountAmount != g.DiscountAmount || w.LastStatus != g.LastStatus ||
			w.AITag != g.AITag || w.AISuggestion != g.AISuggestion ||
			w.LastOrderAt.UnixMilli() != g.LastOrderAt.UnixMilli() {
			t.Fatalf("%s row %d differs:\n want=%+v\n got =%+v", label, i, w, g)
		}
	}
}

func seedEquivalenceOrders(t *testing.T, db *gorm.DB) (merchantA, merchantB model.Merchant) {
	t.Helper()
	now := time.Now()
	mk := func(merchant model.Merchant, store model.Store, phone, orderNo, status string, amount, discount int64, createdAt time.Time) {
		t.Helper()
		paidAt := createdAt
		order := model.Order{
			OrderNo: orderNo, OrderType: "store_order",
			MerchantID: &merchant.ID, StoreID: &store.ID, CustomerPhone: phone,
			TotalAmount: amount, DiscountAmount: discount, Status: status,
			PaidAt: &paidAt,
		}
		order.CreatedAt = createdAt
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("seed equivalence order %s: %v", orderNo, err)
		}
	}
	merchantA, storeA, _, _ := createSyntheticStoreOrder(t, db, "CP-EQ-SEED-A", "pending", 100, nil)
	merchantB, storeB, _, _ := createSyntheticStoreOrder(t, db, "CP-EQ-SEED-B", "pending", 100, nil)
	// Merchant A, one store (so MAX(stores.name) == latest store name).
	mk(merchantA, storeA, "13800000301", "CP-EQ-A1-1", "completed", 6000, 500, now.Add(-5*time.Hour))
	mk(merchantA, storeA, "13800000301", "CP-EQ-A1-2", "received", 4000, 0, now.Add(-time.Hour))
	mk(merchantA, storeA, "13800000302", "CP-EQ-A2-1", "closed", 300, 0, now.Add(-2*time.Hour))
	mk(merchantA, storeA, "13800000302", "CP-EQ-A2-2", "received", 300, 0, now.Add(-30*time.Minute))
	mk(merchantA, storeA, "13800000303", "CP-EQ-A3-1", "received", 12000, 1000, now.Add(-4*time.Hour))
	// Merchant B, one store.
	mk(merchantB, storeB, "13800000301", "CP-EQ-B1-1", "received", 900, 0, now.Add(-3*time.Hour))
	mk(merchantB, storeB, "13800000304", "CP-EQ-B4-1", "received", 200, 0, now.Add(-90*time.Minute))
	mk(merchantB, storeB, "13800000304", "CP-EQ-B4-2", "received", 200, 0, now.Add(-80*time.Minute))
	mk(merchantB, storeB, "13800000304", "CP-EQ-B4-3", "received", 200, 0, now.Add(-70*time.Minute))
	// Excluded from both old and new reads: unpaid, zero-amount, empty
	// phone, non-store order.
	unpaid := model.Order{OrderNo: "CP-EQ-X-UNPAID", OrderType: "store_order", MerchantID: &merchantA.ID, StoreID: &storeA.ID, CustomerPhone: "13800000301", TotalAmount: 9999, Status: "pending"}
	if err := db.Create(&unpaid).Error; err != nil {
		t.Fatalf("seed unpaid: %v", err)
	}
	zeroPaid := now.Add(-10 * time.Minute)
	zero := model.Order{OrderNo: "CP-EQ-X-ZERO", OrderType: "store_order", MerchantID: &merchantA.ID, StoreID: &storeA.ID, CustomerPhone: "13800000302", TotalAmount: 0, Status: "received", PaidAt: &zeroPaid}
	if err := db.Create(&zero).Error; err != nil {
		t.Fatalf("seed zero: %v", err)
	}
	empty := model.Order{OrderNo: "CP-EQ-X-EMPTY", OrderType: "store_order", MerchantID: &merchantA.ID, StoreID: &storeA.ID, CustomerPhone: "", TotalAmount: 5000, Status: "received", PaidAt: &zeroPaid}
	if err := db.Create(&empty).Error; err != nil {
		t.Fatalf("seed empty phone: %v", err)
	}
	sub := model.Order{OrderNo: "CP-EQ-X-SUB", OrderType: "merchant_subscription", MerchantID: &merchantA.ID, CustomerPhone: "13800000301", TotalAmount: 19900, Status: "paid", PaidAt: &zeroPaid}
	if err := db.Create(&sub).Error; err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return merchantA, merchantB
}

func TestCustomerProfileReadEquivalenceWithLegacyAggregation(t *testing.T) {
	_, adminSvc, _, db := newCustomerProfileServices(t)
	merchantA, merchantB := seedEquivalenceOrders(t, db)
	if _, err := adminSvc.RebuildCustomerProfiles(0); err != nil {
		t.Fatalf("rebuild for equivalence: %v", err)
	}
	combos := []struct {
		name   string
		page   int
		size   int
		filter CustomerProfileFilter
	}{
		{"all", 1, 10000, CustomerProfileFilter{}},
		{"page 2 of 2", 2, 2, CustomerProfileFilter{}},
		{"page beyond end", 99, 2, CustomerProfileFilter{}},
		{"keyword phone fragment", 1, 10000, CustomerProfileFilter{Keyword: "0301"}},
		{"keyword merchant name", 1, 10000, CustomerProfileFilter{Keyword: "CP-EQ-SEED"}},
		{"ai_tag repeat", 1, 10000, CustomerProfileFilter{AITag: "复购顾客"}},
		{"ai_tag high value", 1, 10000, CustomerProfileFilter{AITag: "高价值顾客"}},
		{"status closed", 1, 10000, CustomerProfileFilter{Status: "closed"}},
		{"status received + sort total asc", 1, 10000, CustomerProfileFilter{Status: "received", SortBy: "total_amount", SortOrder: "asc"}},
		{"sort order_count desc", 1, 10000, CustomerProfileFilter{SortBy: "order_count", SortOrder: "desc"}},
		{"sort total_amount asc", 1, 10000, CustomerProfileFilter{SortBy: "total_amount", SortOrder: "asc"}},
		{"sort discount desc", 1, 10000, CustomerProfileFilter{SortBy: "discount_amount", SortOrder: "desc"}},
		{"sort phone asc", 1, 10000, CustomerProfileFilter{SortBy: "customer_phone", SortOrder: "asc"}},
		{"export 10000 cap", 1, 10000, CustomerProfileFilter{SortBy: "total_amount", SortOrder: "desc"}},
	}
	for _, tc := range combos {
		want, wantTotal := legacyListCustomerProfiles(t, db, tc.page, tc.size, tc.filter)
		got, gotTotal, err := adminSvc.ListCustomerProfiles(tc.page, tc.size, tc.filter)
		if err != nil {
			t.Fatalf("%s: new list: %v", tc.name, err)
		}
		if wantTotal != gotTotal {
			t.Fatalf("%s: total=%d, want %d", tc.name, gotTotal, wantTotal)
		}
		assertProfileRowsEqual(t, tc.name, want, got)
	}
	// Merchant Top N, both merchants.
	for _, m := range []model.Merchant{merchantA, merchantB} {
		want := legacyListMerchantCustomerProfiles(t, db, m.ID, 10)
		got, err := adminSvc.ListMerchantCustomerProfiles(m.ID, 10)
		if err != nil {
			t.Fatalf("merchant %d new top: %v", m.ID, err)
		}
		assertProfileRowsEqual(t, "merchant top", want, got)
	}
}

func TestCustomerProfileRebuildMatchesLegacyAggregation(t *testing.T) {
	_, adminSvc, _, db := newCustomerProfileServices(t)
	merchantA, merchantB := seedEquivalenceOrders(t, db)

	rebuilt, err := adminSvc.RebuildCustomerProfiles(0)
	if err != nil {
		t.Fatalf("rebuild all: %v", err)
	}
	if rebuilt != 5 {
		t.Fatalf("rebuilt=%d, want 5 profiles (3 merchant A + 2 merchant B)", rebuilt)
	}
	want, _ := legacyListCustomerProfiles(t, db, 1, 10000, CustomerProfileFilter{})
	got, total, err := adminSvc.ListCustomerProfiles(1, 10000, CustomerProfileFilter{})
	if err != nil {
		t.Fatalf("list after rebuild: %v", err)
	}
	if total != 5 {
		t.Fatalf("total=%d, want 5", total)
	}
	assertProfileRowsEqual(t, "rebuild vs legacy", want, got)

	// Per-merchant rebuild: corrupt merchant B's rows, rebuild only
	// merchant A, and merchant B must stay corrupted.
	if err := db.Exec("UPDATE customer_profiles SET total_amount_cents = 1 WHERE merchant_id = ?", merchantB.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := adminSvc.RebuildCustomerProfiles(merchantA.ID); err != nil {
		t.Fatalf("rebuild merchant A: %v", err)
	}
	var corrupted int64
	if err := db.Model(&model.CustomerProfile{}).Where("merchant_id = ? AND total_amount_cents = 1", merchantB.ID).Count(&corrupted).Error; err != nil {
		t.Fatal(err)
	}
	if corrupted != 2 {
		t.Fatalf("merchant B corrupted rows=%d, want 2 untouched by merchant A rebuild", corrupted)
	}
	// Full rebuild repairs merchant B too.
	if _, err := adminSvc.RebuildCustomerProfiles(0); err != nil {
		t.Fatalf("rebuild all again: %v", err)
	}
	got, total, err = adminSvc.ListCustomerProfiles(1, 10000, CustomerProfileFilter{})
	if err != nil {
		t.Fatalf("list after full rebuild: %v", err)
	}
	if total != 5 {
		t.Fatalf("total=%d, want 5", total)
	}
	assertProfileRowsEqual(t, "rebuild again vs legacy", want, got)
}

func TestCustomerProfileLatestStoreNameIsLatestOrderStore(t *testing.T) {
	_, adminSvc, _, db := newCustomerProfileServices(t)
	merchant, storeA, _, _ := createSyntheticStoreOrder(t, db, "CP-STORE-SEED", "pending", 100, nil)
	storeB := model.Store{MerchantID: merchant.ID, Name: "第二门店", Status: "active", IsOpen: true}
	if err := db.Create(&storeB).Error; err != nil {
		t.Fatalf("create second store: %v", err)
	}
	now := time.Now()
	mk := func(orderNo, status string, store model.Store, amount int64, createdAt time.Time) {
		t.Helper()
		paidAt := createdAt
		order := model.Order{
			OrderNo: orderNo, OrderType: "store_order",
			MerchantID: &merchant.ID, StoreID: &store.ID, CustomerPhone: "13800000401",
			TotalAmount: amount, Status: status, PaidAt: &paidAt,
		}
		order.CreatedAt = createdAt
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("seed store order %s: %v", orderNo, err)
		}
	}
	// Older order at the alphabetically-later store, newer order at the
	// other store: MAX(stores.name) would pick the wrong one.
	mk("CP-STORE-OLD", "completed", storeB, 1000, now.Add(-2*time.Hour))
	mk("CP-STORE-NEW", "received", storeA, 2000, now.Add(-time.Hour))
	if _, err := adminSvc.RebuildCustomerProfiles(0); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	profile := readProfile(t, db, merchant.ID, "13800000401")
	if profile.LastStoreName != storeA.Name || profile.LastStoreID == nil || *profile.LastStoreID != storeA.ID {
		t.Fatalf("last store=%v/%q, want %d/%q (latest order's store, not MAX name)", profile.LastStoreID, profile.LastStoreName, storeA.ID, storeA.Name)
	}
}

func TestConcurrentCustomerProfilePaymentsDoNotLoseCounts(t *testing.T) {
	orderSvc, _, _, db := newCustomerProfileServices(t)
	useSerializedSyntheticDB(t, db)
	merchant, _, _, first := createSyntheticStoreOrder(t, db, "CP-CONC-001", "pending", 1000, nil)
	_, _, _, second := createSyntheticStoreOrder(t, db, "CP-CONC-002", "pending", 2000, nil)
	if err := db.Exec("UPDATE orders SET merchant_id = ?, store_id = ? WHERE id = ?", merchant.ID, first.StoreID, second.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, o := range []model.Order{first, second} {
		setProfileOrderFacts(t, db, o.ID, "13800000501", time.Now().Add(-10*time.Minute), 0)
	}
	results := runConcurrentCallbacks(orderSvc, []dto.PaymentCallbackRequest{
		{OrderNo: first.OrderNo, TransactionNo: "cp-conc-txn-001", PaymentChannel: "alipay", Amount: first.TotalAmount, Status: "success"},
		{OrderNo: second.OrderNo, TransactionNo: "cp-conc-txn-002", PaymentChannel: "alipay", Amount: second.TotalAmount, Status: "success"},
	})
	for _, r := range results {
		if r.err != nil {
			t.Fatalf("concurrent callback for %s: %v", r.callback.OrderNo, r.err)
		}
	}
	profile := readProfile(t, db, merchant.ID, "13800000501")
	if profile.OrderCount != 2 || profile.TotalAmountCents != 3000 {
		t.Fatalf("profile after concurrent payments count=%d total=%d, want 2/3000", profile.OrderCount, profile.TotalAmountCents)
	}
}
