package service

import (
	"testing"

	"go-web-gin-health/internal/model"
)

func TestEvaluateGroupBuyReviewSyntheticMatrix(t *testing.T) {
	cases := []struct {
		name             string
		channel          string
		url              string
		merchantStatus   string
		storeStatus      string
		previouslyPassed bool
		wantStatus       string
		wantMethod       string
		wantRule         string
	}{
		{"抖音精确白名单自动通过", GroupBuyChannelDouyin, "https://life.douyin.com/deal/demo", "active", "active", false, "approved", "auto", GroupBuyRuleAutoApproved},
		{"美团精确白名单自动通过", GroupBuyChannelMeituan, "https://i.meituan.com/deal/demo", "active", "active", false, "approved", "auto", GroupBuyRuleAutoApproved},
		{"大众点评精确白名单自动通过", GroupBuyChannelMeituan, "https://www.dianping.com/shop/demo", "active", "active", false, "approved", "auto", GroupBuyRuleAutoApproved},
		{"平台域名家族内的未登记子域进入人工队列", GroupBuyChannelDouyin, "https://shop.douyin.com/deal/demo", "active", "active", false, "pending", "manual", GroupBuyRuleAmbiguousHost},
		{"抖音短链进入人工队列", GroupBuyChannelDouyin, "https://v.douyin.com/demo", "active", "active", false, "pending", "manual", GroupBuyRuleShortOrRootLink},
		{"平台根页进入人工队列", GroupBuyChannelDouyin, "https://life.douyin.com/", "active", "active", false, "pending", "manual", GroupBuyRuleShortOrRootLink},
		{"选择平台与另一平台域名不符", GroupBuyChannelDouyin, "https://i.meituan.com/deal/demo", "active", "active", false, "rejected", "auto", GroupBuyRuleWrongChannel},
		{"仿冒域名硬拒绝", GroupBuyChannelDouyin, "https://life.douyin.com.attacker.example/deal", "active", "active", false, "rejected", "auto", GroupBuyRuleLookalikeHost},
		{"未知外部域名硬拒绝", GroupBuyChannelMeituan, "https://coupons.example/deal", "active", "active", false, "rejected", "auto", GroupBuyRuleUnknownHost},
		{"HTTP 链接硬拒绝", GroupBuyChannelDouyin, "http://life.douyin.com/deal/demo", "active", "active", false, "rejected", "auto", GroupBuyRuleURLUnsafe},
		{"含凭据链接硬拒绝", GroupBuyChannelDouyin, "https://user:secret@life.douyin.com/deal/demo", "active", "active", false, "rejected", "auto", GroupBuyRuleURLUnsafe},
		{"显式 443 端口硬拒绝", GroupBuyChannelDouyin, "https://life.douyin.com:443/deal/demo", "active", "active", false, "rejected", "auto", GroupBuyRuleURLUnsafe},
		{"IP 地址硬拒绝", GroupBuyChannelDouyin, "https://127.0.0.1/deal/demo", "active", "active", false, "rejected", "auto", GroupBuyRuleURLUnsafe},
		{"畸形 URL 硬拒绝", GroupBuyChannelDouyin, "https://%zz", "active", "active", false, "rejected", "auto", GroupBuyRuleURLUnsafe},
		{"商家停用进入人工队列", GroupBuyChannelDouyin, "https://life.douyin.com/deal/demo", "suspended", "active", false, "pending", "manual", GroupBuyRuleMerchantState},
		{"门店停用进入人工队列", GroupBuyChannelDouyin, "https://life.douyin.com/deal/demo", "active", "disabled", false, "pending", "manual", GroupBuyRuleMerchantState},
		{"已通过入口改链转人工队列", GroupBuyChannelDouyin, "https://life.douyin.com/deal/new", "active", "active", true, "pending", "manual", GroupBuyRuleChangedAfterPass},
		{"未知渠道拒绝", "other", "https://life.douyin.com/deal/demo", "active", "active", false, "rejected", "auto", GroupBuyRuleUnsupportedChannel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateGroupBuyReview(tc.channel, tc.url, tc.merchantStatus, tc.storeStatus, tc.previouslyPassed)
			if got.Status != tc.wantStatus || got.Method != tc.wantMethod || got.RuleID != tc.wantRule {
				t.Fatalf("decision=%+v, want status=%q method=%q rule=%q", got, tc.wantStatus, tc.wantMethod, tc.wantRule)
			}
			if got.Reason == "" {
				t.Fatal("every decision should include an explainable reason")
			}
		})
	}
}

