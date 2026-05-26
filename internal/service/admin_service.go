package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
	"go-web-gin-health/internal/utils"
)

type AdminService struct {
	cfg       *config.Config
	db        *gorm.DB
	users     *repository.UserRepository
	merchants *repository.MerchantRepository
	stores    *repository.StoreRepository
	leads     *repository.CustomerLeadRepository
	plans     *repository.MerchantPlanRepository
	packages  *repository.PackageRepository
	cards     *repository.CardRepository
	systems   *repository.SystemRepository
}

type MerchantSettlementPrepare struct {
	MerchantID        uint          `json:"merchant_id"`
	OrderCount        int           `json:"order_count"`
	TotalAmountCents  int64         `json:"total_amount_cents"`
	RefundAmountCents int64         `json:"refund_amount_cents"`
	NetAmountCents    int64         `json:"net_amount_cents"`
	PeriodStart       *time.Time    `json:"settlement_period_start"`
	PeriodEnd         *time.Time    `json:"settlement_period_end"`
	Orders            []model.Order `json:"orders"`
}

type SecurityCheckItem struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

type SecurityCheckReport struct {
	OverallStatus  string                 `json:"overall_status"`
	GeneratedAt    time.Time              `json:"generated_at"`
	Environment    string                 `json:"environment"`
	Items          []SecurityCheckItem    `json:"items"`
	HealthSections []OperationHealthBlock `json:"health_sections"`
}

type OperationHealthBlock struct {
	Key        string                  `json:"key"`
	Title      string                  `json:"title"`
	Status     string                  `json:"status"`
	Summary    string                  `json:"summary"`
	Suggestion string                  `json:"suggestion"`
	ActionText string                  `json:"action_text"`
	ActionPath string                  `json:"action_path"`
	Metrics    []OperationHealthMetric `json:"metrics"`
}

type OperationHealthMetric struct {
	Label  string `json:"label"`
	Value  int64  `json:"value"`
	Hint   string `json:"hint"`
	Status string `json:"status"`
}

type AIProviderPreset struct {
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	Model       string `json:"model"`
	Description string `json:"description"`
}

type AIConfigStatus struct {
	Enabled        bool               `json:"enabled"`
	Provider       string             `json:"provider"`
	BaseURL        string             `json:"base_url"`
	Model          string             `json:"model"`
	TimeoutSeconds int                `json:"timeout_seconds"`
	HasAPIKey      bool               `json:"has_api_key"`
	Ready          bool               `json:"ready"`
	Status         string             `json:"status"`
	Summary        string             `json:"summary"`
	Suggestion     string             `json:"suggestion"`
	EnvExample     string             `json:"env_example"`
	Presets        []AIProviderPreset `json:"presets"`
}

type AIUsageScenarioStat struct {
	Scenario string `json:"scenario"`
	Count    int64  `json:"count"`
}

type AIUsageOverview struct {
	TodayTotal      int64                 `json:"today_total"`
	TodaySuccess    int64                 `json:"today_success"`
	TodayFailed     int64                 `json:"today_failed"`
	TodayFallback   int64                 `json:"today_fallback"`
	ActiveMerchants int64                 `json:"active_merchants"`
	AverageLatency  int64                 `json:"average_latency_ms"`
	FailureRate     int64                 `json:"failure_rate"`
	TopScenarios    []AIUsageScenarioStat `json:"top_scenarios"`
	Suggestion      string                `json:"suggestion"`
}

type MerchantListRow struct {
	model.Merchant
	OrderCount             int64      `json:"order_count"`
	TradeAmount            int64      `json:"trade_amount"`
	RefundAmount           int64      `json:"refund_amount"`
	PendingOrderCount      int64      `json:"pending_order_count"`
	FailedOrderCount       int64      `json:"failed_order_count"`
	ReceivedOrderCount     int64      `json:"received_order_count"`
	ClosedOrderCount       int64      `json:"closed_order_count"`
	UnsettledOrderCount    int64      `json:"unsettled_order_count"`
	PendingSettlementCount int64      `json:"pending_settlement_count"`
	OpenFollowUpCount      int64      `json:"open_follow_up_count"`
	LastFollowUpAt         *time.Time `json:"last_follow_up_at"`
	PaymentConfigStatus    string     `json:"payment_config_status"`
	PaymentConfigAudit     string     `json:"payment_config_audit"`
	PaymentConfigMode      string     `json:"payment_config_mode"`
	HasAlipayQRCode        bool       `json:"has_alipay_qr_code"`
	HasWechatQRCode        bool       `json:"has_wechat_qr_code"`
}

func NewAdminService(cfg *config.Config, db *gorm.DB) *AdminService {
	return &AdminService{
		cfg:       cfg,
		db:        db,
		users:     repository.NewUserRepository(db),
		merchants: repository.NewMerchantRepository(db),
		stores:    repository.NewStoreRepository(db),
		leads:     repository.NewCustomerLeadRepository(db),
		plans:     repository.NewMerchantPlanRepository(db),
		packages:  repository.NewPackageRepository(db),
		cards:     repository.NewCardRepository(db),
		systems:   repository.NewSystemRepository(db),
	}
}

func (s *AdminService) ListUsers(page, pageSize int) ([]model.User, int64, error) {
	return s.users.List((page-1)*pageSize, pageSize)
}

func (s *AdminService) ListUsersFiltered(page, pageSize int, filter repository.UserListFilter) ([]model.User, int64, error) {
	return s.users.ListFiltered(filter, (page-1)*pageSize, pageSize)
}

type CustomerProfileRow struct {
	CustomerPhone  string    `json:"customer_phone"`
	MerchantID     uint      `json:"merchant_id"`
	MerchantName   string    `json:"merchant_name"`
	StoreName      string    `json:"store_name"`
	OrderCount     int64     `json:"order_count"`
	TotalAmount    int64     `json:"total_amount"`
	DiscountAmount int64     `json:"discount_amount"`
	LastOrderAt    time.Time `json:"last_order_at"`
	LastStatus     string    `json:"last_status"`
	AITag          string    `json:"ai_tag"`
	AISuggestion   string    `json:"ai_suggestion"`
}

type CustomerProfileFilter struct {
	Keyword   string
	AITag     string
	Status    string
	SortBy    string
	SortOrder string
}

func (s *AdminService) ListCustomerProfiles(page, pageSize int, filter CustomerProfileFilter) ([]CustomerProfileRow, int64, error) {
	var rows []CustomerProfileRow
	query := s.db.Table("orders").
		Select(`orders.customer_phone AS customer_phone,
			orders.merchant_id AS merchant_id,
			MAX(merchants.name) AS merchant_name,
			MAX(stores.name) AS store_name,
			COUNT(orders.id) AS order_count,
			COALESCE(SUM(orders.total_amount),0) AS total_amount,
			COALESCE(SUM(orders.discount_amount),0) AS discount_amount,
			MAX(orders.created_at) AS last_order_at,
			SUBSTRING_INDEX(GROUP_CONCAT(orders.status ORDER BY orders.created_at DESC), ',', 1) AS last_status`).
		Joins("LEFT JOIN merchants ON merchants.id = orders.merchant_id").
		Joins("LEFT JOIN stores ON stores.id = orders.store_id").
		Where("orders.order_type = ? AND orders.customer_phone <> ''", "store_order").
		Where("orders.status IN ?", []string{"received", "accepted", "completed", "closed"})

	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("orders.customer_phone LIKE ? OR merchants.name LIKE ? OR stores.name LIKE ?", like, like, like)
	}

	query = query.Group("orders.customer_phone, orders.merchant_id")
	if err := query.Order("last_order_at desc").Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

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
		return []CustomerProfileRow{}, total, nil
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], total, nil
}

func (s *AdminService) ListMerchantCustomerProfiles(merchantID uint, limit int) ([]CustomerProfileRow, error) {
	if limit <= 0 {
		limit = 10
	}
	var rows []CustomerProfileRow
	err := s.db.Table("orders").
		Select(`orders.customer_phone AS customer_phone,
			orders.merchant_id AS merchant_id,
			MAX(merchants.name) AS merchant_name,
			MAX(stores.name) AS store_name,
			COUNT(orders.id) AS order_count,
			COALESCE(SUM(orders.total_amount),0) AS total_amount,
			COALESCE(SUM(orders.discount_amount),0) AS discount_amount,
			MAX(orders.created_at) AS last_order_at,
			SUBSTRING_INDEX(GROUP_CONCAT(orders.status ORDER BY orders.created_at DESC), ',', 1) AS last_status`).
		Joins("LEFT JOIN merchants ON merchants.id = orders.merchant_id").
		Joins("LEFT JOIN stores ON stores.id = orders.store_id").
		Where("orders.merchant_id = ? AND orders.order_type = ? AND orders.customer_phone <> ''", merchantID, "store_order").
		Where("orders.status IN ?", []string{"received", "accepted", "completed", "closed"}).
		Group("orders.customer_phone, orders.merchant_id").
		Order("total_amount desc, last_order_at desc").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].AITag, rows[i].AISuggestion = buildCustomerInsight(rows[i])
	}
	return rows, nil
}

func buildCustomerInsight(row CustomerProfileRow) (string, string) {
	daysSinceLastOrder := int(time.Since(row.LastOrderAt).Hours() / 24)
	if !row.LastOrderAt.IsZero() && daysSinceLastOrder >= 30 {
		return "沉睡顾客", "建议推送限时唤醒券，并优先展示近期热销商品。"
	}
	if row.TotalAmount >= 10000 || row.OrderCount >= 5 {
		return "高价值顾客", "建议进入重点维护名单，可配置会员专属券或新品优先通知。"
	}
	if row.OrderCount >= 2 {
		return "复购顾客", "建议在下单后 3-7 天推送复购优惠，提升再次转化。"
	}
	if row.LastStatus == "closed" {
		return "流失风险", "最近订单已关闭，建议检查支付或履约体验，并投放低门槛优惠。"
	}
	return "新顾客", "建议发送首单感谢和二次到店优惠，尽快形成复购。"
}

