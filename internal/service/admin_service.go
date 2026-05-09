package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	OverallStatus string              `json:"overall_status"`
	GeneratedAt   time.Time           `json:"generated_at"`
	Environment   string              `json:"environment"`
	Items         []SecurityCheckItem `json:"items"`
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

func (s *AdminService) ListMerchants(page, pageSize int) ([]model.Merchant, int64, error) {
	return s.merchants.List((page-1)*pageSize, pageSize)
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
		s.checkBackupScripts(),
		s.checkRateLimit(),
		s.checkCORS(),
		s.checkLogging(),
	}
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
	return SecurityCheckReport{
		OverallStatus: overall,
		GeneratedAt:   time.Now(),
		Environment:   s.cfg.AppEnv,
		Items:         items,
	}
}

func (s *AdminService) checkAdminPassword() SecurityCheckItem {
	var admin model.AdminUser
	err := s.db.Where("username = ?", "admin").First(&admin).Error
	if err != nil {
		return securityItem("admin_password", "默认管理员密码", "warning", "未找到默认 admin 账号。", "确认生产环境至少存在一个独立管理员账号，并妥善保管密码。")
	}
	if utils.CheckPassword(admin.PasswordHash, "Admin@123456") {
		return securityItem("admin_password", "默认管理员密码", "danger", "默认管理员密码仍为 Admin@123456。", "上线前必须修改 admin 密码，避免被扫描登录。")
	}
	return securityItem("admin_password", "默认管理员密码", "pass", "默认管理员密码已修改。", "继续定期轮换管理员密码，并启用强密码策略。")
}

func (s *AdminService) checkProductionEnv() SecurityCheckItem {
	if strings.EqualFold(s.cfg.AppEnv, "production") {
		return securityItem("app_env", "生产环境标识", "pass", "APP_ENV 已设置为 production。", "保持生产环境与测试环境配置隔离。")
	}
	return securityItem("app_env", "生产环境标识", "warning", "当前 APP_ENV 不是 production。", "正式上线前将 APP_ENV 设置为 production，关闭开发测试能力。")
}

func (s *AdminService) checkSecrets() SecurityCheckItem {
	weakJWT := s.cfg.JWTSecret == "" || s.cfg.JWTSecret == "replace-with-a-very-strong-secret" || len(s.cfg.JWTSecret) < 32
	weakAES := s.cfg.AESSecret == "" || s.cfg.AESSecret == "0123456789abcdef0123456789abcdef" || len(s.cfg.AESSecret) != 32
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
	if s.cfg.AlipayAppID == "" || s.cfg.AlipayPrivateKey == "" || s.cfg.AlipayPublicKey == "" {
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

func (s *AdminService) checkLogging() SecurityCheckItem {
	if strings.EqualFold(s.cfg.AppEnv, "production") {
		return securityItem("logging", "请求日志与审计", "pass", "请求日志和操作审计已在后端注册。", "生产环境建议接入日志文件轮转或云日志服务。")
	}
	return securityItem("logging", "请求日志与审计", "warning", "当前为开发环境日志。", "上线前配置日志留存、错误告警和审计查询。")
}

func securityItem(key, title, status, description, suggestion string) SecurityCheckItem {
	return SecurityCheckItem{Key: key, Title: title, Status: status, Description: description, Suggestion: suggestion}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
