package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

const adminOrderExportLimit = 10000

func serializeAdminOrders(orders []model.Order, includeContact bool) []gin.H {
	rows := make([]gin.H, 0, len(orders))
	for i := range orders {
		rows = append(rows, serializeAdminOrder(&orders[i], includeContact))
	}
	return rows
}

func serializeAdminOrder(order *model.Order, includeContact bool) gin.H {
	if order == nil {
		return gin.H{}
	}
	row := serializeOrder(order)
	phone := order.CustomerPhone
	if !includeContact {
		phone = utils.MaskPhone(phone)
	}
	row["customer_phone"] = phone
	if order.User != nil {
		user := gin.H{"id": order.User.ID, "display_name": order.User.DisplayName, "phone": order.User.Phone}
		if !includeContact {
			user["phone"] = utils.MaskPhone(order.User.Phone)
		} else {
			user["email"] = order.User.Email
		}
		row["user"] = user
	} else {
		row["user"] = nil
	}
	if order.Merchant != nil {
		row["merchant"] = gin.H{"id": order.Merchant.ID, "name": order.Merchant.Name}
	} else {
		row["merchant"] = nil
	}
	if order.Store != nil {
		row["store"] = gin.H{"id": order.Store.ID, "name": order.Store.Name}
	} else {
		row["store"] = nil
	}
	if order.Package != nil {
		row["package"] = gin.H{"id": order.Package.ID, "name": order.Package.Name}
	}
	if order.MerchantPlan != nil {
		row["merchant_plan"] = gin.H{"id": order.MerchantPlan.ID, "name": order.MerchantPlan.Name}
	}
	if order.Promotion != nil {
		row["promotion"] = gin.H{"id": order.Promotion.ID, "name": order.Promotion.Title}
	}
	delete(row, "customer_note")
	delete(row, "merchant_note")
	delete(row, "customer_review")
	delete(row, "reviewed_at")
	delete(row, "operation_logs")
	return row
}

func serializeAdminOrderFinanceSummary(order model.Order) gin.H {
	row := gin.H{
		"id": order.ID, "order_no": order.OrderNo, "order_type": order.OrderType,
		"merchant_id": order.MerchantID, "store_id": order.StoreID, "settlement_id": order.SettlementID,
		"amount": order.Amount, "total_amount": order.TotalAmount,
		"refunded_amount": order.RefundedAmount, "refund_status": order.RefundStatus,
		"status": order.Status, "payment_channel": order.PaymentChannel,
		"paid_at": order.PaidAt, "transaction_no": order.TransactionNo,
	}
	if order.Merchant != nil {
		row["merchant"] = gin.H{"id": order.Merchant.ID, "name": order.Merchant.Name}
	}
	if order.Store != nil {
		row["store"] = gin.H{"id": order.Store.ID, "name": order.Store.Name}
	}
	return row
}

func serializeAdminPayments(records []model.PaymentRecord) []gin.H {
	rows := make([]gin.H, 0, len(records))
	for _, record := range records {
		rows = append(rows, gin.H{
			"id": record.ID, "order_id": record.OrderID, "transaction_no": record.TransactionNo,
			"payment_channel": record.PaymentChannel, "amount": record.Amount,
			"status": record.Status, "created_at": record.CreatedAt,
			"order": serializeAdminOrderFinanceSummary(record.Order),
		})
	}
	return rows
}

func serializeAdminRefunds(records []model.RefundRecord) []gin.H {
	rows := make([]gin.H, 0, len(records))
	for _, record := range records {
		rows = append(rows, gin.H{
			"id": record.ID, "order_id": record.OrderID, "merchant_id": record.MerchantID,
			"store_id": record.StoreID, "refund_no": record.RefundNo,
			"transaction_no": record.TransactionNo, "amount": record.Amount,
			"reason": record.Reason, "operator_role": record.OperatorRole,
			"status": record.Status, "created_at": record.CreatedAt,
			"order": serializeAdminOrderFinanceSummary(record.Order),
		})
	}
	return rows
}

