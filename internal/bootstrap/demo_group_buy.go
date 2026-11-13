package bootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type demoGroupBuyEntry struct {
	merchantName string
	storeName    string
	channel      string
	url          string
	codeNumber   uint
	status       string
	active       bool
	clicks       uint64
	reason       string
}

// seedDemoGroupBuyLinks creates only clearly labeled synthetic records in
// non-production environments. It never replaces existing merchant data.
func seedDemoGroupBuyLinks(db *gorm.DB) error {
	entries := []demoGroupBuyEntry{
		{merchantName: "【演示数据】山风咖啡馆", storeName: "山风咖啡·北街店", channel: "douyin", url: "https://life.douyin.com/discount/demo-cafe", codeNumber: 1, status: "pending", active: true, clicks: 0},
		{merchantName: "【演示数据】山风咖啡馆", storeName: "山风咖啡·北街店", channel: "meituan", url: "https://i.meituan.com/deal/demo-cafe", codeNumber: 2, status: "approved", active: true, clicks: 238},
		{merchantName: "【演示数据】河岸生活馆", storeName: "河岸生活·中心店", channel: "douyin", url: "https://v.douyin.com/demo-riverside/", codeNumber: 3, status: "rejected", active: true, clicks: 0, reason: "演示：请补充可核对的门店活动页面链接。"},
		{merchantName: "【演示数据】河岸生活馆", storeName: "河岸生活·中心店", channel: "meituan", url: "https://www.dianping.com/shop/demo-riverside", codeNumber: 4, status: "approved", active: true, clicks: 17, reason: "演示：临时暂停，等待商家确认页面内容。"},
	}
	merchantIDs := make(map[string]uint)
	storeIDs := make(map[string]uint)
	for _, entry := range entries {
		if _, ok := merchantIDs[entry.merchantName]; ok {
			continue
		}
		merchant := model.Merchant{Name: entry.merchantName, Status: "active", ContactPhone: "00000000000"}
		if err := db.Where("name = ?", entry.merchantName).First(&merchant).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Create(&merchant).Error; err != nil {
				return err
			}
		}
		merchantIDs[entry.merchantName] = merchant.ID
		store := model.Store{MerchantID: merchant.ID, Name: entry.storeName, Status: "active", IsOpen: true}
		if err := db.Where("merchant_id = ? AND name = ?", merchant.ID, entry.storeName).First(&store).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Create(&store).Error; err != nil {
				return err
			}
		}
		storeIDs[entry.storeName] = store.ID
	}

	for _, entry := range entries {
		merchantID := merchantIDs[entry.merchantName]
		storeID := storeIDs[entry.storeName]
		var existing model.MerchantGroupBuyLink
		err := db.Where("merchant_id = ? AND store_id = ? AND channel = ?", merchantID, storeID, entry.channel).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := time.Now().Add(-3 * time.Hour)
		link := model.MerchantGroupBuyLink{
			MerchantID: merchantID, StoreID: storeID, Channel: entry.channel,
			DestinationURL: entry.url, Code: fmt.Sprintf("%032x", entry.codeNumber),
			Enabled: entry.active, ReviewStatus: "pending", ClickCount: 0,
			BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now},
		}
		if err := db.Create(&link).Error; err != nil {
			return err
		}
		pendingState, _ := json.Marshal(map[string]interface{}{
			"destination_url": link.DestinationURL, "review_status": "pending", "enabled": link.Enabled, "admin_suspended": false,
		})
		initial := model.MerchantGroupBuyLinkAudit{
			LinkID: link.ID, MerchantID: merchantID, StoreID: storeID, Channel: entry.channel,
			Action: "demo_seeded", ActorType: "system_demo", ActorName: "演示数据",
			BeforeState: "{}", AfterState: string(pendingState),
			BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now},
		}
		if err := db.Create(&initial).Error; err != nil {
			return err
		}

		if entry.status != "pending" {
			before := pendingState
			link.ReviewStatus = entry.status
			link.ReviewReason = entry.reason
			if entry.status == "approved" {
				reviewedAt := now.Add(45 * time.Minute)
				link.ReviewedAt = &reviewedAt
				link.ReviewedBy = uintPointer(1)
			}
			if entry.status == "rejected" {
				reviewedAt := now.Add(45 * time.Minute)
				link.ReviewedAt = &reviewedAt
				link.ReviewedBy = uintPointer(1)
			}
			if entry.reason != "" && entry.status == "approved" {
				link.AdminSuspended = true
				link.SuspensionReason = entry.reason
				link.SuspendedAt = link.ReviewedAt
				link.SuspendedBy = uintPointer(1)
			}
			link.ClickCount = entry.clicks
			if entry.clicks > 0 {
				lastClicked := now.Add(-90 * time.Minute)
				link.LastClickedAt = &lastClicked
			}
			if err := db.Save(&link).Error; err != nil {
				return err
			}
			after, _ := json.Marshal(map[string]interface{}{
				"destination_url": link.DestinationURL, "review_status": link.ReviewStatus,
				"review_reason": link.ReviewReason, "enabled": link.Enabled,
				"admin_suspended": link.AdminSuspended, "suspension_reason": link.SuspensionReason,
			})
				action := "platform_reject"
				if entry.status == "approved" {
					action = "platform_approve"
				}
			if link.AdminSuspended {
				action = "platform_suspend"
			}
			audit := model.MerchantGroupBuyLinkAudit{
				LinkID: link.ID, MerchantID: merchantID, StoreID: storeID, Channel: entry.channel,
				Action: action, ActorType: "system_demo", ActorID: 1, ActorName: "演示平台管理员",
				Reason: entry.reason, BeforeState: string(before), AfterState: string(after),
				BaseModel: model.BaseModel{CreatedAt: now.Add(45 * time.Minute), UpdatedAt: now.Add(45 * time.Minute)},
			}
			if err := db.Create(&audit).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func uintPointer(value uint) *uint { return &value }
