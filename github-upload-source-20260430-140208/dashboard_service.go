package service

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

type DashboardService struct {
	db *gorm.DB
}

func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{db: db}
}

func (s *DashboardService) SummaryWithRange(startValue, endValue string) (map[string]interface{}, error) {
	start, end := dashboardRange(startValue, endValue)
	var userCount int64
	var merchantCount int64
	var orderCount int64
	var paymentCount int64
	var totalRevenue int64
	var platformSubscriptionRevenue int64
	var customerTradeRevenue int64
	var totalRefundAmount int64
	var rangeRefundAmount int64
	var todayUsers int64
	var rangeUsers int64
	var rangeOrders int64
	var rangeRevenue int64

	s.db.Model(&model.User{}).Count(&userCount)
	s.db.Model(&model.Merchant{}).Count(&merchantCount)
	s.db.Model(&model.Order{}).Count(&orderCount)
	s.db.Model(&model.PaymentRecord{}).Count(&paymentCount)
	s.db.Model(&model.PaymentRecord{}).Where("status = ?", "success").Select("COALESCE(SUM(amount),0)").Scan(&totalRevenue)
	s.db.Model(&model.Order{}).Where("status = ? AND order_type = ?", "paid", "merchant_subscription").Select("COALESCE(SUM(total_amount),0)").Scan(&platformSubscriptionRevenue)
	s.db.Model(&model.Order{}).Where("status IN ? AND order_type = ?", []string{"received", "accepted", "completed", "closed"}, "store_order").Select("COALESCE(SUM(total_amount),0)").Scan(&customerTradeRevenue)
	s.db.Model(&model.RefundRecord{}).Where("status = ?", "success").Select("COALESCE(SUM(amount),0)").Scan(&totalRefundAmount)
	s.db.Model(&model.RefundRecord{}).Where("status = ? AND created_at BETWEEN ? AND ?", "success", start, end).Select("COALESCE(SUM(amount),0)").Scan(&rangeRefundAmount)
	s.db.Model(&model.User{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&rangeUsers)
	s.db.Model(&model.Order{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&rangeOrders)
	s.db.Model(&model.PaymentRecord{}).Where("status = ? AND created_at BETWEEN ? AND ?", "success", start, end).Select("COALESCE(SUM(amount),0)").Scan(&rangeRevenue)
	todayStart := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&model.User{}).Where("created_at >= ?", todayStart).Count(&todayUsers)

	var userTrend []map[string]interface{}
	s.db.Model(&model.User{}).
		Select("DATE(created_at) AS date, COUNT(*) AS count").
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("DATE(created_at)").
		Order("date asc").
		Scan(&userTrend)

	var revenueTrend []map[string]interface{}
	s.db.Model(&model.PaymentRecord{}).
		Select("DATE(created_at) AS date, COALESCE(SUM(amount),0) AS amount").
		Where("status = ? AND created_at BETWEEN ? AND ?", "success", start, end).
		Group("DATE(created_at)").
		Order("date asc").
		Scan(&revenueTrend)

	var memberStats []map[string]interface{}
	s.db.Model(&model.User{}).
		Select("member_level AS level, COUNT(*) AS count").
		Group("member_level").
		Order("count desc").
		Scan(&memberStats)

	var packageRevenue []map[string]interface{}
	s.db.Table("orders").
		Select("membership_packages.name AS name, membership_packages.code AS code, COALESCE(SUM(orders.amount),0) AS amount, COUNT(orders.id) AS count").
		Joins("LEFT JOIN membership_packages ON membership_packages.id = orders.package_id").
		Where("orders.status = ? AND orders.paid_at BETWEEN ? AND ?", "paid", start, end).
		Group("membership_packages.id, membership_packages.name, membership_packages.code").
		Order("amount desc").
		Scan(&packageRevenue)

	var merchantOps []map[string]interface{}
	s.db.Table("merchants").
		Select(`merchants.id AS merchant_id,
			merchants.name AS merchant_name,
			merchants.status AS status,
			merchants.subscription_status AS subscription_status,
			COUNT(orders.id) AS order_count,
			COALESCE(SUM(CASE WHEN orders.status IN ('received','accepted','completed','closed') THEN orders.total_amount ELSE 0 END),0) AS trade_amount,
			COALESCE((SELECT SUM(refund_records.amount) FROM refund_records WHERE refund_records.merchant_id = merchants.id AND refund_records.status = 'success' AND refund_records.created_at BETWEEN ? AND ?),0) AS refund_amount,
			COALESCE(AVG(CASE WHEN orders.status IN ('received','accepted','completed','closed') THEN orders.total_amount ELSE NULL END),0) AS avg_order_amount,
			COALESCE(SUM(CASE WHEN orders.status IN ('accepted','completed') THEN 1 ELSE 0 END),0) AS accepted_count,
			COALESCE(SUM(CASE WHEN orders.status = 'closed' THEN 1 ELSE 0 END),0) AS closed_count`, start, end).
		Joins("LEFT JOIN orders ON orders.merchant_id = merchants.id AND orders.order_type = ? AND orders.created_at BETWEEN ? AND ?", "store_order", start, end).
		Group("merchants.id, merchants.name, merchants.status, merchants.subscription_status").
		Order("trade_amount desc, order_count desc").
		Limit(20).
		Scan(&merchantOps)

	var revenueMix []map[string]interface{}
	s.db.Table("orders").
		Select("order_type, COALESCE(SUM(total_amount),0) AS amount, COUNT(id) AS count").
		Where("status IN ? AND created_at BETWEEN ? AND ?", []string{"paid", "received", "accepted", "completed", "closed"}, start, end).
		Group("order_type").
		Order("amount desc").
		Scan(&revenueMix)

	var merchantRevenueTrend []map[string]interface{}
	s.db.Table("orders").
		Select("DATE(created_at) AS date, COALESCE(SUM(total_amount),0) AS amount").
		Where("order_type = ? AND status = ? AND created_at BETWEEN ? AND ?", "merchant_subscription", "paid", start, end).
		Group("DATE(created_at)").
		Order("date asc").
		Scan(&merchantRevenueTrend)

	var storeOrderTrend []map[string]interface{}
	s.db.Table("orders").
		Select("DATE(created_at) AS date, COALESCE(SUM(total_amount),0) AS amount, COUNT(id) AS count").
		Where("order_type = ? AND status IN ? AND created_at BETWEEN ? AND ?", "store_order", []string{"received", "accepted", "completed", "closed"}, start, end).
		Group("DATE(created_at)").
		Order("date asc").
		Scan(&storeOrderTrend)

	var riskMerchants []map[string]interface{}
	s.db.Table("merchants").
		Select(`merchants.id AS merchant_id,
			merchants.name AS merchant_name,
			merchants.status AS status,
			COUNT(orders.id) AS order_count,
			COALESCE(SUM(CASE WHEN orders.status = 'closed' THEN 1 ELSE 0 END),0) AS closed_count,
			COALESCE(SUM(CASE WHEN orders.status IN ('received','accepted','completed','closed') THEN orders.total_amount ELSE 0 END),0) AS trade_amount`).
		Joins("LEFT JOIN orders ON orders.merchant_id = merchants.id AND orders.order_type = ? AND orders.created_at BETWEEN ? AND ?", "store_order", start, end).
		Group("merchants.id, merchants.name, merchants.status").
		Having("COUNT(orders.id) > 0").
		Order("closed_count desc, order_count desc").
		Limit(8).
		Scan(&riskMerchants)

	var productPatrol []map[string]interface{}
	s.db.Table("store_products").
		Select("store_products.id AS product_id, store_products.name AS product_name, store_products.status, store_products.price, stores.name AS store_name, merchants.name AS merchant_name").
		Joins("LEFT JOIN stores ON stores.id = store_products.store_id").
		Joins("LEFT JOIN merchants ON merchants.id = stores.merchant_id").
		Order("store_products.updated_at desc").
		Limit(8).
		Scan(&productPatrol)

	var orderPatrol []map[string]interface{}
	s.db.Table("orders").
		Select("orders.id, orders.order_no, orders.order_type, orders.status, orders.total_amount, orders.customer_phone, stores.name AS store_name, merchants.name AS merchant_name, orders.created_at").
		Joins("LEFT JOIN stores ON stores.id = orders.store_id").
		Joins("LEFT JOIN merchants ON merchants.id = orders.merchant_id").
		Where("orders.order_type = ? AND orders.created_at BETWEEN ? AND ?", "store_order", start, end).
		Order("orders.id desc").
		Limit(8).
		Scan(&orderPatrol)

	return map[string]interface{}{
		"user_count":                    userCount,
		"merchant_count":                merchantCount,
		"order_count":                   orderCount,
		"payment_count":                 paymentCount,
		"total_revenue":                 totalRevenue,
		"platform_subscription_revenue": platformSubscriptionRevenue,
		"customer_trade_revenue":        customerTradeRevenue,
		"total_refund_amount":           totalRefundAmount,
		"customer_trade_net":            customerTradeRevenue - totalRefundAmount,
		"range": map[string]interface{}{
			"start": start.Format("2006-01-02"),
			"end":   end.Format("2006-01-02"),
		},
		"today_user_count":       todayUsers,
		"range_user_count":       rangeUsers,
		"range_order_count":      rangeOrders,
		"range_revenue":          rangeRevenue,
		"range_refund_amount":    rangeRefundAmount,
		"range_net_revenue":      rangeRevenue - rangeRefundAmount,
		"user_trend":             userTrend,
		"revenue_trend":          revenueTrend,
		"member_stats":           memberStats,
		"package_revenue":        packageRevenue,
		"merchant_ops":           merchantOps,
		"revenue_mix":            revenueMix,
		"merchant_revenue_trend": merchantRevenueTrend,
		"store_order_trend":      storeOrderTrend,
		"risk_merchants":         riskMerchants,
		"product_patrol":         productPatrol,
		"order_patrol":           orderPatrol,
	}, nil
}

func (s *DashboardService) Summary() (map[string]interface{}, error) {
	return s.SummaryWithRange("", "")
}

func dashboardRange(startValue, endValue string) (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)
	if strings.TrimSpace(startValue) != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(startValue), time.Local); err == nil {
			start = parsed
		}
	}
	if strings.TrimSpace(endValue) != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(endValue), time.Local); err == nil {
			end = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, time.Local)
		}
	}
	if start.After(end) {
		return end, start
	}
	return start, end
}