func (s *AdminService) ListMerchants(page, pageSize int) ([]MerchantListRow, int64, error) {
	merchants, total, err := s.merchants.List((page-1)*pageSize, pageSize)
	if err != nil || len(merchants) == 0 {
		return []MerchantListRow{}, total, err
	}
	ids := make([]uint, 0, len(merchants))
	for _, merchant := range merchants {
		ids = append(ids, merchant.ID)
	}

	type orderAgg struct {
		MerchantID          uint
		OrderCount          int64
		TradeAmount         int64
		RefundAmount        int64
		PendingOrderCount   int64
		FailedOrderCount    int64
		ReceivedOrderCount  int64
		ClosedOrderCount    int64
		UnsettledOrderCount int64
	}
	var orderAggs []orderAgg
	if err := s.db.Table("orders").
		Select(`merchant_id,
			COUNT(*) AS order_count,
			COALESCE(SUM(total_amount),0) AS trade_amount,
			COALESCE(SUM(refunded_amount),0) AS refund_amount,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) AS pending_order_count,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed_order_count,
			SUM(CASE WHEN status = 'received' THEN 1 ELSE 0 END) AS received_order_count,
			SUM(CASE WHEN status = 'closed' THEN 1 ELSE 0 END) AS closed_order_count,
			SUM(CASE WHEN settlement_id IS NULL AND status IN ('received','accepted','completed') THEN 1 ELSE 0 END) AS unsettled_order_count`).
		Where("merchant_id IN ? AND order_type = ?", ids, "store_order").
		Group("merchant_id").
		Scan(&orderAggs).Error; err != nil {
		return nil, 0, err
	}
	orderMap := make(map[uint]orderAgg, len(orderAggs))
	for _, item := range orderAggs {
		orderMap[item.MerchantID] = item
	}

	type settlementAgg struct {
		MerchantID uint
		Count      int64
	}
	var settlementAggs []settlementAgg
	if err := s.db.Table("merchant_settlements").
		Select("merchant_id, COUNT(*) AS count").
		Where("merchant_id IN ? AND status = ?", ids, "pending").
		Group("merchant_id").
		Scan(&settlementAggs).Error; err != nil {
		return nil, 0, err
	}
	settlementMap := make(map[uint]int64, len(settlementAggs))
	for _, item := range settlementAggs {
		settlementMap[item.MerchantID] = item.Count
	}

	type followAgg struct {
		MerchantID     uint
		OpenCount      int64
		LastFollowUpAt *time.Time
	}
	var followAggs []followAgg
	if err := s.db.Table("merchant_follow_ups").
		Select("merchant_id, SUM(CASE WHEN status = 'open' THEN 1 ELSE 0 END) AS open_count, MAX(created_at) AS last_follow_up_at").
		Where("merchant_id IN ?", ids).
		Group("merchant_id").
		Scan(&followAggs).Error; err != nil {
		return nil, 0, err
	}
	followMap := make(map[uint]followAgg, len(followAggs))
	for _, item := range followAggs {
		followMap[item.MerchantID] = item
	}

	var paymentConfigs []model.MerchantPaymentConfig
	if err := s.db.Where("merchant_id IN ?", ids).Find(&paymentConfigs).Error; err != nil {
		return nil, 0, err
	}
	paymentMap := make(map[uint]model.MerchantPaymentConfig, len(paymentConfigs))
	for _, item := range paymentConfigs {
		paymentMap[item.MerchantID] = item
	}

	rows := make([]MerchantListRow, 0, len(merchants))
	for _, merchant := range merchants {
		row := MerchantListRow{Merchant: merchant}
		if agg, ok := orderMap[merchant.ID]; ok {
			row.OrderCount = agg.OrderCount
			row.TradeAmount = agg.TradeAmount
			row.RefundAmount = agg.RefundAmount
			row.PendingOrderCount = agg.PendingOrderCount
			row.FailedOrderCount = agg.FailedOrderCount
			row.ReceivedOrderCount = agg.ReceivedOrderCount
			row.ClosedOrderCount = agg.ClosedOrderCount
			row.UnsettledOrderCount = agg.UnsettledOrderCount
		}
		row.PendingSettlementCount = settlementMap[merchant.ID]
		if agg, ok := followMap[merchant.ID]; ok {
			row.OpenFollowUpCount = agg.OpenCount
			row.LastFollowUpAt = agg.LastFollowUpAt
		}
		if config, ok := paymentMap[merchant.ID]; ok {
			row.PaymentConfigStatus = config.Status
			row.PaymentConfigAudit = config.AuditStatus
			row.PaymentConfigMode = config.Mode
			row.HasAlipayQRCode = strings.TrimSpace(config.AlipayQRCode) != ""
			row.HasWechatQRCode = strings.TrimSpace(config.WechatQRCode) != ""
		}
		rows = append(rows, row)
	}
	return rows, total, nil
}

func (s *AdminService) ListMerchantPlans() ([]model.MerchantPlan, error) {
	return s.plans.List()
}

func (s *AdminService) SaveMerchantPlan(id uint, req dto.SaveMerchantPlanRequest) (*model.MerchantPlan, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("套餐名称不能为空")
	}
	if req.PriceCents < 0 {
		return nil, errors.New("套餐价格不能为负数")
	}
	if req.DurationDays <= 0 {
		return nil, errors.New("套餐周期必须大于 0 天")
	}

	plan := &model.MerchantPlan{}
	var err error
	if id > 0 {
		plan, err = s.plans.FindByID(id)
		if err != nil {
			return nil, err
		}
	}
	plan.Name = name
	plan.PriceCents = req.PriceCents
	plan.DurationDays = req.DurationDays
	plan.Sort = req.Sort
	if err := s.plans.Save(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *AdminService) UpdateMerchantStatus(id uint, status string) error {
	merchant, err := s.merchants.FindByID(id)
	if err != nil {
		return err
	}
	merchant.Status = status
	return s.merchants.Save(merchant)
}

