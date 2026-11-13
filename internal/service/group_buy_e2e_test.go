package service

import (
	"errors"
	"testing"
	"time"

	"go-web-gin-health/internal/model"
)

// This synthetic integration test exercises the merchant-to-admin review loop
// against SQLite only. It never contacts a platform, payment provider, or other
// external service.
func TestSyntheticMerchantSubmitToAdminReviewAuditLoop(t *testing.T) {
	db := newGroupBuyTestDB(t)
	autoMerchant, autoStore := createGroupBuyTestMerchant(t, db, "合成自动通过商家")
	rejectMerchant, rejectStore := createGroupBuyTestMerchant(t, db, "合成自动拒绝商家")
	manualApproveMerchant, manualApproveStore := createGroupBuyTestMerchant(t, db, "合成人工通过商家")
	manualRejectMerchant, manualRejectStore := createGroupBuyTestMerchant(t, db, "合成人工拒绝商家")
	merchantService := NewMerchantGroupBuyService(db)
	adminService := NewPlatformGroupBuyService(db)

	autoApproved, err := merchantService.Save(
		autoMerchant.ID, autoStore.ID, GroupBuyChannelDouyin,
		"https://life.douyin.com/deal/synthetic-approved", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	autoRejected, err := merchantService.Save(
		rejectMerchant.ID, rejectStore.ID, GroupBuyChannelDouyin,
		"https://coupon-synthetic.invalid/deal/rejected", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	manualApproved, err := merchantService.Save(
		manualApproveMerchant.ID, manualApproveStore.ID, GroupBuyChannelDouyin,
		"https://shop.douyin.com/deal/synthetic-review", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	manualRejected, err := merchantService.Save(
		manualRejectMerchant.ID, manualRejectStore.ID, GroupBuyChannelDouyin,
		"https://v.douyin.com/synthetic-short", true,
	)
	if err != nil {
		t.Fatal(err)
	}

	if autoApproved.ReviewStatus != "approved" || autoApproved.ReviewMethod != "auto" || autoApproved.ReviewRuleID != GroupBuyRuleAutoApproved {
		t.Fatalf("synthetic trusted destination should auto-pass, got %#v", autoApproved)
	}
	if autoRejected.ReviewStatus != "rejected" || autoRejected.ReviewMethod != "auto" || autoRejected.ReviewRuleID != GroupBuyRuleUnknownHost {
		t.Fatalf("synthetic unknown destination should auto-reject, got %#v", autoRejected)
	}
	if manualApproved.ReviewStatus != "pending" || manualApproved.ReviewMethod != "manual" || manualApproved.ReviewRuleID != GroupBuyRuleAmbiguousHost {
		t.Fatalf("synthetic ambiguous destination should enter manual queue, got %#v", manualApproved)
	}
	if manualRejected.ReviewStatus != "pending" || manualRejected.ReviewMethod != "manual" || manualRejected.ReviewRuleID != GroupBuyRuleShortOrRootLink {
		t.Fatalf("synthetic short link should enter manual queue, got %#v", manualRejected)
	}

	if _, err := merchantService.ResolveAndTrack(manualApproved.Code); !errors.Is(err, ErrGroupBuyLinkNotFound) {
		t.Fatalf("pending item must not redirect before admin review, got %v", err)
	}
	queue, queueTotal, _, err := adminService.List(PlatformGroupBuyFilter{ReviewStatus: "pending", ReviewMethod: "manual"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if queueTotal != 2 || len(queue) != 2 {
		t.Fatalf("admin manual queue should contain two synthetic exceptions, total=%d rows=%d", queueTotal, len(queue))
	}

	if _, err := adminService.ApplyAdminAction(manualApproved.ID, 9001, "admin", "approve", "合成验证：模拟后台核对通过"); err != nil {
		t.Fatal(err)
	}
	if _, err := adminService.ApplyAdminAction(manualRejected.ID, 9001, "admin", "reject", "合成验证：短链落地页未核实"); err != nil {
		t.Fatal(err)
	}
	if _, err := adminService.ApplyAdminAction(autoRejected.ID, 9001, "admin", "approve", "不得绕过域名安全规则"); err == nil {
		t.Fatal("admin must not override an automatically rejected untrusted domain")
	}

	if destination, err := merchantService.ResolveAndTrack(autoApproved.Code); err != nil || destination != autoApproved.DestinationURL {
		t.Fatalf("auto-approved link should resolve, destination=%q err=%v", destination, err)
	}
	if destination, err := merchantService.ResolveAndTrack(manualApproved.Code); err != nil || destination != manualApproved.DestinationURL {
		t.Fatalf("manually approved link should resolve, destination=%q err=%v", destination, err)
	}
	for _, link := range []*model.MerchantGroupBuyLink{autoRejected, manualRejected} {
		if _, err := merchantService.ResolveAndTrack(link.Code); !errors.Is(err, ErrGroupBuyLinkNotFound) {
			t.Fatalf("rejected link %d must not redirect, got %v", link.ID, err)
		}
	}

	approvedDetail, err := adminService.GetDetail(manualApproved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if approvedDetail.Link.ReviewStatus != "approved" || approvedDetail.Link.ReviewMethod != "manual" || len(approvedDetail.History) != 3 {
		t.Fatalf("manual approval state/history mismatch: link=%#v history=%#v", approvedDetail.Link, approvedDetail.History)
	}
	approval := approvedDetail.History[0]
	if approval.Action != "platform_approve" || approval.ActorType != "platform_admin" || approval.ActorID != 9001 || approval.Reason != "合成验证：模拟后台核对通过" || approval.RuleID != GroupBuyRuleAdminDecision || approval.BeforeState == "" || approval.AfterState == "" {
		t.Fatalf("manual approval audit is missing actor, rule, reason, or state snapshots: %#v", approval)
	}
	rejectedDetail, err := adminService.GetDetail(manualRejected.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rejectedDetail.Link.ReviewStatus != "rejected" || rejectedDetail.History[0].Action != "platform_reject" || rejectedDetail.History[0].Reason != "合成验证：短链落地页未核实" {
		t.Fatalf("manual rejection should remain visible in audit history: %#v", rejectedDetail)
	}

	_, total, summary, err := adminService.List(PlatformGroupBuyFilter{}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || summary.TotalLinks != 4 || summary.AutoApprovedCount != 1 || summary.AutoRejectedCount != 1 || summary.PendingCount != 0 || summary.ClickTotal != 2 {
		t.Fatalf("post-review summary should reflect final synthetic states and two allowed redirects: total=%d summary=%#v", total, summary)
	}
}

func TestGroupBuyRedirectStaysLiveAfterExpiryButStopsForSafetyPause(t *testing.T) {
	db := newGroupBuyTestDB(t)
	merchantService := NewMerchantGroupBuyService(db)

	expiredMerchant, expiredStore := createGroupBuyTestMerchant(t, db, "合成普通到期团购商家")
	past := time.Now().Add(-24 * time.Hour)
	if err := db.Model(&model.Merchant{}).Where("id = ?", expiredMerchant.ID).Updates(map[string]interface{}{
		"subscription_status":    "active",
		"subscription_expire_at": past,
	}).Error; err != nil {
		t.Fatalf("expire synthetic merchant subscription: %v", err)
	}
	expiredLink, err := merchantService.Save(expiredMerchant.ID, expiredStore.ID, GroupBuyChannelDouyin, "https://life.douyin.com/deal/expired-but-live", true)
	if err != nil {
		t.Fatalf("save approved link for expired synthetic merchant: %v", err)
	}
	if destination, err := merchantService.ResolveAndTrack(expiredLink.Code); err != nil || destination != expiredLink.DestinationURL {
		t.Fatalf("ordinary expiry should keep the approved public link live: destination=%q err=%v", destination, err)
	}

	pausedMerchant, pausedStore := createGroupBuyTestMerchant(t, db, "合成安全暂停团购商家")
	pausedLink, err := merchantService.Save(pausedMerchant.ID, pausedStore.ID, GroupBuyChannelDouyin, "https://life.douyin.com/deal/paused-blocked", true)
	if err != nil {
		t.Fatalf("save approved link before synthetic safety pause: %v", err)
	}
	if err := db.Model(&model.Merchant{}).Where("id = ?", pausedMerchant.ID).Update("subscription_status", "paused").Error; err != nil {
		t.Fatalf("pause synthetic merchant subscription: %v", err)
	}
	if _, err := merchantService.ResolveAndTrack(pausedLink.Code); !errors.Is(err, ErrGroupBuyLinkNotFound) {
		t.Fatalf("safety-paused merchant must not redirect, got %v", err)
	}
	var storedPausedLink model.MerchantGroupBuyLink
	if err := db.First(&storedPausedLink, pausedLink.ID).Error; err != nil {
		t.Fatalf("reload synthetic paused link: %v", err)
	}
	if storedPausedLink.ClickCount != 0 || storedPausedLink.LastClickedAt != nil {
		t.Fatalf("blocked paused redirect still recorded a click: count=%d last=%v", storedPausedLink.ClickCount, storedPausedLink.LastClickedAt)
	}
}
