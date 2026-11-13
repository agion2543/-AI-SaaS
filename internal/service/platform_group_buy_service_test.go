package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"go-web-gin-health/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newGroupBuyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&model.Merchant{}, &model.Store{}, &model.AdminUser{}, &model.MerchantGroupBuyLink{}, &model.MerchantGroupBuyLinkAudit{}); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func createGroupBuyTestMerchant(t *testing.T, db *gorm.DB, name string) (model.Merchant, model.Store) {
	t.Helper()
	merchant := model.Merchant{Name: name, Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: name + "门店", Status: "active", IsOpen: true}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create store: %v", err)
	}
	return merchant, store
}

func TestGroupBuyRedirectRequiresApprovalAndActiveStates(t *testing.T) {
	db := newGroupBuyTestDB(t)
	merchant, store := createGroupBuyTestMerchant(t, db, "测试商家")
	merchantLinks := NewMerchantGroupBuyService(db)
	adminLinks := NewPlatformGroupBuyService(db)

	link, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelDouyin, "https://shop.douyin.com/deal/first", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.ResolveAndTrack(link.Code); !errors.Is(err, ErrGroupBuyLinkNotFound) {
		t.Fatalf("pending link must not redirect, got %v", err)
	}
	if err := adminLinks.db.Model(&model.MerchantGroupBuyLink{}).Where("id = ?", link.ID).Update("click_count", 0).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := adminLinks.ApplyAdminAction(link.ID, 7, "admin", "approve", "页面可核对"); err != nil {
		t.Fatal(err)
	}
	got, err := merchantLinks.ResolveAndTrack(link.Code)
	if err != nil || got != "https://shop.douyin.com/deal/first" {
		t.Fatalf("approved active link should redirect, destination=%q err=%v", got, err)
	}

	if _, err := adminLinks.ApplyAdminAction(link.ID, 7, "admin", "suspend", "风险复核"); err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.ResolveAndTrack(link.Code); !errors.Is(err, ErrGroupBuyLinkNotFound) {
		t.Fatalf("platform-suspended link must not redirect, got %v", err)
	}
	if _, err := adminLinks.ApplyAdminAction(link.ID, 7, "admin", "resume", "复核完成"); err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.ResolveAndTrack(link.Code); err != nil {
		t.Fatalf("resumed approved link should redirect, got %v", err)
	}

	if err := db.Model(&model.Store{}).Where("id = ?", store.ID).Update("status", "suspended").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.ResolveAndTrack(link.Code); !errors.Is(err, ErrGroupBuyLinkNotFound) {
		t.Fatalf("inactive store must block redirect, got %v", err)
	}
	if err := db.Model(&model.Store{}).Where("id = ?", store.ID).Update("status", "active").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Merchant{}).Where("id = ?", merchant.ID).Update("status", "suspended").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.ResolveAndTrack(link.Code); !errors.Is(err, ErrGroupBuyLinkNotFound) {
		t.Fatalf("inactive merchant must block redirect, got %v", err)
	}
	if err := db.Model(&model.Merchant{}).Where("id = ?", merchant.ID).Update("status", "active").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantGroupBuyLink{}).Where("id = ?", link.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.ResolveAndTrack(link.Code); !errors.Is(err, ErrGroupBuyLinkNotFound) {
		t.Fatalf("merchant-disabled link must not redirect, got %v", err)
	}

	var current model.MerchantGroupBuyLink
	if err := db.First(&current, link.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.ClickCount != 2 {
		t.Fatalf("only two successful approved, active redirects should count; got %d", current.ClickCount)
	}
}

func TestMerchantTenantBoundaryAndDestinationChangeAudit(t *testing.T) {
	db := newGroupBuyTestDB(t)
	owner, ownerStore := createGroupBuyTestMerchant(t, db, "链接所属商家")
	other, _ := createGroupBuyTestMerchant(t, db, "其他商家")
	merchantLinks := NewMerchantGroupBuyService(db)
	platformLinks := NewPlatformGroupBuyService(db)

	link, err := merchantLinks.Save(owner.ID, ownerStore.ID, GroupBuyChannelMeituan, "https://i.meituan.com/deal/original", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.List(other.ID, ownerStore.ID); !errors.Is(err, ErrGroupBuyStoreAccess) {
		t.Fatalf("other tenant must not list the store links, got %v", err)
	}
	if _, err := merchantLinks.Save(other.ID, ownerStore.ID, GroupBuyChannelMeituan, "https://i.meituan.com/deal/changed", true); !errors.Is(err, ErrGroupBuyStoreAccess) {
		t.Fatalf("other tenant must not change the store link, got %v", err)
	}

	if _, err := platformLinks.ApplyAdminAction(link.ID, 9, "super_admin", "approve", "已核对"); err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.ResolveAndTrack(link.Code); err != nil {
		t.Fatalf("approved link should redirect before edit: %v", err)
	}
	updated, err := merchantLinks.Save(owner.ID, ownerStore.ID, GroupBuyChannelMeituan, "https://www.dianping.com/shop/revised", true)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ReviewStatus != "pending" || updated.ReviewedAt != nil || updated.ReviewedBy != nil {
		t.Fatalf("destination edit must clear approval and return to pending, got %#v", updated)
	}
	if _, err := merchantLinks.ResolveAndTrack(link.Code); !errors.Is(err, ErrGroupBuyLinkNotFound) {
		t.Fatalf("edited destination must wait for review, got %v", err)
	}

	detail, err := platformLinks.GetDetail(link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.History) != 5 {
		t.Fatalf("expected create, auto-review, manual approval, and URL-change audit events; got %d", len(detail.History))
	}
	if detail.History[0].Action != "system_pending" || detail.History[0].RuleID != GroupBuyRuleChangedAfterPass || detail.History[1].Action != "merchant_destination_changed" || detail.History[1].ActorType != "merchant_admin" || detail.History[1].ActorName != owner.Name {
		t.Fatalf("latest history should show the rule decision and merchant's destination change: %#v", detail.History[:2])
	}
	if detail.Link.MerchantID != owner.ID || detail.Link.Domain != "www.dianping.com" {
		t.Fatalf("admin detail should retain tenant identity and verified domain, got %#v", detail.Link)
	}
	rows, total, _, err := platformLinks.List(PlatformGroupBuyFilter{}, 1, 20)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].MerchantID != owner.ID {
		t.Fatalf("admin list should contain only the created link, rows=%#v total=%d err=%v", rows, total, err)
	}
}

func TestAdminActionValidationAndAuditAreAtomic(t *testing.T) {
	db := newGroupBuyTestDB(t)
	merchant, store := createGroupBuyTestMerchant(t, db, "操作审计商家")
	merchantLinks := NewMerchantGroupBuyService(db)
	platformLinks := NewPlatformGroupBuyService(db)
	link, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelDouyin, "https://v.douyin.com/audit/", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := platformLinks.ApplyAdminAction(link.ID, 5, "merchant_admin", "approve", ""); err == nil {
		t.Fatal("merchant role must not perform platform approval")
	}
	if _, err := platformLinks.ApplyAdminAction(link.ID, 5, "admin", "reject", " "); err == nil {
		t.Fatal("reject must require a reason")
	}
	if _, err := platformLinks.ApplyAdminAction(link.ID, 5, "admin", "reject", "不符合要求"); err != nil {
		t.Fatal(err)
	}
	if _, err := platformLinks.ApplyAdminAction(link.ID, 5, "admin", "resume", ""); err == nil {
		t.Fatal("a rejected link must not be resumed")
	}
	detail, err := platformLinks.GetDetail(link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.History) != 3 || detail.Link.ReviewStatus != "rejected" || detail.Link.ReviewReason != "不符合要求" {
		t.Fatalf("review state and immutable history mismatch: %#v", detail)
	}
	latest := detail.History[0]
	if latest.ActorType != "platform_admin" || latest.ActorID != 5 || latest.ActorName != "管理员 #5" || latest.Reason != "不符合要求" {
		t.Fatalf("audit event should identify the administrator and reason: %#v", latest)
	}
}
