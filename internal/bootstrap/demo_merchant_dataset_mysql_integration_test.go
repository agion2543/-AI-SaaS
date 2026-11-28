package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

const (
	demoSeedITDSNEnv  = "MYSQL_DEMO_SEED_IT_DSN"
	demoSeedITRootEnv = "MYSQL_DEMO_SEED_IT_ROOT"
)

// openSafeDemoSeedMySQL mirrors the other MySQL integration tests: the
// test only runs against a disposable socket-only MySQL owned by the
// harness script (see /home/hatch/.cache/mysql-it/run-it.sh) and skips
// otherwise. It refuses any ambient MYSQL_DSN/MYSQL_PWD.
func openSafeDemoSeedMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	if strings.TrimSpace(os.Getenv("MYSQL_DSN")) != "" || strings.TrimSpace(os.Getenv("MYSQL_PWD")) != "" {
		t.Fatal("refusing demo-seed integration test while MYSQL_DSN or MYSQL_PWD is set")
	}
	raw := strings.TrimSpace(os.Getenv(demoSeedITDSNEnv))
	rootValue := strings.TrimSpace(os.Getenv(demoSeedITRootEnv))
	if raw == "" || rootValue == "" {
		t.Skipf("set %s and %s from the one-off MySQL harness to run", demoSeedITDSNEnv, demoSeedITRootEnv)
	}
	rootInfo, err := os.Lstat(rootValue)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatal("refusing missing, non-directory, or symlinked temporary MySQL root")
	}
	root, err := filepath.EvalSymlinks(rootValue)
	if err != nil || filepath.Clean(rootValue) != root || !strings.HasPrefix(root, "/tmp/cue-demo-seed-mysql-") {
		t.Fatal("refusing MySQL root outside a canonical /tmp/cue-demo-seed-mysql-* directory")
	}
	if _, err := os.Stat(filepath.Join(root, ".cue-demo-seed-it-owned")); err != nil {
		t.Fatal("refusing MySQL root without the test-run ownership marker")
	}
	parsed, err := mysqlDriver.ParseDSN(raw)
	if err != nil {
		t.Fatal("refusing malformed disposable MySQL DSN")
	}
	if parsed.Net != "unix" || parsed.User != "root" || parsed.Passwd != "" || !strings.HasPrefix(parsed.DBName, "cue_demo_seed_it_") {
		t.Fatal("refusing DSN: require passwordless root, Unix socket, and cue_demo_seed_it_* database")
	}
	socket := filepath.Clean(parsed.Addr)
	if !filepath.IsAbs(socket) || !strings.HasPrefix(socket, root+string(filepath.Separator)) {
		t.Fatal("refusing MySQL socket outside the temporary MySQL root")
	}
	db, err := gorm.Open(gormmysql.Open(raw), &gorm.Config{})
	if err != nil {
		t.Fatalf("open disposable demo-seed MySQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get disposable demo-seed SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.AdminRole{}, &model.AdminUser{}, &model.MembershipPackage{}, &model.MerchantPlan{},
		&model.Merchant{}, &model.MerchantPaymentConfig{}, &model.Store{}, &model.StoreProduct{},
		&model.Order{}, &model.OrderItem{}, &model.PaymentRecord{}, &model.RefundRecord{},
		&model.CustomerLead{}, &model.ShareCampaign{}, &model.ShareActivityConfig{}, &model.ReferralCoupon{},
		&model.Promotion{}, &model.CampaignTarget{}, &model.CampaignPostmortem{},
		&model.AISuggestionExecution{}, &model.GroupBuyManualRedemption{},
		&model.CustomerProfile{}, &model.MerchantGroupBuyLink{}, &model.MerchantGroupBuyLinkAudit{},
		&model.User{}, &model.VerificationCode{}, &model.MaintenancePlan{}, &model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate disposable demo-seed schema: %v", err)
	}
	return db
}