func (ctl *OrderController) ExportAdminOrders(c *gin.Context) {
	includeContact, allowed := authorizeAdminCustomerContactRequest(c)
	if !allowed {
		return
	}
	orders, err := ctl.service.AllOrders()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to load platform orders")
		return
	}
	filters, ok := parseAdminOrderExportFilters(c)
	if !ok {
		utils.Error(c, http.StatusBadRequest, "invalid order export filters")
		return
	}
	matched := make([]model.Order, 0)
	for _, order := range orders {
		if adminOrderMatchesExportFilters(order, filters) {
			matched = append(matched, order)
		}
	}
	truncated := len(matched) > adminOrderExportLimit
	if truncated {
		matched = matched[:adminOrderExportLimit]
	}
	if !recordAdminCustomerOrderExport(c, ctl.audit, "platform_orders") {
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=platform-orders.xlsx")
	if truncated {
		c.Header("X-Export-Truncated", "true")
	}
	headers := []string{"Order No", "Order Type", "Merchant", "Store", "Customer Phone", "Amount (Yuan)", "Refunded (Yuan)", "Status", "Refund Status", "Payment Channel", "Paid At", "Created At", "Subscription Plan Snapshot", "Subscription Price Snapshot (Yuan)", "Subscription Duration Snapshot (Days)", "Manual Review Status", "Manual Review Plan", "Manual Review Amount (Yuan)"}
	rows := make([][]string, 0, len(matched))
	for _, order := range matched {
		phone := order.CustomerPhone
		if !includeContact {
			phone = utils.MaskPhone(phone)
		}
		merchantName, storeName := "", ""
		if order.Merchant != nil {
			merchantName = order.Merchant.Name
		}
		if order.Store != nil {
			storeName = order.Store.Name
		}
		var paidAt string
		if order.PaidAt != nil {
			paidAt = order.PaidAt.Format("2006-01-02 15:04:05")
		}
		amount := order.TotalAmount
		if amount <= 0 {
			amount = order.Amount
		}
		planPrice, planDuration := "", ""
		if order.PlanPriceCentsSnapshot != nil {
			planPrice = utils.FenToYuan(*order.PlanPriceCentsSnapshot)
		}
		if order.PlanDurationDaysSnapshot != nil {
			planDuration = fmt.Sprint(*order.PlanDurationDaysSnapshot)
		}
		manualStatus, manualPlan, manualAmount := "", "", ""
		if order.ManualReview != nil {
			manualStatus = order.ManualReview.ReviewStatus
			manualPlan = order.ManualReview.PlanName
			manualAmount = utils.FenToYuan(order.ManualReview.AmountCents)
		}
		rows = append(rows, []string{
			order.OrderNo, order.OrderType, merchantName, storeName, phone,
			utils.FenToYuan(amount), utils.FenToYuan(order.RefundedAmount),
			order.Status, order.RefundStatus, order.PaymentChannel, paidAt,
			order.CreatedAt.Format("2006-01-02 15:04:05"),
			planNameSnapshot(order), planPrice, planDuration, manualStatus, manualPlan, manualAmount,
		})
	}
	if truncated {
		rows = append(rows, []string{"已达到 10,000 条单次导出上限；剩余记录未导出，请缩小筛选范围后重试。"})
	}
	if err := utils.WriteXLSX(c.Writer, "Platform Orders", headers, rows); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to generate order export")
		return
	}
}

type adminOrderExportFilters struct {
	DateFrom          string
	DateTo            string
	Keyword           string
	OrderType         string
	Status            string
	Exception         string
	MerchantIDs       map[uint]struct{}
	RestrictMerchants bool
}

func parseAdminOrderExportFilters(c *gin.Context) (adminOrderExportFilters, bool) {
	filters := adminOrderExportFilters{
		DateFrom: c.Query("date_from"), DateTo: c.Query("date_to"),
		Keyword:   strings.ToLower(strings.TrimSpace(c.Query("keyword"))),
		OrderType: strings.TrimSpace(c.Query("order_type")), Status: strings.TrimSpace(c.Query("status")),
		Exception: strings.TrimSpace(c.Query("exception")),
	}
	for _, date := range []string{filters.DateFrom, filters.DateTo} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return filters, false
			}
		}
	}
	if raw := strings.TrimSpace(c.Query("merchant_ids")); raw != "" {
		filters.RestrictMerchants = true
		filters.MerchantIDs = make(map[uint]struct{})
		for _, item := range strings.Split(raw, ",") {
			value, err := strconv.ParseUint(strings.TrimSpace(item), 10, 64)
			if err != nil || value == 0 {
				return filters, false
			}
			filters.MerchantIDs[uint(value)] = struct{}{}
		}
	}
	return filters, true
}

func adminOrderMatchesExportFilters(order model.Order, filters adminOrderExportFilters) bool {
	if filters.OrderType != "" && order.OrderType != filters.OrderType {
		return false
	}
	if filters.Status != "" && filters.Status != "expiring" && order.Status != filters.Status {
		return false
	}
	if filters.DateFrom != "" && order.CreatedAt.Format("2006-01-02") < filters.DateFrom {
		return false
	}
	if filters.DateTo != "" && order.CreatedAt.Format("2006-01-02") > filters.DateTo {
		return false
	}
	if filters.RestrictMerchants {
		if order.MerchantID == nil {
			return false
		}
		if _, ok := filters.MerchantIDs[*order.MerchantID]; !ok {
			return false
		}
	}
	if filters.Keyword != "" {
		values := []string{order.OrderNo, order.CustomerPhone}
		if order.Merchant != nil {
			values = append(values, order.Merchant.Name)
		}
		if order.User != nil {
			values = append(values, order.User.Phone)
		}
		if order.OrderType == "merchant_subscription" {
			values = append(values, planNameSnapshot(order))
			if order.MerchantPlan != nil {
				values = append(values, order.MerchantPlan.Name)
			}
		}
		found := false
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), filters.Keyword) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	switch filters.Exception {
	case "subscription_pending":
		return order.OrderType == "merchant_subscription" && order.Status == "pending"
	case "payment":
		return order.Status == "pending" || order.Status == "failed"
	case "received":
		return order.Status == "received" && order.PaidAt != nil && order.TotalAmount > 0
	case "refund":
		return order.RefundedAmount > 0 || order.RefundStatus == "partial" || order.RefundStatus == "full" || order.RefundStatus == "processing"
	case "settlement":
		return isUnsettledAdminOrder(order)
	case "closed":
		return order.Status == "closed"
	case "expiring":
		return filters.RestrictMerchants
	case "":
		return true
	default:
		return false
	}
}

func isUnsettledAdminOrder(order model.Order) bool {
	return order.OrderType == "store_order" && order.PaidAt != nil && order.TotalAmount > 0 && order.SettlementID == nil && (order.Status == "accepted" || order.Status == "completed")
}

func planNameSnapshot(order model.Order) string {
	if order.PlanNameSnapshot == nil {
		return ""
	}
	return *order.PlanNameSnapshot
}
