package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func newMerchantOnboardingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Merchant{},
		&model.User{},
		&model.VerificationCode{},
		&model.AuditLog{},
		&model.Store{},
		&model.StoreProduct{},
		&model.Promotion{},
		&model.MerchantPlan{},
		&model.MembershipPackage{},
		&model.Order{},
		&model.CardCode{},
	); err != nil {
		t.Fatalf("migrate onboarding test schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func newMerchantOnboardingTestConfig() *config.Config {
	return &config.Config{
		DevPasswordResetEnabled: true,
		JWTSecret:               strings.Repeat("k", 32),
	}
}

func TestRegisterCreatesPendingMerchantWithSubmittedAudit(t *testing.T) {
	db := newMerchantOnboardingTestDB(t)
	service := NewMerchantService(newMerchantOnboardingTestConfig(), db)
	if err := service.SendRegisterCode(dto.SendSMSCodeRequest{Phone: "13800000001"}); err != nil {
		t.Fatalf("send register code: %v", err)
	}
	merchant, user, token, err := service.Register(dto.MerchantRegisterRequest{
		Name: "入驻审核测试商家", ContactPhone: "13800000001", SMSCode: "123456",
		Password: "test123456", ConfirmPassword: "test123456",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if token == "" {
		t.Fatal("register returned an empty token")
	}
	if merchant.Status != "pending" {
		t.Fatalf("registered merchant status=%q, want pending", merchant.Status)
	}
	if user.MerchantID == nil || *user.MerchantID != merchant.ID {
		t.Fatalf("registered user merchant_id=%v, want %d", user.MerchantID, merchant.ID)
	}
	var audit model.AuditLog
	if err := db.Where("action = ? AND merchant_id = ?", "merchant_onboarding_submitted", merchant.ID).First(&audit).Error; err != nil {
		t.Fatalf("onboarding submitted audit row missing: %v", err)
	}
	if audit.TargetType != "merchant" || audit.TargetID != merchant.ID {
		t.Fatalf("submitted audit target=%s/%d, want merchant/%d", audit.TargetType, audit.TargetID, merchant.ID)
	}
}

func TestReviewMerchantOnboardingApproveRejectResubmit(t *testing.T) {
	db := newMerchantOnboardingTestDB(t)
	admin := NewAdminService(nil, db)
	merchant := &model.Merchant{Name: "复核流程测试商家", Status: "pending"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create pending merchant: %v", err)
	}
	reviewer := AuditActor{ID: 7, Type: "super_admin"}
	review := func(approve bool, reason string, action string) (*model.Merchant, error) {
		return admin.ReviewMerchantOnboarding(merchant.ID, approve, reason, reviewer.ID, AuditEvent{
			Actor: reviewer,
			Input: AuditRecordInput{Action: action, TargetType: "merchant", MerchantID: &merchant.ID},
		})
	}

	// Clean pending without a rejection cannot resubmit.
	if _, err := NewMerchantService(nil, db).ResubmitOnboarding(merchant.ID); err == nil {
		t.Fatal("resubmit unexpectedly succeeded on a clean pending merchant")
	}
	// Reject requires a reason, at most 255 characters.
	if _, err := review(false, "", "merchant_onboarding_rejected"); err == nil {
		t.Fatal("reject unexpectedly succeeded without a reason")
	}
	if _, err := review(false, strings.Repeat("拒", 256), "merchant_onboarding_rejected"); err == nil {
		t.Fatal("reject unexpectedly succeeded with a 256-character reason")
	}
	rejected, err := review(false, "资料不完整，请补营业执照", "merchant_onboarding_rejected")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != "pending" || rejected.ReviewReason != "资料不完整，请补营业执照" || rejected.ReviewedBy != 7 || rejected.ReviewedAt == nil {
		t.Fatalf("rejected merchant=%+v, want pending with reason, reviewer 7, review time set", rejected)
	}
	var rejectAudit model.AuditLog
	if err := db.Where("action = ?", "merchant_onboarding_rejected").First(&rejectAudit).Error; err != nil {
		t.Fatalf("rejection audit row missing: %v", err)
	}
	if rejectAudit.TargetType != "merchant" || rejectAudit.MerchantID == nil || *rejectAudit.MerchantID != merchant.ID {
		t.Fatalf("rejection audit target=%s/%v, want merchant/%d", rejectAudit.TargetType, rejectAudit.MerchantID, merchant.ID)
	}

	// Resubmit clears the rejection back to a clean pending state.
	merchantService := NewMerchantService(nil, db)
	resubmitted, err := merchantService.ResubmitOnboarding(merchant.ID, AuditEvent{
		Actor: AuditActor{ID: 9, Type: "merchant_admin"},
		Input: AuditRecordInput{Action: "merchant_onboarding_resubmitted", TargetType: "merchant", MerchantID: &merchant.ID},
	})
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if resubmitted.Status != "pending" || resubmitted.ReviewReason != "" || resubmitted.ReviewedBy != 0 || resubmitted.ReviewedAt != nil {
		t.Fatalf("resubmitted merchant=%+v, want clean pending", resubmitted)
	}
	var resubmitAudit model.AuditLog
	if err := db.Where("action = ?", "merchant_onboarding_resubmitted").First(&resubmitAudit).Error; err != nil {
		t.Fatalf("resubmission audit row missing: %v", err)
	}

	// Approving activates the merchant and clears any earlier rejection.
	approved, err := review(true, "", "merchant_onboarding_approved")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != "active" || approved.ReviewReason != "" || approved.ReviewedBy != 7 || approved.ReviewedAt == nil {
		t.Fatalf("approved merchant=%+v, want active with reviewer 7 and review time set", approved)
	}
	// An active merchant is no longer reviewable or resubmittable.
	if _, err := review(true, "", "merchant_onboarding_approved"); err == nil {
		t.Fatal("review unexpectedly succeeded on an active merchant")
	}
	if _, err := merchantService.ResubmitOnboarding(merchant.ID); err == nil {
		t.Fatal("resubmit unexpectedly succeeded on an active merchant")
	}
}

func TestUpdateMerchantStatusCannotActivatePendingMerchant(t *testing.T) {
	db := newMerchantOnboardingTestDB(t)
	admin := NewAdminService(nil, db)
	actor := AuditActor{ID: 7, Type: "super_admin"}
	update := func(id uint, status string) error {
		return admin.UpdateMerchantStatus(id, status, AuditEvent{
			Actor: actor,
			Input: AuditRecordInput{Action: "merchant_status_update", TargetType: "merchant", MerchantID: &id},
		})
	}

	// pending→active through the generic endpoint is rejected: no write,
	// no audit row. The review API remains the only activation path.
	pending := &model.Merchant{Name: "旁路拦截测试商家", Status: "pending"}
	if err := db.Create(pending).Error; err != nil {
		t.Fatalf("create pending merchant: %v", err)
	}
	if err := update(pending.ID, "active"); err == nil {
		t.Fatal("generic status update unexpectedly activated a pending merchant")
	} else if err.Error() != "待审核商家须通过入驻审核接口开通，不能直接改状态" {
		t.Fatalf("pending→active err=%v, want onboarding-review message", err)
	}
	var after model.Merchant
	if err := db.First(&after, pending.ID).Error; err != nil {
		t.Fatalf("reload pending merchant: %v", err)
	}
	if after.Status != "pending" {
		t.Fatalf("pending merchant status=%q after rejected update, want pending", after.Status)
	}
	var auditCount int64
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "merchant_status_update").Count(&auditCount).Error; err != nil {
		t.Fatalf("count status-update audit rows: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("rejected status update wrote %d audit rows, want 0", auditCount)
	}
	// The review API still activates the same merchant.
	if _, err := admin.ReviewMerchantOnboarding(pending.ID, true, "", actor.ID, AuditEvent{
		Actor: actor,
		Input: AuditRecordInput{Action: "merchant_onboarding_approved", TargetType: "merchant", MerchantID: &pending.ID},
	}); err != nil {
		t.Fatalf("review approve after rejected generic update: %v", err)
	}

	// Other transitions are unchanged: suspended→active (manual unfreeze),
	// active→suspended, and pending→suspended all still succeed.
	suspended := &model.Merchant{Name: "解冻测试商家", Status: "suspended"}
	if err := db.Create(suspended).Error; err != nil {
		t.Fatalf("create suspended merchant: %v", err)
	}
	if err := update(suspended.ID, "active"); err != nil {
		t.Fatalf("suspended→active: %v", err)
	}
	if err := update(suspended.ID, "suspended"); err != nil {
		t.Fatalf("active→suspended: %v", err)
	}
	pending2 := &model.Merchant{Name: "待审核禁用测试商家", Status: "pending"}
	if err := db.Create(pending2).Error; err != nil {
		t.Fatalf("create second pending merchant: %v", err)
	}
	if err := update(pending2.ID, "suspended"); err != nil {
		t.Fatalf("pending→suspended: %v", err)
	}
	var after2 model.Merchant
	if err := db.First(&after2, pending2.ID).Error; err != nil {
		t.Fatalf("reload second pending merchant: %v", err)
	}
	if after2.Status != "suspended" {
		t.Fatalf("second pending merchant status=%q, want suspended", after2.Status)
	}
}

func TestPendingMerchantCannotBuildOrBuy(t *testing.T) {
	db := newMerchantOnboardingTestDB(t)
	service := NewMerchantService(nil, db)
	pending := &model.Merchant{Name: "待审核门禁商家", Status: "pending"}
	if err := db.Create(pending).Error; err != nil {
		t.Fatalf("create pending merchant: %v", err)
	}

	if _, err := service.CreateStore(pending.ID, dto.CreateStoreRequest{Name: "待审核门店", ContactPhone: "13800000001"}); err == nil {
		t.Fatal("pending merchant unexpectedly created a store")
	}
	if _, err := service.SaveStoreProduct(pending.ID, 0, dto.SaveStoreProductRequest{StoreID: 1, Name: "待审核商品", Price: 100}); err == nil {
		t.Fatal("pending merchant unexpectedly saved a product")
	}
	if _, err := service.SavePromotion(pending.ID, 0, dto.SavePromotionRequest{Title: "待审核活动", Type: "amount", Status: "draft", Threshold: 100, Discount: 10}); err == nil {
		t.Fatal("pending merchant unexpectedly saved a promotion")
	}
	if _, err := service.CreateSubscriptionOrder(0, pending.ID, dto.CreateMerchantSubscriptionOrderRequest{PlanID: 1}); err == nil {
		t.Fatal("pending merchant unexpectedly created a subscription order")
	}
	pkg := &model.MembershipPackage{Name: "测试卡密套餐", Code: "onboarding-card-pkg", DurationDays: 30, Price: 1000, Status: "published"}
	if err := db.Create(pkg).Error; err != nil {
		t.Fatalf("create card package: %v", err)
	}
	card := &model.CardCode{BatchNo: "ONB-1", Code: "ONB-CARD-1", PackageID: pkg.ID, Status: "unused"}
	if err := db.Create(card).Error; err != nil {
		t.Fatalf("create card code: %v", err)
	}
	if _, _, err := service.RedeemSubscriptionCard(0, pending.ID, dto.MerchantRedeemCardRequest{Code: "ONB-CARD-1"}); err == nil {
		t.Fatal("pending merchant unexpectedly redeemed a subscription card")
	}
	// A suspended merchant cannot redeem either (P3-2 fix A): same gate as
	// CreateSubscriptionOrder, which already rejects suspended merchants.
	suspended := &model.Merchant{Name: "已禁用门禁商家", Status: "suspended"}
	if err := db.Create(suspended).Error; err != nil {
		t.Fatalf("create suspended merchant: %v", err)
	}
	if _, _, err := service.RedeemSubscriptionCard(0, suspended.ID, dto.MerchantRedeemCardRequest{Code: "ONB-CARD-1"}); err == nil || err.Error() != "商家已被禁用" {
		t.Fatalf("suspended merchant card redeem err=%v, want 商家已被禁用", err)
	}
	if _, err := service.CreateSubscriptionOrder(0, suspended.ID, dto.CreateMerchantSubscriptionOrderRequest{PlanID: 1}); err == nil || err.Error() != "商家已被禁用" {
		t.Fatalf("suspended merchant subscription order err=%v, want 商家已被禁用", err)
	}

	// The same calls succeed for an active merchant.
	active := &model.Merchant{Name: "已通过门禁商家", Status: "active"}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create active merchant: %v", err)
	}
	plan := &model.MerchantPlan{Name: "月付", PriceCents: 19900, DurationDays: 30}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create merchant plan: %v", err)
	}
	fallback := &model.MembershipPackage{Name: "测试占位套餐", Code: "onboarding-fallback", DurationDays: 30, Price: 1000, Status: "published"}
	if err := db.Create(fallback).Error; err != nil {
		t.Fatalf("create fallback package: %v", err)
	}
	if _, err := service.CreateSubscriptionOrder(0, active.ID, dto.CreateMerchantSubscriptionOrderRequest{PlanID: plan.ID}); err != nil {
		t.Fatalf("active merchant subscription order: %v", err)
	}
	if _, _, err := service.RedeemSubscriptionCard(0, active.ID, dto.MerchantRedeemCardRequest{Code: "ONB-CARD-1"}); err != nil {
		t.Fatalf("active merchant card redeem: %v", err)
	}
	if _, err := service.CreateStore(active.ID, dto.CreateStoreRequest{Name: "已通过门店", ContactPhone: "13800000001"}); err != nil {
		t.Fatalf("active merchant create store: %v", err)
	}
}

func TestMerchantLoginByCodeAndUnregisteredPhoneHint(t *testing.T) {
	db := newMerchantOnboardingTestDB(t)
	service := NewMerchantService(newMerchantOnboardingTestConfig(), db)

	// Seed a merchant + merchant_admin user directly: going through the
	// register flow would issue a register code for the same phone, and the
	// shared SMS policy rate-limits any second code within one minute.
	hash, err := utils.HashPassword("login123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	merchant := &model.Merchant{Name: "登录方式测试商家", Status: "pending"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	user := &model.User{
		UUID: uuid.NewString(), Email: "login-test@pending.local",
		Phone: "13800000002", MerchantID: &merchant.ID,
		Role: "merchant_admin", PasswordHash: hash,
		DisplayName: "登录方式测试商家", Status: "active",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create merchant user: %v", err)
	}

	// Password login still works without login_type (backward compatible).
	if _, _, token, err := service.Login(dto.MerchantLoginRequest{Phone: "13800000002", Password: "login123"}); err != nil {
		t.Fatalf("password login: %v", err)
	} else if token == "" {
		t.Fatal("password login returned an empty token")
	}

	// Wrong password keeps the generic message.
	if _, _, _, err := service.Login(dto.MerchantLoginRequest{Phone: "13800000002", Password: "wrong123"}); err == nil || err.Error() != "账号或密码错误" {
		t.Fatalf("wrong password err=%v, want 账号或密码错误", err)
	}

	// An unregistered phone gets a distinct message the UI can act on.
	if _, _, _, err := service.Login(dto.MerchantLoginRequest{Phone: "13800000003", Password: "login123"}); err == nil || err.Error() != "该手机号未注册，请先注册" {
		t.Fatalf("unregistered phone err=%v, want 该手机号未注册，请先注册", err)
	}

	// Unknown login_type is rejected instead of silently falling back.
	if _, _, _, err := service.Login(dto.MerchantLoginRequest{Phone: "13800000002", Password: "login123", LoginType: "wechat"}); err == nil || err.Error() != "不支持的登录方式" {
		t.Fatalf("unknown login_type err=%v, want 不支持的登录方式", err)
	}

	// Send-code for an unregistered phone reports no merchant account.
	if err := service.SendLoginCode(dto.MerchantSendLoginCodeRequest{Phone: "13800000003"}); err == nil || err.Error() != "商家账号不存在" {
		t.Fatalf("send login code to unregistered phone err=%v, want 商家账号不存在", err)
	}

	// Code login works with the development fixed code; a wrong code fails
	// first without consuming the issued code.
	if err := service.SendLoginCode(dto.MerchantSendLoginCodeRequest{Phone: "13800000002"}); err != nil {
		t.Fatalf("send login code: %v", err)
	}
	if _, _, _, err := service.Login(dto.MerchantLoginRequest{Phone: "13800000002", LoginType: "code", SMSCode: "000000"}); err == nil {
		t.Fatal("wrong login code unexpectedly succeeded")
	}
	if _, _, token, err := service.Login(dto.MerchantLoginRequest{Phone: "13800000002", LoginType: "code", SMSCode: "123456"}); err != nil {
		t.Fatalf("code login: %v", err)
	} else if token == "" {
		t.Fatal("code login returned an empty token")
	}

	// A consumed code cannot be reused.
	if _, _, _, err := service.Login(dto.MerchantLoginRequest{Phone: "13800000002", LoginType: "code", SMSCode: "123456"}); err == nil {
		t.Fatal("reused login code unexpectedly succeeded")
	}
}