func (s *AdminService) ListMerchantFollowUps(merchantID uint) ([]model.MerchantFollowUp, error) {
	var rows []model.MerchantFollowUp
	if err := s.db.Where("merchant_id = ?", merchantID).Order("status desc, id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *AdminService) ListAllMerchantFollowUps(status, priority string) ([]model.MerchantFollowUp, error) {
	var rows []model.MerchantFollowUp
	query := s.db.Preload("Merchant").Model(&model.MerchantFollowUp{})
	if value := strings.TrimSpace(status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(priority); value != "" {
		query = query.Where("priority = ?", value)
	}
	if err := query.Order("status desc, next_follow_at asc, id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *AdminService) CreateMerchantFollowUp(merchantID, operatorID uint, operatorRole string, req dto.CreateMerchantFollowUpRequest) (*model.MerchantFollowUp, error) {
	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errors.New("follow-up content is required")
	}
	if len([]rune(content)) > 1000 {
		return nil, errors.New("follow-up content cannot exceed 1000 characters")
	}
	followType := strings.TrimSpace(req.Type)
	if followType == "" {
		followType = "risk"
	}
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	var nextFollowAt *time.Time
	if value := strings.TrimSpace(req.NextFollowAt); value != "" {
		parsed, err := parseFollowUpTime(value)
		if err != nil {
			return nil, err
		}
		nextFollowAt = &parsed
	}
	var orderID *uint
	if req.OrderID > 0 {
		orderID = &req.OrderID
	}
	row := &model.MerchantFollowUp{
		MerchantID:   merchantID,
		Type:         followType,
		Priority:     priority,
		Status:       "open",
		Content:      content,
		Source:       trimTo(strings.TrimSpace(req.Source), 40),
		SourceID:     req.SourceID,
		OrderID:      orderID,
		OrderNo:      trimTo(strings.TrimSpace(req.OrderNo), 60),
		NextFollowAt: nextFollowAt,
		OperatorID:   operatorID,
		OperatorRole: strings.TrimSpace(operatorRole),
	}
	if err := s.db.Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func (s *AdminService) UpdateMerchantFollowUpStatus(id uint, status string) (*model.MerchantFollowUp, error) {
	var row model.MerchantFollowUp
	if err := s.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	row.Status = status
	if err := s.db.Save(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *AdminService) OpenMerchantSubscription(id uint, req dto.OpenMerchantSubscriptionRequest) (*model.Merchant, error) {
	merchant, err := s.merchants.FindByID(id)
	if err != nil {
		return nil, err
	}

	plan := strings.TrimSpace(req.Plan)
	note := strings.TrimSpace(req.Note)
	days := req.DurationDays
	if plan != "month" && plan != "year" {
		return nil, errors.New("subscription plan is invalid")
	}
	merchantPlanName := map[string]string{"month": "月付", "year": "年付"}[plan]
	merchantPlan, err := s.plans.FindByName(merchantPlanName)
	if err != nil {
		return nil, errors.New("merchant subscription plan is not initialized")
	}
	if days == 0 {
		days = merchantPlan.DurationDays
	}

	merchant.SubscriptionPlan = plan
	merchant.SubscriptionStatus = "active"
	merchant.SubscriptionNote = note
	baseTime := time.Now()
	if merchant.SubscriptionExpireAt != nil && merchant.SubscriptionExpireAt.After(baseTime) {
		baseTime = *merchant.SubscriptionExpireAt
	} else if merchant.SubscriptionExpiredAt != nil && merchant.SubscriptionExpiredAt.After(baseTime) {
		baseTime = *merchant.SubscriptionExpiredAt
	}
	expiredAt := baseTime.AddDate(0, 0, days)
	if expiredAt.Before(time.Now()) {
		merchant.SubscriptionStatus = "inactive"
	}
	merchant.SubscriptionExpiredAt = &expiredAt
	merchant.SubscriptionPlanID = &merchantPlan.ID
	merchant.SubscriptionExpireAt = &expiredAt

	if err := s.merchants.Save(merchant); err != nil {
		return nil, err
	}
	return merchant, nil
}

func (s *AdminService) StopMerchantSubscription(id uint) (*model.Merchant, error) {
	merchant, err := s.merchants.FindByID(id)
	if err != nil {
		return nil, err
	}
	merchant.SubscriptionStatus = "inactive"
	merchant.SubscriptionExpireAt = nil
	merchant.SubscriptionExpiredAt = nil
	if err := s.merchants.Save(merchant); err != nil {
		return nil, err
	}
	return merchant, nil
}

func (s *AdminService) GetMerchant(id uint) (*model.Merchant, error) {
	return s.merchants.FindByID(id)
}

func (s *AdminService) ListMerchantSubscriptionOrders(merchantID uint) ([]model.Order, error) {
	var orders []model.Order
	err := s.db.Preload("MerchantPlan").
		Where("merchant_id = ? AND order_type = ?", merchantID, "merchant_subscription").
		Order("created_at desc").
		Limit(20).
		Find(&orders).Error
	return orders, err
}

func (s *AdminService) ListMerchantStoreOrders(merchantID uint, limit int) ([]model.Order, error) {
	if limit <= 0 {
		limit = 10
	}
	var orders []model.Order
	err := s.db.Preload("Store").Preload("Promotion").
		Where("merchant_id = ? AND order_type = ?", merchantID, "store_order").
		Order("created_at desc").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func (s *AdminService) PrepareMerchantSettlement(merchantID uint) (*MerchantSettlementPrepare, error) {
	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, errors.New("商家不存在")
	}
	orders, err := s.unsettledMerchantOrders(merchantID)
	if err != nil {
		return nil, err
	}
	prepare := &MerchantSettlementPrepare{
		MerchantID: merchantID,
		Orders:     orders,
	}
	for i := range orders {
		order := orders[i]
		prepare.OrderCount++
		prepare.TotalAmountCents += settlementOrderAmount(order)
		prepare.RefundAmountCents += order.RefundedAmount
		if prepare.PeriodStart == nil || order.CreatedAt.Before(*prepare.PeriodStart) {
			t := order.CreatedAt
			prepare.PeriodStart = &t
		}
		if prepare.PeriodEnd == nil || order.CreatedAt.After(*prepare.PeriodEnd) {
			t := order.CreatedAt
			prepare.PeriodEnd = &t
		}
	}
	prepare.NetAmountCents = prepare.TotalAmountCents - prepare.RefundAmountCents
	if prepare.NetAmountCents < 0 {
		prepare.NetAmountCents = 0
	}
	return prepare, nil
}

func (s *AdminService) CreateMerchantSettlement(merchantID uint, req dto.CreateMerchantSettlementRequest) (*model.MerchantSettlement, []model.Order, error) {
	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, nil, errors.New("商家不存在")
	}
	remark := strings.TrimSpace(req.Remark)
	var settlement model.MerchantSettlement
	var settledOrders []model.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		orders, err := s.unsettledMerchantOrdersTx(tx, merchantID)
		if err != nil {
			return err
		}
		if len(orders) == 0 {
			return errors.New("暂无可结算订单")
		}
		var total, refunded int64
		var periodStart, periodEnd *time.Time
		orderIDs := make([]uint, 0, len(orders))
		for i := range orders {
			order := orders[i]
			orderIDs = append(orderIDs, order.ID)
			total += settlementOrderAmount(order)
			refunded += order.RefundedAmount
			if periodStart == nil || order.CreatedAt.Before(*periodStart) {
				t := order.CreatedAt
				periodStart = &t
			}
			if periodEnd == nil || order.CreatedAt.After(*periodEnd) {
				t := order.CreatedAt
				periodEnd = &t
			}
		}
		net := total - refunded
		if net < 0 {
			net = 0
		}
		settlement = model.MerchantSettlement{
			MerchantID:            merchantID,
			SettlementPeriodStart: periodStart,
			SettlementPeriodEnd:   periodEnd,
			OrderCount:            len(orders),
			TotalAmountCents:      total,
			RefundAmountCents:     refunded,
			NetAmountCents:        net,
			Status:                "pending",
			Remark:                remark,
		}
		if err := tx.Create(&settlement).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Order{}).Where("id IN ?", orderIDs).Update("settlement_id", settlement.ID).Error; err != nil {
			return err
		}
		settledOrders = orders
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &settlement, settledOrders, nil
}

func (s *AdminService) ListMerchantSettlements(merchantID uint) ([]model.MerchantSettlement, error) {
	var list []model.MerchantSettlement
	err := s.db.Where("merchant_id = ?", merchantID).Order("id desc").Find(&list).Error
	return list, err
}

func (s *AdminService) GetMerchantSettlement(id uint) (*model.MerchantSettlement, []model.Order, error) {
	var settlement model.MerchantSettlement
	if err := s.db.Preload("Merchant").First(&settlement, id).Error; err != nil {
		return nil, nil, err
	}
	var orders []model.Order
	err := s.db.Preload("Store").
		Where("settlement_id = ?", settlement.ID).
		Order("created_at desc").
		Find(&orders).Error
	return &settlement, orders, err
}

func (s *AdminService) MarkMerchantSettlementPaid(id uint, req dto.MarkMerchantSettlementPaidRequest) (*model.MerchantSettlement, error) {
	var settlement model.MerchantSettlement
	if err := s.db.First(&settlement, id).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	settlement.Status = "paid"
	settlement.PaidAt = &now
	if remark := strings.TrimSpace(req.Remark); remark != "" {
		settlement.Remark = remark
	}
	if err := s.db.Save(&settlement).Error; err != nil {
		return nil, err
	}
	return &settlement, nil
}

func (s *AdminService) unsettledMerchantOrders(merchantID uint) ([]model.Order, error) {
	return s.unsettledMerchantOrdersTx(s.db, merchantID)
}

func (s *AdminService) unsettledMerchantOrdersTx(tx *gorm.DB, merchantID uint) ([]model.Order, error) {
	var orders []model.Order
	err := tx.Preload("Store").
		Where("merchant_id = ? AND order_type = ? AND settlement_id IS NULL", merchantID, "store_order").
		Where("status IN ?", []string{"accepted", "completed"}).
		Order("created_at asc").
		Find(&orders).Error
	return orders, err
}

func settlementOrderAmount(order model.Order) int64 {
	if order.TotalAmount > 0 {
		return order.TotalAmount
	}
	return order.Amount
}

func (s *AdminService) ListMerchantStores(merchantID uint, page, pageSize int) ([]model.Store, int64, error) {
	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, 0, errors.New("商家不存在")
	}
	return s.stores.ListByMerchant(merchantID, (page-1)*pageSize, pageSize)
}

func (s *AdminService) ListMerchantLeads(merchantID uint, page, pageSize int) ([]model.CustomerLead, int64, error) {
	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, 0, errors.New("\u5546\u5bb6\u4e0d\u5b58\u5728")
	}
	return s.leads.ListByMerchant(merchantID, (page-1)*pageSize, pageSize)
}

func (s *AdminService) MerchantLeadStats(merchantID uint) (map[string]int64, error) {
	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, errors.New("\u5546\u5bb6\u4e0d\u5b58\u5728")
	}
	return s.leads.CountByStatus(merchantID)
}

