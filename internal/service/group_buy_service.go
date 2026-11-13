package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

const (
	GroupBuyChannelDouyin  = "douyin"
	GroupBuyChannelMeituan = "meituan"
)

var ErrGroupBuyLinkNotFound = errors.New("团购链接不存在或已停用")
var ErrGroupBuyStoreAccess = errors.New("无权访问此门店")

// MerchantGroupBuyService manages per-store platform destinations and aggregate
// redirect counts. It intentionally does not collect request IPs or identifiers.
type MerchantGroupBuyService struct {
	db *gorm.DB
}

func NewMerchantGroupBuyService(db *gorm.DB) *MerchantGroupBuyService {
	return &MerchantGroupBuyService{db: db}
}

func (s *MerchantGroupBuyService) List(merchantID, storeID uint) ([]model.MerchantGroupBuyLink, error) {
	if _, err := s.ownedStore(merchantID, storeID); err != nil {
		return nil, err
	}
	var links []model.MerchantGroupBuyLink
	err := s.db.Where("merchant_id = ? AND store_id = ?", merchantID, storeID).
		Order("channel ASC").Find(&links).Error
	return links, err
}

func (s *MerchantGroupBuyService) Save(merchantID, storeID uint, channel, destination string, enabled bool) (*model.MerchantGroupBuyLink, error) {
	if _, err := s.ownedStore(merchantID, storeID); err != nil {
		return nil, err
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	parsed, err := parseSafeGroupBuyURL(destination)
	if err != nil {
		return nil, fmt.Errorf("%s：%w", GroupBuyRuleURLUnsafe, err)
	}
	destination = parsed.String()
	if channel != GroupBuyChannelDouyin && channel != GroupBuyChannelMeituan {
		return nil, fmt.Errorf("%s：暂不支持此团购平台", GroupBuyRuleUnsupportedChannel)
	}

	var saved model.MerchantGroupBuyLink
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var reviewState struct {
			MerchantStatus string
			StoreStatus    string
		}
		if err := tx.Table("stores").Select("merchants.status AS merchant_status, stores.status AS store_status").
			Joins("LEFT JOIN merchants ON merchants.id = stores.merchant_id").
			Where("stores.id = ? AND stores.merchant_id = ?", storeID, merchantID).Scan(&reviewState).Error; err != nil {
			return err
		}
		var link model.MerchantGroupBuyLink
		findErr := tx.Where("merchant_id = ? AND store_id = ? AND channel = ?", merchantID, storeID, channel).First(&link).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			code, codeErr := newGroupBuyCode()
			if codeErr != nil {
				return codeErr
			}
			decision := evaluateGroupBuyReview(channel, destination, reviewState.MerchantStatus, reviewState.StoreStatus, false)
			now := time.Now()
			link = model.MerchantGroupBuyLink{
				MerchantID: merchantID, StoreID: storeID, Channel: channel,
				DestinationURL: destination, Code: code, Enabled: enabled,
				ReviewStatus: decision.Status, ReviewMethod: decision.Method,
				ReviewRuleID: decision.RuleID, ReviewReason: decision.Reason,
			}
			if decision.Status != groupBuyReviewPending {
				link.ReviewedAt = &now
			}
			if err := tx.Create(&link).Error; err != nil {
				return err
			}
			if err := appendMerchantGroupBuyAudit(tx, nil, &link, "merchant_link_created", ""); err != nil {
				return err
			}
			if err := appendGroupBuyAuditWithRule(tx, nil, &link, "system_"+decision.Status, "system", 0, "团购自动审核", decision.Reason, decision.RuleID); err != nil {
				return err
			}
			saved = link
			return nil
		}
		if findErr != nil {
			return findErr
		}

		before := link
		destinationChanged := link.DestinationURL != destination
		updates := map[string]interface{}{"destination_url": destination, "enabled": enabled}
		var decision groupBuyReviewDecision
		if destinationChanged {
			decision = evaluateGroupBuyReview(channel, destination, reviewState.MerchantStatus, reviewState.StoreStatus, link.ReviewStatus == groupBuyReviewApproved)
			now := time.Now()
			var reviewedAt interface{}
			if decision.Status != groupBuyReviewPending {
				reviewedAt = &now
			}
			updates["review_status"] = decision.Status
			updates["review_method"] = decision.Method
			updates["review_rule_id"] = decision.RuleID
			updates["review_reason"] = decision.Reason
			updates["reviewed_by"] = nil
			updates["reviewed_at"] = reviewedAt
		}
		if link.Code == "" {
			link.Code, err = newGroupBuyCode()
			if err != nil {
				return err
			}
			updates["code"] = link.Code
		}
		if err := tx.Model(&model.MerchantGroupBuyLink{}).
			Where("id = ? AND merchant_id = ? AND store_id = ?", link.ID, merchantID, storeID).
			Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND merchant_id = ? AND store_id = ?", link.ID, merchantID, storeID).First(&saved).Error; err != nil {
			return err
		}
		if destinationChanged || before.Enabled != enabled || before.Code == "" {
			action := "merchant_link_updated"
			if destinationChanged {
				action = "merchant_destination_changed"
			}
			if err := appendMerchantGroupBuyAudit(tx, &before, &saved, action, ""); err != nil {
				return err
			}
			if destinationChanged {
				if err := appendGroupBuyAuditWithRule(tx, &before, &saved, "system_"+decision.Status, "system", 0, "团购自动审核", decision.Reason, decision.RuleID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func (s *MerchantGroupBuyService) ResolveAndTrack(code string) (string, error) {
	code = strings.TrimSpace(code)
	if len(code) != 32 {
		return "", ErrGroupBuyLinkNotFound
	}
	var link model.MerchantGroupBuyLink
	if err := s.db.Where("code = ? AND enabled = ? AND review_status = ? AND admin_suspended = ?", code, true, "approved", false).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrGroupBuyLinkNotFound
		}
		return "", err
	}
	var activeStoreCount int64
	if err := s.db.Table("stores").
		Joins("JOIN merchants ON merchants.id = stores.merchant_id").
		Where("stores.id = ? AND stores.merchant_id = ? AND stores.status = ? AND merchants.status = ? AND COALESCE(merchants.subscription_status, '') <> ?", link.StoreID, link.MerchantID, "active", "active", "paused").
		Count(&activeStoreCount).Error; err != nil {
		return "", err
	}
	if activeStoreCount == 0 {
		return "", ErrGroupBuyLinkNotFound
	}
	destination, err := ValidateGroupBuyDestination(link.Channel, link.DestinationURL)
	if err != nil {
		return "", fmt.Errorf("saved group-buy destination rejected: %w", err)
	}
	result := s.db.Model(&model.MerchantGroupBuyLink{}).
		Where(`id = ? AND enabled = ? AND review_status = ? AND admin_suspended = ?
			AND EXISTS (SELECT 1 FROM stores JOIN merchants ON merchants.id = stores.merchant_id
				WHERE stores.id = ? AND stores.merchant_id = ? AND stores.status = ? AND merchants.status = ?
				AND COALESCE(merchants.subscription_status, '') <> ?)`,
			link.ID, true, "approved", false, link.StoreID, link.MerchantID, "active", "active", "paused").
		Updates(map[string]interface{}{
			"click_count":     gorm.Expr("click_count + ?", 1),
			"last_clicked_at": time.Now(),
		})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", ErrGroupBuyLinkNotFound
	}
	return destination, nil
}

func (s *MerchantGroupBuyService) ownedStore(merchantID, storeID uint) (*model.Store, error) {
	if merchantID == 0 || storeID == 0 {
		return nil, errors.New("门店参数无效")
	}
	var store model.Store
	if err := s.db.Select("id", "merchant_id").Where("id = ? AND merchant_id = ?", storeID, merchantID).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupBuyStoreAccess
		}
		return nil, err
	}
	return &store, nil
}

// ValidateGroupBuyDestination accepts HTTPS links only on the selected platform's
// own domain family. It performs no network lookups and prevents arbitrary redirects.
func ValidateGroupBuyDestination(channel, raw string) (string, error) {
	parsed, err := parseSafeGroupBuyURL(raw)
	if err != nil {
		return "", err
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	allowedDomains := groupBuyChannelFamilies(channel)
	if len(allowedDomains) == 0 {
		return "", errors.New("暂不支持此团购平台")
	}
	if groupBuyHostInFamily(strings.ToLower(parsed.Hostname()), allowedDomains...) {
		return parsed.String(), nil
	}
	return "", errors.New("链接域名与所选平台不匹配，请粘贴抖音或美团/大众点评的官方链接")
}

func newGroupBuyCode() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
