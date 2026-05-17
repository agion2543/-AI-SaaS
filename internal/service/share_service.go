package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

type ShareService struct {
	db *gorm.DB
}

type ShareStats struct {
	TotalCampaigns     int64 `json:"total_campaigns"`
	ScanCount          int64 `json:"scan_count"`
	LeadCount          int64 `json:"lead_count"`
	ConversionCount    int64 `json:"conversion_count"`
	ConversionAmount   int64 `json:"conversion_amount"`
	RewardCouponCount  int64 `json:"reward_coupon_count"`
	UnusedCouponCount  int64 `json:"unused_coupon_count"`
	UsedCouponCount    int64 `json:"used_coupon_count"`
	ExpiredCouponCount int64 `json:"expired_coupon_count"`
	VoidedCouponCount  int64 `json:"voided_coupon_count"`
}

type ShareStatsParams struct {
	StartAt *time.Time
	EndAt   *time.Time
}

type CouponListParams struct {
	Phone    string
	Status   string
	StoreID  uint
	Keyword  string
	Page     int
	PageSize int
}

type CouponSourceOrder struct {
	ID            uint       `json:"id"`
	OrderNo       string     `json:"order_no"`
	CustomerPhone string     `json:"customer_phone"`
	Amount        int64      `json:"amount"`
	TotalAmount   int64      `json:"total_amount"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	PaidAt        *time.Time `json:"paid_at"`
}

type CouponListItem struct {
	model.ReferralCoupon
	SourceOrder *CouponSourceOrder `json:"source_order,omitempty"`
}

type CouponManageStats struct {
	Total   int64 `json:"total"`
	Unused  int64 `json:"unused"`
	Used    int64 `json:"used"`
	Expired int64 `json:"expired"`
	Voided  int64 `json:"voided"`
}

type ShareActivityConfigRequest struct {
	Enabled                 bool   `json:"enabled"`
	PosterTitle             string `json:"poster_title"`
	PosterCopy              string `json:"poster_copy"`
	FriendCouponAmount      int64  `json:"friend_coupon_amount"`
	FriendCouponThreshold   int64  `json:"friend_coupon_threshold"`
	ReferrerCouponAmount    int64  `json:"referrer_coupon_amount"`
	ReferrerCouponThreshold int64  `json:"referrer_coupon_threshold"`
	ValidDays               int    `json:"valid_days"`
}

func NewShareService(db *gorm.DB) *ShareService {
	return &ShareService{db: db}
}

func (s *ShareService) GetActivityConfig(merchantID uint) (*model.ShareActivityConfig, error) {
	var config model.ShareActivityConfig
	if err := s.db.Where("merchant_id = ?", merchantID).First(&config).Error; err == nil {
		return &config, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	config = model.ShareActivityConfig{
		MerchantID:              merchantID,
		Enabled:                 false,
		PosterTitle:             "好友扫码领券",
		PosterCopy:              "分享给好友，好友扫码领券下单，你也可获得复购奖励。",
		FriendCouponAmount:      500,
		FriendCouponThreshold:   3000,
		ReferrerCouponAmount:    500,
		ReferrerCouponThreshold: 3000,
		ValidDays:               30,
		Status:                  "active",
	}
	if err := s.db.Create(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *ShareService) UpdateActivityConfig(merchantID uint, req ShareActivityConfigRequest) (*model.ShareActivityConfig, error) {
	config, err := s.GetActivityConfig(merchantID)
	if err != nil {
		return nil, err
	}
	if req.ValidDays <= 0 || req.ValidDays > 365 {
		return nil, errors.New("valid days must be between 1 and 365")
	}
	if req.FriendCouponAmount < 0 || req.ReferrerCouponAmount < 0 || req.FriendCouponThreshold < 0 || req.ReferrerCouponThreshold < 0 {
		return nil, errors.New("coupon amount and threshold cannot be negative")
	}
	updates := map[string]interface{}{
		"enabled":                   req.Enabled,
		"poster_title":              strings.TrimSpace(req.PosterTitle),
		"poster_copy":               strings.TrimSpace(req.PosterCopy),
		"friend_coupon_amount":      req.FriendCouponAmount,
		"friend_coupon_threshold":   req.FriendCouponThreshold,
		"referrer_coupon_amount":    req.ReferrerCouponAmount,
		"referrer_coupon_threshold": req.ReferrerCouponThreshold,
		"valid_days":                req.ValidDays,
		"status":                    "active",
	}
	if err := s.db.Model(config).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetActivityConfig(merchantID)
}

func (s *ShareService) EnsureCampaignForOrder(orderNo string) (*model.ShareCampaign, error) {
	var order model.Order
	if err := s.db.Preload("Store").Preload("Merchant").Where("order_no = ? AND order_type = ?", orderNo, "store_order").First(&order).Error; err != nil {
		return nil, errors.New("订单不存在")
	}
	if order.MerchantID == nil || order.StoreID == nil {
		return nil, errors.New("订单缺少商家或门店信息")
	}

	config, err := s.GetActivityConfig(*order.MerchantID)
	if err != nil {
		return nil, err
	}
	if !config.Enabled || config.Status != "active" {
		return nil, errors.New("share activity is not enabled")
	}

	var campaign model.ShareCampaign
	if err := s.db.Where("order_id = ?", order.ID).First(&campaign).Error; err == nil {
		return &campaign, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	storeName := "本地好店"
	if order.Store != nil && order.Store.Name != "" {
		storeName = order.Store.Name
	}
	campaign = model.ShareCampaign{
		MerchantID:    *order.MerchantID,
		StoreID:       *order.StoreID,
		OrderID:       &order.ID,
		ShareCode:     newShareCode(),
		CustomerPhone: order.CustomerPhone,
		PosterTitle:   fmt.Sprintf("%s 好友领券", storeName),
		PosterCopy:    fmt.Sprintf("我刚在 %s 下单，体验不错。扫码领取好友专属优惠，下次一起试试。", storeName),
		Status:        "active",
	}
	if title := strings.TrimSpace(config.PosterTitle); title != "" {
		campaign.PosterTitle = title
	}
	if copy := strings.TrimSpace(config.PosterCopy); copy != "" {
		campaign.PosterCopy = copy
	}
	if err := s.db.Create(&campaign).Error; err != nil {
		return nil, err
	}
	return &campaign, nil
}

func (s *ShareService) TrackScan(code string) (*model.ShareCampaign, error) {
	campaign, err := s.FindByCode(code)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.db.Model(campaign).Updates(map[string]interface{}{
		"scan_count":      gorm.Expr("scan_count + ?", 1),
		"last_scanned_at": &now,
	}).Error; err != nil {
		return nil, err
	}
	campaign.ScanCount++
	campaign.LastScannedAt = &now
	return campaign, nil
}

func (s *ShareService) RecordConversion(code string, order *model.Order) {
	code = strings.TrimSpace(code)
	if code == "" || order == nil || order.ID == 0 {
		return
	}
	var campaign model.ShareCampaign
	if err := s.db.Where("share_code = ? AND status = ?", code, "active").First(&campaign).Error; err != nil {
		return
	}
	if order.MerchantID == nil || order.StoreID == nil || campaign.MerchantID != *order.MerchantID {
		return
	}
	amount := order.TotalAmount
	if amount <= 0 {
		amount = order.Amount
	}
	_ = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&campaign).Updates(map[string]interface{}{
			"conversion_count":  gorm.Expr("conversion_count + ?", 1),
			"conversion_amount": gorm.Expr("conversion_amount + ?", amount),
		}).Error; err != nil {
			return err
		}
		return NewShareService(tx).issueRewardCoupons(&campaign, order)
	})
}

func (s *ShareService) FindByCode(code string) (*model.ShareCampaign, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("分享码不能为空")
	}
	var campaign model.ShareCampaign
	if err := s.db.Preload("Store").Preload("Merchant").Where("share_code = ? AND status = ?", code, "active").First(&campaign).Error; err != nil {
		return nil, errors.New("分享活动不存在或已失效")
	}
	return &campaign, nil
}

func (s *ShareService) StatsByMerchant(merchantID uint, params ShareStatsParams) (*ShareStats, []model.ShareCampaign, []model.ReferralCoupon, error) {
	var stats ShareStats
	var campaigns []model.ShareCampaign
	var coupons []model.ReferralCoupon
	query := s.db.Model(&model.ShareCampaign{}).Where("merchant_id = ?", merchantID)
	if params.StartAt != nil {
		query = query.Where("created_at >= ?", *params.StartAt)
	}
	if params.EndAt != nil {
		query = query.Where("created_at <= ?", *params.EndAt)
	}
	query.Count(&stats.TotalCampaigns)
	query.Select("COALESCE(SUM(scan_count),0)").Scan(&stats.ScanCount)
	query.Select("COALESCE(SUM(lead_count),0)").Scan(&stats.LeadCount)
	query.Select("COALESCE(SUM(conversion_count),0)").Scan(&stats.ConversionCount)
	query.Select("COALESCE(SUM(conversion_amount),0)").Scan(&stats.ConversionAmount)

	couponQuery := s.db.Model(&model.ReferralCoupon{}).Where("merchant_id = ?", merchantID)
	if params.StartAt != nil {
		couponQuery = couponQuery.Where("created_at >= ?", *params.StartAt)
	}
	if params.EndAt != nil {
		couponQuery = couponQuery.Where("created_at <= ?", *params.EndAt)
	}
	couponQuery.Count(&stats.RewardCouponCount)
	couponStatusQuery := func(status string) *gorm.DB {
		q := s.db.Model(&model.ReferralCoupon{}).Where("merchant_id = ? AND status = ?", merchantID, status)
		if params.StartAt != nil {
			q = q.Where("created_at >= ?", *params.StartAt)
		}
		if params.EndAt != nil {
			q = q.Where("created_at <= ?", *params.EndAt)
		}
		return q
	}
	couponStatusQuery("unused").Count(&stats.UnusedCouponCount)
	couponStatusQuery("used").Count(&stats.UsedCouponCount)
	couponStatusQuery("expired").Count(&stats.ExpiredCouponCount)
	couponStatusQuery("voided").Count(&stats.VoidedCouponCount)

	campaignQuery := s.db.Preload("Store").Where("merchant_id = ?", merchantID)
	if params.StartAt != nil {
		campaignQuery = campaignQuery.Where("created_at >= ?", *params.StartAt)
	}
	if params.EndAt != nil {
		campaignQuery = campaignQuery.Where("created_at <= ?", *params.EndAt)
	}
	if err := campaignQuery.Order("conversion_count desc, scan_count desc, id desc").Limit(50).Find(&campaigns).Error; err != nil {
		return nil, nil, nil, err
	}
	recentCouponQuery := s.db.Preload("Store").Where("merchant_id = ?", merchantID)
	if params.StartAt != nil {
		recentCouponQuery = recentCouponQuery.Where("created_at >= ?", *params.StartAt)
	}
	if params.EndAt != nil {
		recentCouponQuery = recentCouponQuery.Where("created_at <= ?", *params.EndAt)
	}
	if err := recentCouponQuery.Order("id desc").Limit(50).Find(&coupons).Error; err != nil {
		return nil, nil, nil, err
	}
	return &stats, campaigns, coupons, nil
}

func (s *ShareService) ListMerchantCoupons(merchantID uint, params CouponListParams) (*CouponManageStats, []CouponListItem, int64, error) {
	if err := s.expireCoupons(time.Now()); err != nil {
		return nil, nil, 0, err
	}

	stats := &CouponManageStats{}
	s.db.Model(&model.ReferralCoupon{}).Where("merchant_id = ?", merchantID).Count(&stats.Total)
	s.db.Model(&model.ReferralCoupon{}).Where("merchant_id = ? AND status = ?", merchantID, "unused").Count(&stats.Unused)
	s.db.Model(&model.ReferralCoupon{}).Where("merchant_id = ? AND status = ?", merchantID, "used").Count(&stats.Used)
	s.db.Model(&model.ReferralCoupon{}).Where("merchant_id = ? AND status = ?", merchantID, "expired").Count(&stats.Expired)
	s.db.Model(&model.ReferralCoupon{}).Where("merchant_id = ? AND status = ?", merchantID, "voided").Count(&stats.Voided)

	query := s.db.Preload("Store").Preload("ShareCampaign").
		Where("merchant_id = ?", merchantID).
		Model(&model.ReferralCoupon{})
	if phone := normalizePhone(params.Phone); phone != "" {
		query = query.Where("owner_phone LIKE ?", "%"+phone+"%")
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.StoreID > 0 {
		query = query.Where("store_id = ?", params.StoreID)
	}
	if keyword := strings.TrimSpace(params.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("coupon_no LIKE ? OR title LIKE ? OR remark LIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}
	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var coupons []model.ReferralCoupon
	if err := query.Order("id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&coupons).Error; err != nil {
		return nil, nil, 0, err
	}

	orderIDs := make([]uint, 0, len(coupons))
	for _, coupon := range coupons {
		if coupon.OrderID != nil && *coupon.OrderID > 0 {
			orderIDs = append(orderIDs, *coupon.OrderID)
		}
	}
	orderMap := map[uint]CouponSourceOrder{}
	if len(orderIDs) > 0 {
		var orders []model.Order
		if err := s.db.Where("merchant_id = ? AND id IN ?", merchantID, orderIDs).Find(&orders).Error; err != nil {
			return nil, nil, 0, err
		}
		for _, order := range orders {
			orderMap[order.ID] = CouponSourceOrder{
				ID:            order.ID,
				OrderNo:       order.OrderNo,
				CustomerPhone: order.CustomerPhone,
				Amount:        order.Amount,
				TotalAmount:   order.TotalAmount,
				Status:        order.Status,
				CreatedAt:     order.CreatedAt,
				PaidAt:        order.PaidAt,
			}
		}
	}

	items := make([]CouponListItem, 0, len(coupons))
	for _, coupon := range coupons {
		item := CouponListItem{ReferralCoupon: coupon}
		if coupon.OrderID != nil {
			if order, ok := orderMap[*coupon.OrderID]; ok {
				item.SourceOrder = &order
			}
		}
		items = append(items, item)
	}
	return stats, items, total, nil
}

func (s *ShareService) RedeemMerchantCoupon(merchantID, couponID uint, remark string) (*model.ReferralCoupon, error) {
	return s.updateCouponStatus(merchantID, couponID, "used", strings.TrimSpace(remark))
}

func (s *ShareService) VoidMerchantCoupon(merchantID, couponID uint, remark string) (*model.ReferralCoupon, error) {
	return s.updateCouponStatus(merchantID, couponID, "voided", strings.TrimSpace(remark))
}

func (s *ShareService) ListUsableCoupons(storeID uint, phone string) ([]model.ReferralCoupon, error) {
	phone = normalizePhone(phone)
	if phone == "" {
		return []model.ReferralCoupon{}, nil
	}
	now := time.Now()
	if err := s.expireCoupons(now); err != nil {
		return nil, err
	}
	var coupons []model.ReferralCoupon
	err := s.db.Where("store_id = ? AND owner_phone = ? AND status = ?", storeID, phone, "unused").
		Where("(valid_from IS NULL OR valid_from <= ?) AND (valid_to IS NULL OR valid_to >= ?)", now, now).
		Order("amount desc, id desc").
		Find(&coupons).Error
	return coupons, err
}

func (s *ShareService) ValidateCouponForOrder(storeID uint, phone, couponNo string, amount int64) (*model.ReferralCoupon, int64, error) {
	couponNo = strings.TrimSpace(couponNo)
	phone = normalizePhone(phone)
	if couponNo == "" {
		return nil, 0, nil
	}
	if phone == "" {
		return nil, 0, errors.New("使用奖励券需要填写手机号")
	}
	now := time.Now()
	if err := s.expireCoupons(now); err != nil {
		return nil, 0, err
	}
	var coupon model.ReferralCoupon
	if err := s.db.Where("coupon_no = ? AND store_id = ? AND owner_phone = ?", couponNo, storeID, phone).First(&coupon).Error; err != nil {
		return nil, 0, errors.New("奖励券不存在或不属于当前手机号")
	}
	if coupon.Status != "unused" {
		return nil, 0, errors.New("奖励券已使用或已失效")
	}
	if coupon.ValidFrom != nil && coupon.ValidFrom.After(now) {
		return nil, 0, errors.New("奖励券尚未生效")
	}
	if coupon.ValidTo != nil && coupon.ValidTo.Before(now) {
		coupon.Status = "expired"
		_ = s.db.Save(&coupon).Error
		return nil, 0, errors.New("奖励券已过期")
	}
	if amount < coupon.Threshold {
		return nil, 0, fmt.Errorf("订单满 %s 元才可使用该券", formatFen(coupon.Threshold))
	}
	discount := coupon.Amount
	if discount > amount {
		discount = amount
	}
	return &coupon, discount, nil
}

func (s *ShareService) ConsumeCouponForOrder(order *model.Order) error {
	if order == nil || order.CouponID == nil {
		return nil
	}
	now := time.Now()
	return s.db.Model(&model.ReferralCoupon{}).
		Where("id = ? AND status = ?", *order.CouponID, "unused").
		Updates(map[string]interface{}{
			"status":  "used",
			"used_at": &now,
		}).Error
}

func (s *ShareService) updateCouponStatus(merchantID, couponID uint, targetStatus, remark string) (*model.ReferralCoupon, error) {
	if err := s.expireCoupons(time.Now()); err != nil {
		return nil, err
	}
	var coupon model.ReferralCoupon
	if err := s.db.Where("id = ? AND merchant_id = ?", couponID, merchantID).First(&coupon).Error; err != nil {
		return nil, errors.New("奖励券不存在或不属于当前商家")
	}
	if coupon.Status != "unused" {
		return nil, errors.New("只有未使用且未过期的奖励券可以操作")
	}
	now := time.Now()
	updates := map[string]interface{}{
		"status": targetStatus,
	}
	if targetStatus == "used" {
		updates["used_at"] = &now
	}
	if remark != "" {
		if coupon.Remark != "" {
			remark = coupon.Remark + "；" + remark
		}
		updates["remark"] = remark
	}
	if err := s.db.Model(&coupon).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.Preload("Store").Preload("ShareCampaign").First(&coupon, coupon.ID).Error; err != nil {
		return nil, err
	}
	return &coupon, nil
}

func (s *ShareService) expireCoupons(now time.Time) error {
	return s.db.Model(&model.ReferralCoupon{}).
		Where("status = ? AND valid_to IS NOT NULL AND valid_to < ?", "unused", now).
		Update("status", "expired").Error
}

func (s *ShareService) issueRewardCoupons(campaign *model.ShareCampaign, order *model.Order) error {
	if campaign == nil || order == nil || order.MerchantID == nil || order.StoreID == nil {
		return nil
	}
	var existing int64
	if err := s.db.Model(&model.ReferralCoupon{}).
		Where("order_id = ? AND share_campaign_id = ?", order.ID, campaign.ID).
		Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}

	config, err := s.GetActivityConfig(campaign.MerchantID)
	if err != nil {
		return err
	}
	if !config.Enabled || config.Status != "active" {
		return nil
	}
	validDays := config.ValidDays
	if validDays <= 0 {
		validDays = 30
	}

	now := time.Now()
	validTo := now.AddDate(0, 0, validDays)
	coupons := make([]model.ReferralCoupon, 0, 2)
	if order.CustomerPhone != "" {
		coupons = append(coupons, model.ReferralCoupon{
			MerchantID:      *order.MerchantID,
			StoreID:         *order.StoreID,
			ShareCampaignID: &campaign.ID,
			OrderID:         &order.ID,
			CouponNo:        newCouponNo("NC"),
			OwnerPhone:      order.CustomerPhone,
			OwnerType:       "new_customer",
			Title:           "好友新客下次到店券",
			Amount:          config.FriendCouponAmount,
			Threshold:       config.FriendCouponThreshold,
			Status:          "unused",
			ValidFrom:       &now,
			ValidTo:         &validTo,
			Remark:          "好友扫码下单后自动发放",
		})
	}
	if campaign.CustomerPhone != "" {
		coupons = append(coupons, model.ReferralCoupon{
			MerchantID:      *order.MerchantID,
			StoreID:         *order.StoreID,
			ShareCampaignID: &campaign.ID,
			OrderID:         &order.ID,
			CouponNo:        newCouponNo("RF"),
			OwnerPhone:      campaign.CustomerPhone,
			OwnerType:       "referrer",
			Title:           "分享奖励复购券",
			Amount:          config.ReferrerCouponAmount,
			Threshold:       config.ReferrerCouponThreshold,
			Status:          "unused",
			ValidFrom:       &now,
			ValidTo:         &validTo,
			Remark:          "好友完成支付后奖励给分享人",
		})
	}
	if len(coupons) == 0 {
		return nil
	}
	if err := s.db.Create(&coupons).Error; err != nil {
		return err
	}
	return s.db.Model(campaign).Update("lead_count", gorm.Expr("lead_count + ?", len(coupons))).Error
}

func newShareCode() string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(raw) > 12 {
		return raw[:12]
	}
	return raw
}

func newCouponNo(prefix string) string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(raw) > 16 {
		raw = raw[:16]
	}
	return fmt.Sprintf("%s%s", prefix, strings.ToUpper(raw))
}