// TestMySQLDemoSeedDatasetAndOpsDashboard runs the reworked demo seed
// twice on real MySQL 8 and checks the P3 ops dashboard reads the seeded
// numbers back through the real service methods, plus a live customer
// order against the demo merchant (active + live subscription gate).
func TestMySQLDemoSeedDatasetAndOpsDashboard(t *testing.T) {
	db := openSafeDemoSeedMySQL(t)

	if err := seedDemoMerchantDataset(db); err != nil {
		t.Fatalf("seed demo merchant dataset on MySQL: %v", err)
	}
	before := demoDatasetCounts(t, db)
	if err := seedDemoMerchantDataset(db); err != nil {
		t.Fatalf("reseed demo merchant dataset on MySQL: %v", err)
	}
	after := demoDatasetCounts(t, db)
	for key, want := range before {
		if after[key] != want {
			t.Errorf("MySQL reseed changed %s count: before=%d after=%d", key, want, after[key])
		}
	}
	if after["merchants"] != 1 || after["orders"] != 16 || after["order_items"] != 28 || after["profiles"] != 15 {
		t.Fatalf("MySQL demo counts unexpected: %+v", after)
	}

	var merchant model.Merchant
	if err := db.Where("name = ?", demoCafeMerchantName).First(&merchant).Error; err != nil {
		t.Fatalf("load demo merchant on MySQL: %v", err)
	}
	if merchant.Status != "active" || merchant.SubscriptionStatus != "active" ||
		merchant.SubscriptionExpireAt == nil || merchant.SubscriptionExpireAt.Before(time.Now().AddDate(0, 0, 7)) {
		t.Fatalf("demo merchant not order-ready on MySQL: status=%q subscription=%q expire=%v",
			merchant.Status, merchant.SubscriptionStatus, merchant.SubscriptionExpireAt)
	}
	var store model.Store
	if err := db.Where("merchant_id = ? AND name = ?", merchant.ID, demoCafeStoreName).First(&store).Error; err != nil {
		t.Fatalf("load demo store on MySQL: %v", err)
	}
	if store.OrderMode != "submit_later" {
		t.Fatalf("demo store order_mode=%q, want submit_later", store.OrderMode)
	}

	cfg := &config.Config{AppEnv: "test", AIEnabled: false}
	merchantSvc := service.NewMerchantService(cfg, db)
	today := time.Now().In(demoBeijingLocation).Format("2006-01-02")

	// Daily summary for today: 3 paid orders, trade 13600, refund 3800,
	// net 9800, 3 new customers; channels table 5800 + store_qr 7800.
	summary, err := merchantSvc.OpsDailySummary(merchant.ID, today)
	if err != nil {
		t.Fatalf("daily summary on MySQL: %v", err)
	}
	if summary.TradeAmount != 13600 || summary.OrderCount != 3 || summary.RefundAmount != 3800 ||
		summary.NetAmount != 9800 || summary.NewCustomers != 3 {
		t.Errorf("daily summary %+v, want trade 13600 / orders 3 / refund 3800 / net 9800 / new 3", summary)
	}
	channelByName := map[string]int64{}
	for _, row := range summary.ByChannel {
		channelByName[row.Channel] = row.AmountCents
	}
	if channelByName["store_qr"] != 7800 || channelByName["table"] != 5800 {
		t.Errorf("daily channels %+v, want store_qr 7800 / table 5800", channelByName)
	}
	if summary.ManualRedemptionCount != 3 || summary.ManualRedemptionAmountCents != 15000 {
		t.Errorf("daily manual redemption count=%d amount=%d, want 3/15000",
			summary.ManualRedemptionCount, summary.ManualRedemptionAmountCents)
	}

	// Product health (30 days): 15 rows — 3 bestseller, 7 slow,
	// 2 normal, 3 unsold.
	health, err := merchantSvc.OpsProductHealth(merchant.ID, 30)
	if err != nil {
		t.Fatalf("product health on MySQL: %v", err)
	}
	tiers := map[string]int{}
	for _, row := range health.Rows {
		tiers[row.Status]++
	}
	if len(health.Rows) != 15 || tiers["bestseller"] != 3 || tiers["slow"] != 7 || tiers["normal"] != 2 || tiers["unsold"] != 3 {
		t.Errorf("product health rows=%d tiers=%v, want 15 rows / 3 bestseller / 7 slow / 2 normal / 3 unsold",
			len(health.Rows), tiers)
	}

	// Dormant tiers: exactly one customer per tier, phones masked.
	dormant, err := merchantSvc.OpsDormantCustomers(merchant.ID)
	if err != nil {
		t.Fatalf("dormant customers on MySQL: %v", err)
	}
	for _, tier := range dormant.Tiers {
		if tier.Count != 1 {
			t.Errorf("dormant tier %s count=%d, want 1", tier.Tier, tier.Count)
		}
		if len(tier.SamplePhones) != 1 || !strings.HasPrefix(tier.SamplePhones[0], "138****") {
			t.Errorf("dormant tier %s sample phones=%v, want one masked 138**** number", tier.Tier, tier.SamplePhones)
		}
	}

	// Postmortem snapshot read through the service: 12 orders, 58500,
	// 12 new customers against targets 15 / 80000.
	adminSvc := service.NewAdminService(cfg, db)
	postmortems, err := adminSvc.ListCampaignPostmortems(merchant.ID, "", 20, 0)
	if err != nil {
		t.Fatalf("list postmortems on MySQL: %v", err)
	}
	if len(postmortems) != 1 {
		t.Fatalf("got %d postmortems, want 1", len(postmortems))
	}
	pm := postmortems[0]
	if pm.ActualOrders != 12 || pm.ActualAmountCents != 58500 || pm.ActualNewCustomers != 12 ||
		pm.TargetOrders != 15 || pm.TargetAmountCents != 80000 {
		t.Errorf("postmortem actuals orders=%d amount=%d new=%d targets=%d/%d, want 12/58500/12 and 15/80000",
			pm.ActualOrders, pm.ActualAmountCents, pm.ActualNewCustomers, pm.TargetOrders, pm.TargetAmountCents)
	}

	// Live order against the demo merchant: create (submitted) -> mark
	// paid (payment_confirming) -> merchant confirms (paid + profile).
	var product model.StoreProduct
	if err := db.Where("store_id = ? AND name = ?", store.ID, "美式咖啡").First(&product).Error; err != nil {
		t.Fatalf("load demo product on MySQL: %v", err)
	}
	orderSvc := service.NewOrderService(db)
	created, err := orderSvc.CreateCustomerStoreOrder(dto.CreateCustomerOrderRequest{
		StoreID: store.ID, CustomerPhone: "13800000199", CustomerNote: "演示验证：现场下单",
		PaymentChannel: "alipay", PayMode: "page",
		RequestKey: strings.Repeat("a1", 32), AccessToken: strings.Repeat("b2", 32),
		Items: []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 1, Price: product.Price}},
	})
	if err != nil {
		t.Fatalf("live customer order on demo merchant: %v", err)
	}
	if created.Status != "submitted" || created.TotalAmount != 1800 {
		t.Fatalf("live order status=%q total=%d, want submitted/1800", created.Status, created.TotalAmount)
	}
	marked, err := orderSvc.MarkCustomerStoreOrderPaid(created.OrderNo, strings.Repeat("b2", 32))
	if err != nil {
		t.Fatalf("mark live order paid: %v", err)
	}
	if marked.Status != "payment_confirming" {
		t.Fatalf("marked order status=%q, want payment_confirming", marked.Status)
	}
	confirmed, err := orderSvc.ConfirmMerchantOrderPayment(merchant.ID, created.ID)
	if err != nil {
		t.Fatalf("merchant confirms live order payment: %v", err)
	}
	if confirmed.PaidAt == nil {
		t.Fatalf("confirmed live order has no paid_at")
	}
	var liveProfile model.CustomerProfile
	if err := db.Where("merchant_id = ? AND customer_phone = ?", merchant.ID, "13800000199").First(&liveProfile).Error; err != nil {
		t.Fatalf("live order did not build a customer profile: %v", err)
	}
	if liveProfile.OrderCount != 1 || liveProfile.TotalAmountCents != 1800 {
		t.Errorf("live profile count=%d total=%d, want 1/1800", liveProfile.OrderCount, liveProfile.TotalAmountCents)
	}
	// The dashboard now reflects the live order too.
	summaryAfter, err := merchantSvc.OpsDailySummary(merchant.ID, today)
	if err != nil {
		t.Fatalf("daily summary after live order: %v", err)
	}
	if summaryAfter.TradeAmount != 15400 || summaryAfter.OrderCount != 4 || summaryAfter.NewCustomers != 4 {
		t.Errorf("daily summary after live order trade=%d orders=%d new=%d, want 15400/4/4",
			summaryAfter.TradeAmount, summaryAfter.OrderCount, summaryAfter.NewCustomers)
	}
}