func (s *AdminService) GenerateMerchantInsights(merchantID uint) (map[string]interface{}, error) {
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, errors.New("\u5546\u5bb6\u4e0d\u5b58\u5728")
	}

	stores, storeTotal, err := s.stores.ListByMerchant(merchantID, 0, 100)
	if err != nil {
		return nil, err
	}
	stats, err := s.leads.CountByStatus(merchantID)
	if err != nil {
		return nil, err
	}
	orderStats := s.merchantOrderStats(merchantID)
	hotProducts := s.hotProducts(merchantID, true)
	slowProducts := s.hotProducts(merchantID, false)
	customerProfiles, err := s.ListMerchantCustomerProfiles(merchantID, 8)
	if err != nil {
		return nil, err
	}
	customerSummary := summarizeCustomerProfiles(customerProfiles)

	activeStores := int64(0)
	for _, store := range stores {
		if store.Status == "active" {
			activeStores++
		}
	}

	totalLeads := stats["new"] + stats["contacted"] + stats["converted"] + stats["invalid"]
	conversionRate := 0.0
	if totalLeads > 0 {
		conversionRate = float64(stats["converted"]) / float64(totalLeads)
	}

	insights := make([]map[string]string, 0, 5)
	if merchant.Status != "active" {
		insights = append(insights, map[string]string{
			"type":    "warning",
			"title":   "\u5148\u5b8c\u6210\u5546\u5bb6\u542f\u7528",
			"content": "\u5f53\u524d\u5546\u5bb6\u5c1a\u672a\u542f\u7528\uff0c\u987e\u5ba2\u7ebf\u7d22\u4f1a\u53d7\u9650\u3002\u5efa\u8bae\u5148\u5b8c\u6210\u5ba1\u6838\u548c\u670d\u52a1\u5f00\u901a\u3002",
		})
	}
	if merchant.SubscriptionStatus != "active" {
		insights = append(insights, map[string]string{
			"type":    "warning",
			"title":   "\u5efa\u8bae\u5f00\u901a\u5546\u5bb6\u670d\u52a1",
			"content": "\u8be5\u5546\u5bb6\u6682\u672a\u5904\u4e8e\u670d\u52a1\u4e2d\uff0c\u540e\u7eed AI \u8fd0\u8425\u3001\u7ebf\u7d22\u5206\u6790\u7b49\u80fd\u529b\u53ef\u4e0e\u5957\u9910\u7ed1\u5b9a\u3002",
		})
	}
	if storeTotal == 0 || activeStores == 0 {
		insights = append(insights, map[string]string{
			"type":    "warning",
			"title":   "\u5148\u5efa\u7acb\u95e8\u5e97\u5165\u53e3",
			"content": "\u5f53\u524d\u6ca1\u6709\u53ef\u7528\u95e8\u5e97\u5165\u53e3\uff0c\u5efa\u8bae\u5148\u4e3a\u5546\u5bb6\u521b\u5efa\u5e76\u542f\u7528\u95e8\u5e97\uff0c\u518d\u63a8\u5e7f\u987e\u5ba2\u626b\u7801\u94fe\u63a5\u3002",
		})
	}
	if totalLeads == 0 {
		insights = append(insights, map[string]string{
			"type":    "info",
			"title":   "\u5efa\u8bae\u5f3a\u5316\u626b\u7801\u8f6c\u5316",
			"content": "\u76ee\u524d\u8fd8\u6ca1\u6709\u987e\u5ba2\u7ebf\u7d22\uff0c\u53ef\u5728\u95e8\u5e97\u9875\u589e\u52a0\u4f18\u60e0\u3001\u9884\u7ea6\u6216\u8fdb\u5e97\u798f\u5229\u6587\u6848\u3002",
		})
	} else {
		if stats["new"] > 0 {
			insights = append(insights, map[string]string{
				"type":    "action",
				"title":   "\u4f18\u5148\u5904\u7406\u65b0\u7ebf\u7d22",
				"content": "\u5f53\u524d\u8fd8\u6709\u65b0\u7ebf\u7d22\u672a\u8ddf\u8fdb\uff0c\u5efa\u8bae\u5728 24 \u5c0f\u65f6\u5185\u8054\u7cfb\uff0c\u907f\u514d\u610f\u5411\u964d\u4f4e\u3002",
			})
		}
		if stats["contacted"] > stats["converted"] {
			insights = append(insights, map[string]string{
				"type":    "action",
				"title":   "\u8ddf\u8fdb\u5df2\u8054\u7cfb\u5ba2\u6237",
				"content": "\u5df2\u8054\u7cfb\u7ebf\u7d22\u591a\u4e8e\u5df2\u6210\u4ea4\u7ebf\u7d22\uff0c\u5efa\u8bae\u751f\u6210\u4e8c\u6b21\u8ddf\u8fdb\u8bdd\u672f\u6216\u63d0\u4f9b\u9650\u65f6\u4f18\u60e0\u3002",
			})
		}
		if conversionRate >= 0.3 {
			insights = append(insights, map[string]string{
				"type":    "success",
				"title":   "\u7ebf\u7d22\u8f6c\u5316\u8868\u73b0\u4e0d\u9519",
				"content": "\u5f53\u524d\u6210\u4ea4\u5360\u6bd4\u8f83\u597d\uff0c\u5efa\u8bae\u590d\u7528\u73b0\u6709\u95e8\u5e97\u9875\u6587\u6848\u548c\u8ddf\u8fdb\u65b9\u5f0f\u3002",
			})
		}
	}
	if orderStats["order_count"] > 0 {
		cancelRate := float64(orderStats["closed_count"]) / float64(orderStats["order_count"])
		if cancelRate >= 0.3 {
			insights = append(insights, map[string]string{
				"type":    "warning",
				"title":   "订单取消率偏高",
				"content": "近期取消订单占比较高，建议检查商品库存、营业时间和接单响应速度，必要时先暂停接单避免继续损失体验。",
			})
		}
		if len(hotProducts) > 0 {
			insights = append(insights, map[string]string{
				"type":    "success",
				"title":   "发现热销商品",
				"content": "热销商品可以放到分类靠前位置，并搭配满减活动提升客单价。",
			})
		}
		if len(slowProducts) > 0 {
			insights = append(insights, map[string]string{
				"type":    "action",
				"title":   "低动销商品需要优化",
				"content": "部分商品近期销量偏低，可尝试改标题、换图片，或创建折扣活动进行测试。",
			})
		}
	}
	if customerSummary["sleeping"] > 0 || customerSummary["risk"] > 0 {
		insights = append(insights, map[string]string{
			"type":    "action",
			"title":   "优先召回风险顾客",
			"content": "已有顾客出现沉睡或订单关闭信号，建议创建低门槛优惠券，并结合热销商品进行召回。",
		})
	}
	if customerSummary["high_value"] > 0 {
		insights = append(insights, map[string]string{
			"type":    "success",
			"title":   "维护高价值顾客",
			"content": "发现高价值顾客，可设置专属折扣、生日权益或新品优先通知，提升长期复购。",
		})
	}

	if len(insights) == 0 {
		insights = append(insights, map[string]string{
			"type":    "info",
			"title":   "\u8fd0\u8425\u72b6\u6001\u7a33\u5b9a",
			"content": "\u5f53\u524d\u6570\u636e\u6682\u65e0\u660e\u663e\u98ce\u9669\uff0c\u5efa\u8bae\u7ee7\u7eed\u89c2\u5bdf\u7ebf\u7d22\u589e\u957f\u548c\u8f6c\u5316\u7387\u3002",
		})
	}

	return map[string]interface{}{
		"summary": map[string]interface{}{
			"total_leads":     totalLeads,
			"active_stores":   activeStores,
			"conversion_rate": conversionRate,
		},
		"lead_stats":        stats,
		"order_stats":       orderStats,
		"hot_products":      hotProducts,
		"slow_products":     slowProducts,
		"customer_profiles": customerProfiles,
		"customer_summary":  customerSummary,
		"insights":          insights,
		"engine":            "order_rule_based_v2",
	}, nil
}

func summarizeCustomerProfiles(rows []CustomerProfileRow) map[string]int64 {
	result := map[string]int64{
		"total":      int64(len(rows)),
		"new":        0,
		"repeat":     0,
		"high_value": 0,
		"sleeping":   0,
		"risk":       0,
	}
	for _, row := range rows {
		switch row.AITag {
		case "新顾客":
			result["new"]++
		case "复购顾客":
			result["repeat"]++
		case "高价值顾客":
			result["high_value"]++
		case "沉睡顾客":
			result["sleeping"]++
		case "流失风险":
			result["risk"]++
		}
	}
	return result
}

func (s *AdminService) merchantOrderStats(merchantID uint) map[string]int64 {
	var rows []struct {
		Status string
		Count  int64
		Amount int64
	}
	s.db.Model(&model.Order{}).
		Select("status, COUNT(*) AS count, COALESCE(SUM(total_amount),0) AS amount").
		Where("merchant_id = ? AND order_type = ?", merchantID, "store_order").
		Group("status").
		Scan(&rows)

	result := map[string]int64{"order_count": 0, "trade_amount": 0, "closed_count": 0, "completed_count": 0, "accepted_count": 0}
	for _, row := range rows {
		result["order_count"] += row.Count
		result["trade_amount"] += row.Amount
		if row.Status == "closed" {
			result["closed_count"] = row.Count
		}
		if row.Status == "completed" {
			result["completed_count"] = row.Count
		}
		if row.Status == "accepted" || row.Status == "completed" {
			result["accepted_count"] += row.Count
		}
	}
	return result
}

func (s *AdminService) hotProducts(merchantID uint, desc bool) []map[string]interface{} {
	var orders []model.Order
	if err := s.db.Where("merchant_id = ? AND order_type = ? AND status IN ?", merchantID, "store_order", []string{"received", "accepted", "completed"}).Find(&orders).Error; err != nil {
		return []map[string]interface{}{}
	}
	type productAgg struct {
		Name     string
		Quantity int
		Amount   int64
	}
	stats := map[string]*productAgg{}
	for _, order := range orders {
		var items []StoreOrderItemSnapshot
		if err := json.Unmarshal([]byte(order.Items), &items); err != nil {
			continue
		}
		for _, item := range items {
			key := item.Name
			if stats[key] == nil {
				stats[key] = &productAgg{Name: item.Name}
			}
			stats[key].Quantity += item.Quantity
			stats[key].Amount += item.LineAmount
		}
	}
	rows := make([]map[string]interface{}, 0, len(stats))
	for _, item := range stats {
		rows = append(rows, map[string]interface{}{"name": item.Name, "quantity": item.Quantity, "amount": item.Amount})
	}
	sort.Slice(rows, func(i, j int) bool {
		if desc {
			return rows[i]["quantity"].(int) > rows[j]["quantity"].(int)
		}
		return rows[i]["quantity"].(int) < rows[j]["quantity"].(int)
	})
	if len(rows) > 5 {
		return rows[:5]
	}
	return rows
}