func TestGroupBuySaveAutomaticallyRoutesAndAudits(t *testing.T) {
	db := newGroupBuyTestDB(t)
	merchant, store := createGroupBuyTestMerchant(t, db, "自动审核合成商家")
	merchantLinks := NewMerchantGroupBuyService(db)

	auto, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelDouyin, "https://life.douyin.com/deal/first", true)
	if err != nil {
		t.Fatal(err)
	}
	if auto.ReviewStatus != "approved" || auto.ReviewMethod != "auto" || auto.ReviewRuleID != GroupBuyRuleAutoApproved || auto.ReviewedAt == nil {
		t.Fatalf("exact trusted link should be auto-approved with metadata: %#v", auto)
	}
	if _, err := merchantLinks.ResolveAndTrack(auto.Code); err != nil {
		t.Fatalf("auto-approved active link should be redirectable: %v", err)
	}

	ambiguous, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelMeituan, "https://shop.meituan.com/deal/ambiguous", true)
	if err != nil {
		t.Fatal(err)
	}
	if ambiguous.ReviewStatus != "pending" || ambiguous.ReviewMethod != "manual" || ambiguous.ReviewRuleID != GroupBuyRuleAmbiguousHost || ambiguous.ReviewedAt != nil {
		t.Fatalf("ambiguous in-family domain should enter manual queue: %#v", ambiguous)
	}
	if _, err := merchantLinks.ResolveAndTrack(ambiguous.Code); err == nil {
		t.Fatal("manual review item must not redirect")
	}

	wrongChannel, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelDouyin, "https://i.meituan.com/deal/wrong-channel", true)
	if err != nil {
		t.Fatal(err)
	}
	if wrongChannel.ReviewStatus != "rejected" || wrongChannel.ReviewMethod != "auto" || wrongChannel.ReviewRuleID != GroupBuyRuleWrongChannel || wrongChannel.ReviewedAt == nil {
		t.Fatalf("wrong-channel input should be persisted as a rejected synthetic entry: %#v", wrongChannel)
	}
	if _, err := merchantLinks.ResolveAndTrack(wrongChannel.Code); err == nil {
		t.Fatal("auto-rejected wrong-channel item must not redirect")
	}

	var systemAudit model.MerchantGroupBuyLinkAudit
	if err := db.Where("link_id = ? AND actor_type = ?", auto.ID, "system").First(&systemAudit).Error; err != nil {
		t.Fatal(err)
	}
	if systemAudit.RuleID != GroupBuyRuleAutoApproved || systemAudit.ActorName == "" || systemAudit.Reason == "" || systemAudit.AfterState == "" {
		t.Fatalf("automatic decision audit must record rule, actor, reason and after state: %#v", systemAudit)
	}
}

func TestApprovedDestinationChangeRequiresManualReasonAndCannotBypassSafety(t *testing.T) {
	db := newGroupBuyTestDB(t)
	merchant, store := createGroupBuyTestMerchant(t, db, "改链复核合成商家")
	merchantLinks := NewMerchantGroupBuyService(db)
	platformLinks := NewPlatformGroupBuyService(db)
	link, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelDouyin, "https://life.douyin.com/deal/old", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelDouyin, "https://life.douyin.com/deal/new", true); err != nil {
		t.Fatal(err)
	}
	var changed model.MerchantGroupBuyLink
	if err := db.First(&changed, link.ID).Error; err != nil {
		t.Fatal(err)
	}
	if changed.ReviewStatus != "pending" || changed.ReviewRuleID != GroupBuyRuleChangedAfterPass || changed.ReviewMethod != "manual" {
		t.Fatalf("approved link change must be queued, got %#v", changed)
	}
	if _, err := platformLinks.ApplyAdminAction(link.ID, 7, "admin", "approve", " "); err == nil {
		t.Fatal("manual approval must require a reason")
	}
	if _, err := platformLinks.ApplyAdminAction(link.ID, 7, "admin", "approve", "人工核验完成"); err != nil {
		t.Fatal(err)
	}
	var latest model.MerchantGroupBuyLinkAudit
	if err := db.Where("link_id = ? AND action = ?", link.ID, "platform_approve").First(&latest).Error; err != nil {
		t.Fatal(err)
	}
	if latest.RuleID != GroupBuyRuleAdminDecision || latest.ActorType != "platform_admin" || latest.Reason != "人工核验完成" {
		t.Fatalf("admin decision audit missing rule/actor/reason: %#v", latest)
	}

	unknown, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelMeituan, "https://coupons.example/unsafe", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := platformLinks.ApplyAdminAction(unknown.ID, 7, "admin", "approve", "我想放行"); err == nil {
		t.Fatal("manual action must not bypass domain-family validation")
	}
}

func TestPlatformGroupBuyListFiltersManualQueueAndSummarizesDecisions(t *testing.T) {
	db := newGroupBuyTestDB(t)
	merchant, store := createGroupBuyTestMerchant(t, db, "列表统计合成商家")
	merchantLinks := NewMerchantGroupBuyService(db)
	platformLinks := NewPlatformGroupBuyService(db)
	if _, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelDouyin, "https://life.douyin.com/deal/auto", true); err != nil {
		t.Fatal(err)
	}
	if _, err := merchantLinks.Save(merchant.ID, store.ID, GroupBuyChannelMeituan, "https://shop.meituan.com/deal/manual", true); err != nil {
		t.Fatal(err)
	}

	rows, total, summary, err := platformLinks.List(PlatformGroupBuyFilter{ReviewStatus: "pending", ReviewMethod: "manual"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ReviewRuleID != GroupBuyRuleAmbiguousHost {
		t.Fatalf("manual queue filter returned total=%d rows=%#v", total, rows)
	}
	if summary.TotalLinks != 1 || summary.PendingCount != 1 || summary.ManualReviewCount != 1 || summary.AutoApprovedCount != 0 {
		t.Fatalf("filtered summary does not match the manual queue: %#v", summary)
	}

	_, allTotal, allSummary, err := platformLinks.List(PlatformGroupBuyFilter{}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if allTotal != 2 || allSummary.TotalLinks != 2 || allSummary.ManualReviewCount != 1 || allSummary.AutoApprovedCount != 1 {
		t.Fatalf("all-link summary should separate the two decisions: total=%d summary=%#v", allTotal, allSummary)
	}
}