func (s *AdminService) UpdateMerchantLead(merchantID, leadID uint, req dto.UpdateCustomerLeadRequest) (*model.CustomerLead, error) {
	status := strings.TrimSpace(req.Status)
	note := strings.TrimSpace(req.FollowUpNote)
	if status != "new" && status != "contacted" && status != "converted" && status != "invalid" {
		return nil, errors.New("\u7ebf\u7d22\u72b6\u6001\u65e0\u6548")
	}
	if len(note) > 500 {
		return nil, errors.New("\u8ddf\u8fdb\u5907\u6ce8\u4e0d\u80fd\u8d85\u8fc7 500 \u5b57")
	}

	lead, err := s.leads.FindByMerchantAndID(merchantID, leadID)
	if err != nil {
		return nil, errors.New("\u7ebf\u7d22\u4e0d\u5b58\u5728")
	}
	lead.Status = status
	lead.FollowUpNote = note
	if err := s.leads.Save(lead); err != nil {
		return nil, err
	}
	return lead, nil
}

func (s *AdminService) SaveMerchantStore(merchantID, storeID uint, req dto.AdminSaveStoreRequest) (*model.Store, error) {
	name := strings.TrimSpace(req.Name)
	address := strings.TrimSpace(req.Address)
	phone := strings.TrimSpace(req.ContactPhone)
	status := strings.TrimSpace(req.Status)
	businessHours := strings.TrimSpace(req.BusinessHours)
	pauseReason := strings.TrimSpace(req.PauseReason)
	isOpen := true
	if req.IsOpen != nil {
		isOpen = *req.IsOpen
	}

	if name == "" {
		return nil, errors.New("门店名称不能为空")
	}
	if !isValidAdminManagedPhone(phone) {
		return nil, errors.New("门店联系电话必须为 11 位手机号")
	}
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "inactive" {
		return nil, errors.New("门店状态无效")
	}
	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, errors.New("商家不存在")
	}

	if storeID == 0 {
		if _, err := s.stores.FindByMerchantAndName(merchantID, name); err == nil {
			return nil, errors.New("同一商家下门店名称不能重复")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		store := &model.Store{
			MerchantID:    merchantID,
			Name:          name,
			Address:       address,
			ContactPhone:  phone,
			Status:        status,
			IsOpen:        isOpen,
			BusinessHours: businessHours,
			PauseReason:   pauseReason,
		}
		if err := s.stores.Create(store); err != nil {
			return nil, err
		}
		return store, nil
	}

	store, err := s.stores.FindByMerchantAndID(merchantID, storeID)
	if err != nil {
		return nil, errors.New("门店不存在")
	}
	if store.Name != name {
		if existing, err := s.stores.FindByMerchantAndName(merchantID, name); err == nil && existing.ID != storeID {
			return nil, errors.New("同一商家下门店名称不能重复")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	store.Name = name
	store.Address = address
	store.ContactPhone = phone
	store.Status = status
	store.BusinessHours = businessHours
	store.PauseReason = pauseReason
	if req.IsOpen != nil {
		store.IsOpen = *req.IsOpen
	}
	if err := s.stores.Save(store); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *AdminService) DisableMerchantStore(merchantID, storeID uint) error {
	store, err := s.stores.FindByMerchantAndID(merchantID, storeID)
	if err != nil {
		return errors.New("门店不存在")
	}
	store.Status = "inactive"
	return s.stores.Save(store)
}

func (s *AdminService) UpdateUserStatus(id uint, status string) error {
	user, err := s.users.FindByID(id)
	if err != nil {
		return err
	}
	user.Status = status
	return s.users.Save(user)
}

func (s *AdminService) ManualOpenMember(id, packageID uint) error {
	user, err := s.users.FindByID(id)
	if err != nil {
		return err
	}
	if packageID == 0 {
		return errors.New("package_id is required")
	}
	pkg, err := s.packages.FindByID(packageID)
	if err != nil {
		return err
	}
	orderService := NewOrderService(s.db)
	return orderService.applyPackageToUser(user, pkg, false)
}

func (s *AdminService) AdjustUserMember(id uint, req dto.ManualOpenMemberRequest) (*model.User, error) {
	user, err := s.users.FindByID(id)
	if err != nil {
		return nil, err
	}

	if req.PackageID > 0 {
		pkg, err := s.packages.FindByID(req.PackageID)
		if err != nil {
			return nil, err
		}
		orderService := NewOrderService(s.db)
		if err := orderService.applyPackageToUser(user, pkg, req.AutoRenew); err != nil {
			return nil, err
		}
		user, err = s.users.FindByID(id)
		if err != nil {
			return nil, err
		}
	}

	level := strings.TrimSpace(req.MemberLevel)
	if level != "" {
		user.MemberLevel = level
	}
	if req.DurationDays > 0 {
		now := time.Now()
		base := now
		if user.ExpiredAt != nil && user.ExpiredAt.After(now) {
			base = *user.ExpiredAt
		}
		expiredAt := base.AddDate(0, 0, req.DurationDays)
		user.ExpiredAt = &expiredAt
	}
	if strings.TrimSpace(req.ExpiredAt) != "" {
		expiredAt, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(req.ExpiredAt), time.Local)
		if err != nil {
			return nil, errors.New("expired_at format must be YYYY-MM-DD")
		}
		expiredAt = time.Date(expiredAt.Year(), expiredAt.Month(), expiredAt.Day(), 23, 59, 59, 0, time.Local)
		user.ExpiredAt = &expiredAt
	}
	if req.QuotaDelta != 0 {
		user.RemainingQuota += req.QuotaDelta
		if user.RemainingQuota < 0 {
			user.RemainingQuota = 0
		}
	}
	user.AutoRenew = req.AutoRenew

	if err := s.users.Save(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AdminService) UpdateUserPhone(id uint, phone string) error {
	phone = strings.TrimSpace(phone)
	if !isValidAdminManagedPhone(phone) {
		return errors.New("phone must be 11 digits")
	}

	user, err := s.users.FindByID(id)
	if err != nil {
		return err
	}

	if user.Phone != phone {
		existing, err := s.users.FindByPhone(phone)
		if err == nil && existing.ID != id {
			return errors.New("phone already in use")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	user.Phone = phone
	return s.users.Save(user)
}

func (s *AdminService) GenerateCards(req dto.BatchCardRequest) ([]model.CardCode, error) {
	pkg, err := s.packages.FindByID(req.PackageID)
	if err != nil {
		return nil, err
	}

	cards := make([]model.CardCode, 0, req.Count)
	batchNo := time.Now().Format("20060102150405")
	for i := 0; i < req.Count; i++ {
		random := make([]byte, 8)
		_, _ = rand.Read(random)
		cards = append(cards, model.CardCode{
			BatchNo:   "BATCH" + batchNo,
			Code:      "CARD-" + hex.EncodeToString(random),
			PackageID: pkg.ID,
			Quota:     req.Quota,
			Status:    "unused",
		})
	}

	return cards, s.cards.CreateBatch(cards)
}

func (s *AdminService) SaveSystemConfig(req dto.SaveSystemConfigRequest) error {
	if err := s.systems.Upsert("site_name", req.SiteName, false); err != nil {
		return err
	}

	encrypted, err := utils.EncryptString(s.cfg.AESSecret, req.PaymentGateway)
	if err != nil {
		return err
	}

	if err := s.systems.Upsert("payment_gateway", encrypted, true); err != nil {
		return err
	}
	if err := s.systems.Upsert("platform_alipay_qr_code", strings.TrimSpace(req.PlatformAlipayQRCode), false); err != nil {
		return err
	}
	if err := s.systems.Upsert("platform_wechat_qr_code", strings.TrimSpace(req.PlatformWechatQRCode), false); err != nil {
		return err
	}
	if err := s.systems.Upsert("platform_subscription_note", strings.TrimSpace(req.PlatformSubscriptionNote), false); err != nil {
		return err
	}
	if err := s.systems.Upsert("filing_info", req.FilingInfo, false); err != nil {
		return err
	}
	return s.systems.Upsert("notice", req.Notice, false)
}

func (s *AdminService) ListCards() ([]model.CardCode, error) {
	return s.cards.ListAll()
}

func (s *AdminService) ListSystemConfigs() ([]model.SystemConfig, error) {
	list, err := s.systems.List()
	if err != nil {
		return nil, err
	}

	for idx := range list {
		if list[idx].IsEncrypted && list[idx].ConfigValue != "" {
			decrypted, decryptErr := utils.DecryptString(s.cfg.AESSecret, list[idx].ConfigValue)
			if decryptErr == nil {
				list[idx].ConfigValue = decrypted
			}
		}
	}

	return list, nil
}

func (s *AdminService) SecurityCheck() SecurityCheckReport {
	items := []SecurityCheckItem{
		s.checkAdminPassword(),
		s.checkProductionEnv(),
		s.checkSecrets(),
		s.checkDatabaseConfig(),
		s.checkAlipayConfig(),
		s.checkAIConfig(),
		s.checkBackupScripts(),
		s.checkRateLimit(),
		s.checkCORS(),
		s.checkBootstrapSafety(),
		s.checkLogging(),
	}
	healthSections := s.operationHealthSections()
	overall := "pass"
	for _, item := range items {
		if item.Status == "danger" {
			overall = "danger"
			break
		}
		if item.Status == "warning" && overall != "danger" {
			overall = "warning"
		}
	}
	for _, section := range healthSections {
		if section.Status == "danger" {
			overall = "danger"
			break
		}
		if section.Status == "warning" && overall != "danger" {
			overall = "warning"
		}
	}
	return SecurityCheckReport{
		OverallStatus:  overall,
		GeneratedAt:    time.Now(),
		Environment:    s.cfg.AppEnv,
		Items:          items,
		HealthSections: healthSections,
	}
}

func (s *AdminService) AIConfigStatus() AIConfigStatus {
	provider := strings.TrimSpace(s.cfg.AIProvider)
	if provider == "" {
		provider = "template"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.AIBaseURL), "/")
	modelName := strings.TrimSpace(s.cfg.AIModel)
	hasKey := strings.TrimSpace(s.cfg.AIAPIKey) != ""
	ready := s.cfg.AIEnabled && baseURL != "" && modelName != "" && hasKey
	status := "warning"
	summary := "AI 当前使用模板兜底，商家端功能可演示但不会调用真实模型。"
	suggestion := "准备好 API Key、额度和计费预警后，再开启 AI_ENABLED=true。"
	if s.cfg.AIEnabled && !ready {
		status = "danger"
		summary = "AI 已开启，但供应商地址、模型或 API Key 配置不完整。"
		suggestion = "补齐 AI_BASE_URL、AI_API_KEY、AI_MODEL 后重启后端，并先生成一次经营建议测试。"
	}
	if ready {
		status = "pass"
		summary = fmt.Sprintf("AI 已配置为 %s / %s，可用于商家端经营建议。", provider, modelName)
		suggestion = "建议继续接入调用日志、套餐额度、缓存和成本预警。"
	}
	return AIConfigStatus{
		Enabled:        s.cfg.AIEnabled,
		Provider:       provider,
		BaseURL:        baseURL,
		Model:          modelName,
		TimeoutSeconds: s.cfg.AITimeoutSeconds,
		HasAPIKey:      hasKey,
		Ready:          ready,
		Status:         status,
		Summary:        summary,
		Suggestion:     suggestion,
		EnvExample:     aiEnvExample(provider, baseURL, modelName),
		Presets:        aiProviderPresets(),
	}
}

func (s *AdminService) AIUsageOverview() (AIUsageOverview, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	base := s.db.Model(&model.MerchantAIUsageLog{}).Where("used_at >= ? AND used_at < ?", start, end)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return AIUsageOverview{}, err
	}
	var success int64
	if err := base.Where("success = ?", true).Count(&success).Error; err != nil {
		return AIUsageOverview{}, err
	}
	var failed int64
	if err := base.Where("success = ?", false).Count(&failed).Error; err != nil {
		return AIUsageOverview{}, err
	}
	var fallback int64
	if err := base.Where("fallback = ?", true).Count(&fallback).Error; err != nil {
		return AIUsageOverview{}, err
	}
	var activeMerchants int64
	if err := base.Distinct("merchant_id").Count(&activeMerchants).Error; err != nil {
		return AIUsageOverview{}, err
	}
	var avgLatency struct {
		Value *float64
	}
	if err := base.Select("AVG(duration_ms) AS value").Scan(&avgLatency).Error; err != nil {
		return AIUsageOverview{}, err
	}
	var scenarios []AIUsageScenarioStat
	if err := base.Select("scenario, COUNT(*) AS count").
		Group("scenario").
		Order("count DESC").
		Limit(5).
		Scan(&scenarios).Error; err != nil {
		return AIUsageOverview{}, err
	}
	failureRate := int64(0)
	if total > 0 {
		failureRate = failed * 100 / total
	}
	latency := int64(0)
	if avgLatency.Value != nil {
		latency = int64(*avgLatency.Value)
	}
	return AIUsageOverview{
		TodayTotal:      total,
		TodaySuccess:    success,
		TodayFailed:     failed,
		TodayFallback:   fallback,
		ActiveMerchants: activeMerchants,
		AverageLatency:  latency,
		FailureRate:     failureRate,
		TopScenarios:    scenarios,
		Suggestion:      aiUsageSuggestion(total, failed, fallback),
	}, nil
}

func (s *AdminService) operationHealthSections() []OperationHealthBlock {
	return []OperationHealthBlock{
		s.transactionHealthBlock(),
		s.paymentRefundHealthBlock(),
		s.settlementHealthBlock(),
		s.merchantOpsHealthBlock(),
		s.productReadinessHealthBlock(),
		s.deploymentReadinessHealthBlock(),
	}
}

func (s *AdminService) transactionHealthBlock() OperationHealthBlock {
	pending := s.countOrders("order_type = ? AND status = ?", "store_order", "pending")
	failed := s.countOrders("order_type = ? AND status = ?", "store_order", "failed")
	paymentConfirming := s.countOrders("order_type = ? AND status = ?", "store_order", "payment_confirming")
	received := s.countOrders("order_type = ? AND status = ?", "store_order", "received")
	staleAccepted := s.countOrders("order_type = ? AND status = ? AND updated_at < ?", "store_order", "accepted", time.Now().Add(-2*time.Hour))
	status := healthStatus(paymentConfirming+received+staleAccepted, pending+failed)
	return OperationHealthBlock{
		Key:        "transaction",
		Title:      "交易运行风险",
		Status:     status,
		Summary:    fmt.Sprintf("待支付 %d 单，支付失败 %d 单，待确认收款 %d 单，待接单 %d 单，长时间履约中 %d 单。", pending, failed, paymentConfirming, received, staleAccepted),
		Suggestion: "优先处理待确认收款、已支付未接单和长时间未完成订单；支付失败订单只做核对和顾客引导。",
		ActionText: "进入订单异常",
		ActionPath: "/admin/orders?exception=all",
		Metrics: []OperationHealthMetric{
			healthMetric("待支付", pending, "顾客尚未完成付款", warningIfPositive(pending)),
			healthMetric("支付失败", failed, "需要核对支付或引导重试", warningIfPositive(failed)),
			healthMetric("待确认收款", paymentConfirming, "顾客已标记付款，商家需要核对到账", dangerIfPositive(paymentConfirming)),
			healthMetric("待接单", received, "已付款，商家需要处理", dangerIfPositive(received)),
			healthMetric("履约超时", staleAccepted, "已接单超过 2 小时未完成", dangerIfPositive(staleAccepted)),
		},
	}
}

func (s *AdminService) paymentRefundHealthBlock() OperationHealthBlock {
	refundsToday := s.countRefunds("created_at >= ?", beginningOfDay(time.Now()))
	refundsWithoutReason := s.countRefunds("reason = '' OR reason IS NULL")
	refundedOrders := s.countOrders("order_type = ? AND refunded_amount > 0", "store_order")
	status := healthStatus(refundsWithoutReason, refundsToday+refundedOrders)
	return OperationHealthBlock{
		Key:        "payment_refund",
		Title:      "支付退款风险",
		Status:     status,
		Summary:    fmt.Sprintf("今日退款记录 %d 条，累计有退款订单 %d 单，缺少退款原因 %d 条。", refundsToday, refundedOrders, refundsWithoutReason),
		Suggestion: "退款要保留原因和操作记录；真实支付接入后，需要同步核对支付渠道退款结果。",
		ActionText: "查看退款售后",
		ActionPath: "/admin/orders?exception=refund",
		Metrics: []OperationHealthMetric{
			healthMetric("今日退款", refundsToday, "当天产生的退款记录", warningIfPositive(refundsToday)),
			healthMetric("退款订单", refundedOrders, "已有退款金额的订单", warningIfPositive(refundedOrders)),
			healthMetric("缺少原因", refundsWithoutReason, "退款记录原因为空", dangerIfPositive(refundsWithoutReason)),
			healthMetric("支付配置", boolToCount(s.cfg.AlipaySandbox), "仍处于支付宝沙箱会影响真实收款", warningIf(s.cfg.AlipaySandbox)),
		},
	}
}

func (s *AdminService) settlementHealthBlock() OperationHealthBlock {
	pendingSettlements := s.countRows(&model.MerchantSettlement{}, "status = ?", "pending")
	overdueSettlements := s.countRows(&model.MerchantSettlement{}, "status = ? AND created_at < ?", "pending", time.Now().AddDate(0, 0, -7))
	unsettledOrders := s.countOrders("order_type = ? AND settlement_id IS NULL AND status IN ?", "store_order", []string{"received", "accepted", "completed"})
	paymentConfigPending := s.countRows(&model.MerchantPaymentConfig{}, "audit_status = ?", "pending")
	status := healthStatus(overdueSettlements+paymentConfigPending, pendingSettlements+unsettledOrders)
	return OperationHealthBlock{
		Key:        "settlement",
		Title:      "结算风险",
		Status:     status,
		Summary:    fmt.Sprintf("待结算单 %d 个，超过 7 天未处理 %d 个，未入结算订单 %d 单。", pendingSettlements, overdueSettlements, unsettledOrders),
		Suggestion: "优先核对待结算和商家收款配置，避免人工打款遗漏或退款后结算口径不一致。",
		ActionText: "进入商家运营",
		ActionPath: "/admin/merchants?risk=settlement",
		Metrics: []OperationHealthMetric{
			healthMetric("待结算单", pendingSettlements, "平台已生成但未标记打款", warningIfPositive(pendingSettlements)),
			healthMetric("结算超期", overdueSettlements, "超过 7 天未完成", dangerIfPositive(overdueSettlements)),
			healthMetric("未入结算", unsettledOrders, "已支付订单还未纳入结算单", warningIfPositive(unsettledOrders)),
			healthMetric("收款待审", paymentConfigPending, "商家收款配置等待审核", dangerIfPositive(paymentConfigPending)),
		},
	}
}

func (s *AdminService) merchantOpsHealthBlock() OperationHealthBlock {
	suspended := s.countRows(&model.Merchant{}, "status = ?", "suspended")
	pendingMerchants := s.countRows(&model.Merchant{}, "status = ?", "pending")
	openFollowUps := s.countRows(&model.MerchantFollowUp{}, "status = ?", "open")
	expiringSubscriptions := s.countRows(&model.Merchant{}, "subscription_status = ? AND subscription_expire_at BETWEEN ? AND ?", "active", time.Now(), time.Now().AddDate(0, 0, 7))
	status := healthStatus(suspended+openFollowUps, pendingMerchants+expiringSubscriptions)
	return OperationHealthBlock{
		Key:        "merchant_ops",
		Title:      "商家运营风险",
		Status:     status,
		Summary:    fmt.Sprintf("冻结商家 %d 个，待审核商家 %d 个，未处理跟进 %d 条，7 天内到期 %d 个。", suspended, pendingMerchants, openFollowUps, expiringSubscriptions),
		Suggestion: "商家明细仍在商家运营页处理；健康中心只提示优先级和跳转入口。",
		ActionText: "查看商家运营",
		ActionPath: "/admin/merchants",
		Metrics: []OperationHealthMetric{
			healthMetric("冻结商家", suspended, "可能影响门店营业", dangerIfPositive(suspended)),
			healthMetric("待审核", pendingMerchants, "新商家等待平台处理", warningIfPositive(pendingMerchants)),
			healthMetric("未处理跟进", openFollowUps, "平台运营事项未关闭", dangerIfPositive(openFollowUps)),
			healthMetric("订阅将到期", expiringSubscriptions, "7 天内到期商家", warningIfPositive(expiringSubscriptions)),
		},
	}
}

func (s *AdminService) productReadinessHealthBlock() OperationHealthBlock {
	activeStores := s.countRows(&model.Store{}, "status = ? AND is_open = ?", "active", true)
	pausedStores := s.countRows(&model.Store{}, "status <> ? OR is_open = ?", "active", false)
	activeProducts := s.countRows(&model.StoreProduct{}, "status = ?", "active")
	noImageProducts := s.countRows(&model.StoreProduct{}, "status = ? AND (image_url = '' OR image_url IS NULL)", "active")
	soldOutProducts := s.countRows(&model.StoreProduct{}, "status = ? AND stock = 0", "active")
	status := healthStatus(boolToCount(activeStores == 0 || activeProducts == 0), pausedStores+noImageProducts+soldOutProducts)
	return OperationHealthBlock{
		Key:        "product_readiness",
		Title:      "顾客下单与商品准备",
		Status:     status,
		Summary:    fmt.Sprintf("营业门店 %d 个，暂停/不可用门店 %d 个，上架商品 %d 个，无图商品 %d 个，售罄商品 %d 个。", activeStores, pausedStores, activeProducts, noImageProducts, soldOutProducts),
		Suggestion: "真实推广前要确保至少有可营业门店、可售商品和清晰库存；无图或售罄商品不阻塞上线，但会影响转化。",
		ActionText: "进入商家商品",
		ActionPath: "/admin/merchants",
		Metrics: []OperationHealthMetric{
			healthMetric("营业门店", activeStores, "顾客可扫码下单的门店", dangerIfZero(activeStores)),
			healthMetric("暂停门店", pausedStores, "暂停接单或非 active 门店", warningIfPositive(pausedStores)),
			healthMetric("上架商品", activeProducts, "顾客端可见商品", dangerIfZero(activeProducts)),
			healthMetric("无图商品", noImageProducts, "影响顾客端转化", warningIfPositive(noImageProducts)),
			healthMetric("售罄商品", soldOutProducts, "库存为 0 的上架商品", warningIfPositive(soldOutProducts)),
		},
	}
}

func (s *AdminService) deploymentReadinessHealthBlock() OperationHealthBlock {
	configWarnings := int64(0)
	if !strings.EqualFold(s.cfg.AppEnv, "production") {
		configWarnings++
	}
	if strings.Contains(strings.ToLower(s.cfg.FrontendURL), "localhost") || strings.Contains(strings.ToLower(s.cfg.AppURL), "localhost") {
		configWarnings++
	}
	if s.cfg.JWTSecret == "replace-with-a-very-strong-secret" || s.cfg.AESSecret == "0123456789abcdef0123456789abcdef" {
		configWarnings++
	}
	migrationFiles := countMigrationFiles()
	backupScripts := boolToCount(fileExists("scripts/backup-mysql.ps1") && fileExists("scripts/backup-mysql.sh"))
	status := healthStatus(configWarnings, boolToCount(backupScripts == 0))
	return OperationHealthBlock{
		Key:        "deployment",
		Title:      "部署与迁移准备",
		Status:     status,
		Summary:    fmt.Sprintf("生产配置风险 %d 项，迁移 SQL %d 个，备份脚本状态 %d。", configWarnings, migrationFiles, backupScripts),
		Suggestion: "上线前按 docs/production-runbook.md 确认环境变量、迁移顺序、备份任务和回滚流程；后续建议接入专门迁移工具。",
		ActionText: "查看配置检查",
		ActionPath: "/admin/system",
		Metrics: []OperationHealthMetric{
			healthMetric("配置风险", configWarnings, "APP_ENV、域名或密钥仍偏开发态", dangerIfPositive(configWarnings)),
			healthMetric("迁移文件", migrationFiles, "sql/migrations 与 migrations 下的升级脚本", "pass"),
			healthMetric("备份脚本", backupScripts, "1 表示 Windows/Linux 脚本齐全", dangerIfZero(backupScripts)),
		},
	}
}

func (s *AdminService) checkAdminPassword() SecurityCheckItem {
	username := strings.TrimSpace(s.cfg.InitialAdminUser)
	if username == "" {
		username = "admin"
	}
	var admin model.AdminUser
	err := s.db.Where("username = ?", username).First(&admin).Error
	if err != nil {
		return securityItem("admin_password", "管理员密码", "warning", fmt.Sprintf("未找到初始化管理员账号 %s。", username), "确认生产环境至少存在一个独立管理员账号，并妥善保管密码。")
	}
	if utils.CheckPassword(admin.PasswordHash, "Admin@123456") {
		return securityItem("admin_password", "默认管理员密码", "danger", "默认管理员密码仍为 Admin@123456。", "上线前必须修改 admin 密码，避免被扫描登录。")
	}
	initialPassword := strings.TrimSpace(s.cfg.InitialAdminPass)
	if initialPassword == "" {
		initialPassword = "SaasAdmin@2026!"
	}
	if utils.CheckPassword(admin.PasswordHash, initialPassword) {
		status := "warning"
		if strings.EqualFold(s.cfg.AppEnv, "production") {
			status = "danger"
		}
		return securityItem("admin_password", "初始化管理员密码", status, "管理员密码仍为项目初始化密码。", "本地开发可继续使用；生产上线前请在后台修改为独立强密码，避免初始化密码长期暴露。")
	}
	return securityItem("admin_password", "管理员密码", "pass", "管理员密码已脱离旧默认值和初始化密码。", "继续定期轮换管理员密码，并启用强密码策略。")
}

func (s *AdminService) checkProductionEnv() SecurityCheckItem {
	if strings.EqualFold(s.cfg.AppEnv, "production") {
		return securityItem("app_env", "生产环境标识", "pass", "APP_ENV 已设置为 production。", "保持生产环境与测试环境配置隔离。")
	}
	return securityItem("app_env", "生产环境标识", "warning", "当前 APP_ENV 不是 production。", "正式上线前将 APP_ENV 设置为 production，关闭开发测试能力。")
}

func (s *AdminService) checkSecrets() SecurityCheckItem {
	weakJWT := s.cfg.JWTSecret == "" ||
		s.cfg.JWTSecret == "replace-with-a-very-strong-secret" ||
		s.cfg.JWTSecret == "replace-with-a-long-random-production-secret" ||
		len(s.cfg.JWTSecret) < 32
	weakAES := s.cfg.AESSecret == "" ||
		s.cfg.AESSecret == "0123456789abcdef0123456789abcdef" ||
		s.cfg.AESSecret == "replace-with-32-char-production-key" ||
		len(s.cfg.AESSecret) != 32
	if weakJWT || weakAES {
		return securityItem("secrets", "密钥配置", "danger", "JWT_SECRET 或 AES_SECRET 仍为默认/弱配置。", "生产环境使用随机生成的强密钥，AES_SECRET 必须为 32 字符。")
	}
	return securityItem("secrets", "密钥配置", "pass", "JWT 与 AES 密钥格式符合上线要求。", "不要把生产密钥提交到 GitHub。")
}

func (s *AdminService) checkDatabaseConfig() SecurityCheckItem {
	dsn := strings.ToLower(s.cfg.MySQLDSN)
	if strings.Contains(dsn, "root:") || strings.Contains(dsn, "127.0.0.1") || strings.Contains(dsn, "localhost") {
		return securityItem("database", "数据库配置", "warning", "数据库仍使用本地或 root 连接配置。", "生产环境建议创建独立 MySQL 用户，限制权限并配置备份策略。")
	}
	return securityItem("database", "数据库配置", "pass", "数据库连接看起来已使用非本地生产配置。", "继续确认数据库安全组、备份和最小权限。")
}

func (s *AdminService) checkAlipayConfig() SecurityCheckItem {
	appID := strings.TrimSpace(s.cfg.AlipayAppID)
	privateKey := strings.TrimSpace(s.cfg.AlipayPrivateKey)
	publicKey := strings.TrimSpace(s.cfg.AlipayPublicKey)
	if appID == "" || appID == "2021000000000000" ||
		privateKey == "" || strings.Contains(privateKey, "YOUR_ALIPAY") ||
		publicKey == "" || strings.Contains(publicKey, "YOUR_ALIPAY") {
		return securityItem("alipay", "支付宝配置", "danger", "支付宝 APPID、应用私钥或支付宝公钥未完整配置。", "上线前填写支付宝正式环境配置，并使用公网 HTTPS 回调地址。")
	}
	if s.cfg.AlipaySandbox {
		return securityItem("alipay", "支付宝配置", "warning", "当前仍处于支付宝沙箱模式。", "正式上线前切换到正式应用，并重新验证支付、回调和退款。")
	}
	if !strings.HasPrefix(strings.ToLower(s.cfg.AlipayNotifyURL), "https://") {
		return securityItem("alipay", "支付宝配置", "warning", "支付宝回调地址不是 HTTPS。", "生产环境必须使用公网 HTTPS 回调地址。")
	}
	return securityItem("alipay", "支付宝配置", "pass", "支付宝正式配置看起来完整。", "上线前做一笔小额真实支付和退款验证。")
}

func (s *AdminService) checkAIConfig() SecurityCheckItem {
	if !s.cfg.AIEnabled {
		return securityItem("ai", "AI 模型接入", "warning", "AI_ENABLED 当前为 false，商家端会使用模板兜底。", "如果要把 AI 作为核心卖点，建议接入 DeepSeek、通义千问或智谱等兼容 OpenAI 协议的模型。")
	}
	if strings.TrimSpace(s.cfg.AIBaseURL) == "" || strings.TrimSpace(s.cfg.AIAPIKey) == "" || strings.TrimSpace(s.cfg.AIModel) == "" {
		return securityItem("ai", "AI 模型接入", "danger", "AI 已启用，但 BASE_URL、API_KEY 或 MODEL 未完整配置。", "补齐 AI_BASE_URL、AI_API_KEY、AI_MODEL 后重启后端，并测试商家端 AI 经营分析。")
	}
	return securityItem("ai", "AI 模型接入", "pass", fmt.Sprintf("AI 已启用，当前模型：%s / %s。", s.cfg.AIProvider, s.cfg.AIModel), "继续监控调用成本、失败率和生成质量；建议给商家端设置日调用额度。")
}

func (s *AdminService) checkBackupScripts() SecurityCheckItem {
	windowsOK := fileExists("scripts/backup-mysql.ps1")
	linuxOK := fileExists("scripts/backup-mysql.sh")
	if windowsOK && linuxOK {
		return securityItem("backup", "数据库备份脚本", "pass", "已提供 Windows 和 Linux 数据库备份脚本。", "上线后配置 Windows 计划任务或 Linux cron 定时执行。")
	}
	return securityItem("backup", "数据库备份脚本", "warning", "数据库备份脚本不完整。", "补齐备份脚本，并定期演练恢复。")
}

func (s *AdminService) checkRateLimit() SecurityCheckItem {
	if s.cfg.RateLimitPerMinute <= 0 {
		return securityItem("rate_limit", "限流中间件", "danger", "RATE_LIMIT_PER_MINUTE 未启用或配置无效。", "生产环境必须开启 API 限流，建议按业务压测后设置。")
	}
	if s.cfg.RateLimitPerMinute > 600 {
		return securityItem("rate_limit", "限流中间件", "warning", "当前限流阈值偏高。", "根据业务量设置更合理的限流阈值。")
	}
	return securityItem("rate_limit", "限流中间件", "pass", "限流中间件已配置。", "继续观察登录、支付、短信验证码等高风险接口。")
}

func (s *AdminService) checkCORS() SecurityCheckItem {
	frontend := strings.ToLower(s.cfg.FrontendURL)
	if frontend == "" || strings.Contains(frontend, "localhost") || strings.Contains(frontend, "127.0.0.1") {
		return securityItem("cors", "跨域配置", "warning", "FRONTEND_URL 仍是本地开发地址。", "生产环境改为正式域名，避免宽泛跨域。")
	}
	if !strings.HasPrefix(frontend, "https://") {
		return securityItem("cors", "跨域配置", "warning", "FRONTEND_URL 不是 HTTPS。", "生产环境建议全站 HTTPS。")
	}
	return securityItem("cors", "跨域配置", "pass", "跨域来源已指向正式 HTTPS 域名。", "保持只允许可信前端域名。")
}

func (s *AdminService) checkBootstrapSafety() SecurityCheckItem {
	if strings.EqualFold(s.cfg.AppEnv, "production") {
		return securityItem("bootstrap_safety", "启动初始化保护", "pass", "生产环境不会自动激活待审核商家，也不会覆盖已修改的管理员密码。", "继续保持商家审核、订阅开通和收款审核由平台端人工确认。")
	}
	return securityItem("bootstrap_safety", "启动初始化保护", "warning", "当前为开发环境，待审核商家会自动激活，便于本地测试。", "正式上线前必须设置 APP_ENV=production，确保商家审核流程不会被绕过。")
}

func (s *AdminService) checkLogging() SecurityCheckItem {
	if strings.EqualFold(s.cfg.AppEnv, "production") {
		return securityItem("logging", "请求日志与审计", "pass", "请求日志和操作审计已在后端注册。", "生产环境建议接入日志文件轮转或云日志服务。")
	}
	return securityItem("logging", "请求日志与审计", "warning", "当前为开发环境日志。", "上线前配置日志留存、错误告警和审计查询。")
}

func (s *AdminService) countRows(modelValue interface{}, query string, args ...interface{}) int64 {
	var count int64
	db := s.db.Model(modelValue)
	if strings.TrimSpace(query) != "" {
		db = db.Where(query, args...)
	}
	_ = db.Count(&count).Error
	return count
}

func (s *AdminService) countOrders(query string, args ...interface{}) int64 {
	return s.countRows(&model.Order{}, query, args...)
}

func (s *AdminService) countRefunds(query string, args ...interface{}) int64 {
	return s.countRows(&model.RefundRecord{}, query, args...)
}

func healthMetric(label string, value int64, hint string, status string) OperationHealthMetric {
	return OperationHealthMetric{Label: label, Value: value, Hint: hint, Status: status}
}

func healthStatus(dangerCount, warningCount int64) string {
	if dangerCount > 0 {
		return "danger"
	}
	if warningCount > 0 {
		return "warning"
	}
	return "pass"
}

func warningIfPositive(value int64) string {
	if value > 0 {
		return "warning"
	}
	return "pass"
}

func dangerIfPositive(value int64) string {
	if value > 0 {
		return "danger"
	}
	return "pass"
}

func warningIf(value bool) string {
	if value {
		return "warning"
	}
	return "pass"
}

func dangerIfZero(value int64) string {
	if value <= 0 {
		return "danger"
	}
	return "pass"
}

func boolToCount(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func beginningOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func countMigrationFiles() int64 {
	var count int64
	for _, dir := range []string{"sql/migrations", "migrations"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
				count++
			}
		}
	}
	return count
}

func securityItem(key, title, status, description, suggestion string) SecurityCheckItem {
	return SecurityCheckItem{Key: key, Title: title, Status: status, Description: description, Suggestion: suggestion}
}

func aiProviderPresets() []AIProviderPreset {
	return []AIProviderPreset{
		{
			Provider:    "deepseek",
			Name:        "DeepSeek",
			BaseURL:     "https://api.deepseek.com/v1",
			Model:       "deepseek-chat",
			Description: "适合 MVP 和小规模商家试运营，成本相对友好。",
		},
		{
			Provider:    "qwen",
			Name:        "阿里云百炼通义千问",
			BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
			Model:       "qwen-plus",
			Description: "适合国内云资源整合和稳定商用。",
		},
		{
			Provider:    "zhipu",
			Name:        "智谱开放平台",
			BaseURL:     "请填写官方 OpenAI-compatible 地址",
			Model:       "请填写官方兼容模型名",
			Description: "作为国内备选供应商，正式接入前以控制台最新地址为准。",
		},
		{
			Provider:    "moonshot",
			Name:        "Moonshot / Kimi",
			BaseURL:     "请填写官方 OpenAI-compatible 地址",
			Model:       "请填写官方兼容模型名",
			Description: "适合长文本营销素材备选，正式接入前确认兼容模式。",
		},
	}
}

func aiEnvExample(provider, baseURL, modelName string) string {
	if strings.TrimSpace(provider) == "" || provider == "template" {
		provider = "deepseek"
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	if strings.TrimSpace(modelName) == "" {
		modelName = "deepseek-chat"
	}
	return fmt.Sprintf("AI_ENABLED=true\nAI_PROVIDER=%s\nAI_BASE_URL=%s\nAI_API_KEY=replace-with-ai-api-key\nAI_MODEL=%s\nAI_TIMEOUT_SECONDS=20", provider, baseURL, modelName)
}

func aiUsageSuggestion(total, failed, fallback int64) string {
	if total == 0 {
		return "今日暂无 AI 调用，接入真实模型后建议先观察经营建议、菜单优化和裂变文案三个高频场景。"
	}
	if failed > 0 {
		return "今日存在 AI 调用失败，建议检查供应商额度、Key、网络和超时设置。"
	}
	if fallback > 0 {
		return "今日存在模板兜底结果，说明部分调用未走真实模型；可用于演示，但商用前建议接入真实 API。"
	}
	return "今日 AI 调用正常，后续建议继续接入套餐额度、缓存和成本预警。"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseFollowUpTime(value string) (time.Time, error) {
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("next follow-up time is invalid")
}

func isValidAdminManagedPhone(phone string) bool {
	if len(phone) != 11 {
		return false
	}
	if !strings.HasPrefix(phone, "1") {
		return false
	}
	for _, char := range phone {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func defaultSubscriptionDays(plan string) int {
	switch plan {
	case "trial":
		return 7
	case "month":
		return 30
	case "year":
		return 365
	default:
		return 0
	}
}
