package controller

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/smartwalle/alipay/v3"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type OrderController struct {
	service *service.OrderService
	alipay  *service.AlipayService
	audit   *service.AuditService
}

func NewOrderController(service *service.OrderService, alipayService *service.AlipayService, audit *service.AuditService) *OrderController {
	return &OrderController{service: service, alipay: alipayService, audit: audit}
}

func (ctl *OrderController) Create(c *gin.Context) {
	var req dto.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	order, err := ctl.service.CreateOrder(c.GetUint("user_id"), req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.PaymentChannel != "alipay" {
		utils.Success(c, gin.H{"order": serializeOrder(order)})
		return
	}

	payment, err := ctl.alipay.BuildCheckoutPayload(order, req.PayMode)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"order": serializeOrder(order), "payment": payment})
}

func (ctl *OrderController) PublicStoreProducts(c *gin.Context) {
	storeID := uint(atoi(c.Param("storeId")))
	products, err := ctl.service.PublicStoreProducts(storeID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": products})
}

func (ctl *OrderController) PublicStoreCoupons(c *gin.Context) {
	storeID := uint(atoi(c.Param("storeId")))
	coupons, err := ctl.service.PublicStoreCoupons(storeID, c.Query("phone"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": coupons})
}

func (ctl *OrderController) CreateCustomerOrder(c *gin.Context) {
	var req dto.CreateCustomerOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	order, err := ctl.service.CreateCustomerStoreOrder(req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.PaymentChannel != "" && req.PaymentChannel != "alipay" {
		utils.Success(c, gin.H{"order": serializeOrder(order)})
		return
	}
	if order.Status != "pending" {
		utils.Success(c, gin.H{"order": serializeOrder(order), "payment_required": false})
		return
	}

	payment, err := ctl.alipay.BuildCheckoutPayload(order, req.PayMode)
	if err != nil {
		utils.Success(c, gin.H{
			"order":         serializeOrder(order),
			"payment_error": err.Error(),
			"retry_path":    "/customer/orders/" + url.PathEscape(order.OrderNo),
		})
		return
	}

	utils.Success(c, gin.H{"order": serializeOrder(order), "payment": payment})
}

func (ctl *OrderController) CustomerOrderDetail(c *gin.Context) {
	order, err := ctl.service.PublicOrderDetail(c.Param("orderNo"))
	if err != nil {
		utils.Error(c, http.StatusNotFound, err.Error())
		return
	}
	paymentConfig, err := ctl.service.PublicPaymentConfigForOrder(order)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializeOrder(order), "payment_config": serializePublicPaymentConfig(paymentConfig)})
}

func (ctl *OrderController) MarkCustomerOrderPaid(c *gin.Context) {
	order, err := ctl.service.MarkCustomerStoreOrderPaid(c.Param("orderNo"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	paymentConfig, _ := ctl.service.PublicPaymentConfigForOrder(order)
	utils.Success(c, gin.H{"order": serializeOrder(order), "payment_config": serializePublicPaymentConfig(paymentConfig), "marked": true})
}

func (ctl *OrderController) AppendCustomerOrderItems(c *gin.Context) {
	var req dto.AppendCustomerOrderItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	order, err := ctl.service.AppendCustomerStoreOrderItems(c.Param("orderNo"), req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	paymentConfig, _ := ctl.service.PublicPaymentConfigForOrder(order)
	utils.Success(c, gin.H{"order": serializeOrder(order), "payment_config": serializePublicPaymentConfig(paymentConfig), "appended": true})
}

func (ctl *OrderController) RetryCustomerOrderPayment(c *gin.Context) {
	var req dto.RetryCustomerOrderPaymentRequest
	_ = c.ShouldBindJSON(&req)

	order, err := ctl.service.PrepareCustomerOrderRetry(c.Param("orderNo"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	payment, err := ctl.alipay.BuildCheckoutPayload(order, req.PayMode)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"order": serializeOrder(order), "payment": payment})
}

func (ctl *OrderController) ReviewCustomerOrder(c *gin.Context) {
	var req dto.ReviewCustomerOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	order, err := ctl.service.ReviewCustomerOrder(c.Param("orderNo"), req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializeOrder(order), "reviewed": true})
}

func (ctl *OrderController) ListMine(c *gin.Context) {
	orders, err := ctl.service.UserOrders(c.GetUint("user_id"))
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, serializeOrders(orders))
}

func (ctl *OrderController) Detail(c *gin.Context) {
	order, err := ctl.service.UserOrderDetail(c.GetUint("user_id"), c.Param("orderNo"))
	if err != nil {
		utils.Error(c, http.StatusNotFound, err.Error())
		return
	}
	utils.Success(c, serializeOrder(order))
}

func (ctl *OrderController) Cancel(c *gin.Context) {
	if err := ctl.service.CancelOrder(c.GetUint("user_id"), c.Param("orderNo")); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"cancelled": true})
}

func (ctl *OrderController) RedeemCard(c *gin.Context) {
	var req dto.RedeemCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := ctl.service.RedeemCard(c.GetUint("user_id"), req.Code); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"redeemed": true})
}

func (ctl *OrderController) PaymentCallback(c *gin.Context) {
	if ctl.alipay == nil || !ctl.alipay.Enabled() {
		c.String(http.StatusServiceUnavailable, "failure")
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "failure")
		return
	}

	values := url.Values(c.Request.PostForm)
	if err := ctl.alipay.VerifyNotification(values); err != nil {
		c.String(http.StatusBadRequest, "failure")
		return
	}

	tradeStatus := values.Get("trade_status")
	if !service.IsAlipaySuccessStatus(tradeStatus) {
		if service.IsAlipayClosedStatus(tradeStatus) {
			amount, _ := utils.YuanToFen(values.Get("total_amount"))
			transactionNo := values.Get("trade_no")
			if transactionNo == "" {
				transactionNo = values.Get("out_trade_no") + "-closed"
			}
			_, _ = ctl.service.HandlePaymentCallback(dto.PaymentCallbackRequest{
				OrderNo:        values.Get("out_trade_no"),
				TransactionNo:  transactionNo,
				PaymentChannel: "alipay",
				Amount:         amount,
				Status:         "failed",
				RawPayload:     values,
			})
			alipay.AckNotification(c.Writer)
			return
		}
		alipay.AckNotification(c.Writer)
		return
	}

	amount, err := utils.YuanToFen(values.Get("total_amount"))
	if err != nil {
		c.String(http.StatusBadRequest, "failure")
		return
	}

	req := dto.PaymentCallbackRequest{
		OrderNo:        values.Get("out_trade_no"),
		TransactionNo:  values.Get("trade_no"),
		PaymentChannel: "alipay",
		Amount:         amount,
		Status:         "success",
		RawPayload:     values,
	}

	if _, err := ctl.service.HandlePaymentCallback(req); err != nil {
		c.String(http.StatusBadRequest, "failure")
		return
	}
	alipay.AckNotification(c.Writer)
}

func (ctl *OrderController) MerchantOrders(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	status := strings.TrimSpace(c.Query("status"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	var storeID *uint
	if raw := atoi(c.Query("store_id")); raw > 0 {
		value := uint(raw)
		storeID = &value
	}

	list, total, err := ctl.service.MerchantOrders(merchantID, status, storeID, keyword, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"list": serializeOrders(list), "total": total})
}

func (ctl *OrderController) MerchantOrderDetail(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	orderID := uint(atoi(c.Param("id")))
	order, err := ctl.service.MerchantOrderDetail(merchantID, orderID)
	if err != nil {
		utils.Error(c, http.StatusNotFound, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializeOrder(order)})
}

func (ctl *OrderController) AcceptMerchantOrder(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	orderID := uint(atoi(c.Param("id")))
	if err := ctl.service.AcceptMerchantOrder(merchantID, orderID); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"accepted": true})
}

func (ctl *OrderController) ConfirmMerchantOrderPayment(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	orderID := uint(atoi(c.Param("id")))
	order, err := ctl.service.ConfirmMerchantOrderPayment(merchantID, orderID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializeOrder(order), "confirmed": true})
}

func (ctl *OrderController) CompleteMerchantOrder(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	orderID := uint(atoi(c.Param("id")))
	if err := ctl.service.CompleteMerchantOrder(merchantID, orderID); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"completed": true})
}

func (ctl *OrderController) CloseMerchantOrder(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	orderID := uint(atoi(c.Param("id")))
	if err := ctl.service.CloseMerchantOrder(merchantID, orderID); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"closed": true})
}

func (ctl *OrderController) UpdateMerchantOrderNote(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.UpdateMerchantOrderNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	orderID := uint(atoi(c.Param("id")))
	order, err := ctl.service.UpdateMerchantOrderNote(merchantID, orderID, req.MerchantNote)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializeOrder(order), "updated": true})
}

func (ctl *OrderController) RefundMerchantOrder(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.RefundOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	orderID := uint(atoi(c.Param("id")))
	order, err := ctl.service.RefundMerchantOrder(merchantID, orderID, req, c.GetUint("user_id"), "merchant")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_order_refund",
		TargetType: "order",
		TargetID:   order.ID,
		TargetName: order.OrderNo,
		MerchantID: order.MerchantID,
		Detail: gin.H{
			"amount":      req.Amount,
			"full_refund": req.Full,
			"reason":      req.Reason,
		},
	})
	utils.Success(c, gin.H{"order": serializeOrder(order), "refunded": true})
}

func (ctl *OrderController) RefundAdminOrder(c *gin.Context) {
	var req dto.RefundOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	orderID := uint(atoi(c.Param("id")))
	order, err := ctl.service.RefundAdminOrder(orderID, req, c.GetUint("user_id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "admin_order_refund",
		TargetType: "order",
		TargetID:   order.ID,
		TargetName: order.OrderNo,
		MerchantID: order.MerchantID,
		Detail: gin.H{
			"amount":      req.Amount,
			"full_refund": req.Full,
			"reason":      req.Reason,
		},
	})
	utils.Success(c, gin.H{"order": serializeOrder(order), "refunded": true})
}

func (ctl *OrderController) ListAll(c *gin.Context) {
	orders, err := ctl.service.AllOrders()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, serializeOrders(orders))
}

func (ctl *OrderController) Payments(c *gin.Context) {
	list, err := ctl.service.AllPayments()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, list)
}

func (ctl *OrderController) ConfirmMerchantSubscriptionPayment(c *gin.Context) {
	var req dto.ConfirmSubscriptionPaymentRequest
	_ = c.ShouldBindJSON(&req)
	orderID := uint(atoi(c.Param("id")))
	order, err := ctl.service.ConfirmMerchantSubscriptionPayment(orderID, req.Remark)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_subscription_payment_confirm",
		TargetType: "order",
		TargetID:   order.ID,
		TargetName: order.OrderNo,
		MerchantID: order.MerchantID,
		Detail: gin.H{
			"amount":       order.TotalAmount,
			"merchant":     order.Merchant,
			"plan":         order.MerchantPlan,
			"order_no":     order.OrderNo,
			"payment_mode": "platform_qr",
			"remark":       req.Remark,
			"expire_at":    order.SubscriptionEndAt,
			"result":       "subscription_opened",
		},
	})
	utils.Success(c, gin.H{"order": serializeOrder(order), "confirmed": true})
}

func (ctl *OrderController) Refunds(c *gin.Context) {
	list, err := ctl.service.AllRefunds()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, list)
}

func (ctl *OrderController) MerchantRefunds(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	list, err := ctl.service.MerchantRefunds(merchantID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, list)
}

func (ctl *OrderController) MerchantSettlements(c *gin.Context) {
	merchantID, ok := currentMerchantOrderMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	summary, list, err := ctl.service.MerchantSettlements(merchantID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, gin.H{"summary": summary, "list": list})
}

func serializeOrders(orders []model.Order) []gin.H {
	rows := make([]gin.H, 0, len(orders))
	for _, order := range orders {
		item := order
		rows = append(rows, serializeOrder(&item))
	}
	return rows
}

func serializeOrder(order *model.Order) gin.H {
	return gin.H{
		"id":                  order.ID,
		"order_no":            order.OrderNo,
		"order_type":          order.OrderType,
		"user_id":             order.UserID,
		"merchant_id":         order.MerchantID,
		"settlement_id":       order.SettlementID,
		"store_id":            order.StoreID,
		"customer_phone":      order.CustomerPhone,
		"package_id":          order.PackageID,
		"merchant_plan_id":    order.MerchantPlanID,
		"amount":              order.Amount,
		"total_amount":        order.TotalAmount,
		"discount_amount":     order.DiscountAmount,
		"refunded_amount":     order.RefundedAmount,
		"refund_status":       order.RefundStatus,
		"promotion_id":        order.PromotionID,
		"referral_coupon_id":  order.CouponID,
		"coupon_no":           order.CouponNo,
		"items":               parseOrderItems(order.Items),
		"customer_note":       order.CustomerNote,
		"merchant_note":       order.MerchantNote,
		"customer_rating":     order.CustomerRating,
		"customer_review":     order.CustomerReview,
		"reviewed_at":         order.ReviewedAt,
		"operation_logs":      parseOrderLogs(order.OperationLogs),
		"status":              order.Status,
		"payment_channel":     order.PaymentChannel,
		"auto_renew":          order.AutoRenew,
		"paid_at":             order.PaidAt,
		"transaction_no":      order.TransactionNo,
		"subscription_end_at": order.SubscriptionEndAt,
		"created_at":          order.CreatedAt,
		"updated_at":          order.UpdatedAt,
		"user":                order.User,
		"merchant":            order.Merchant,
		"store":               order.Store,
		"package":             order.Package,
		"merchant_plan":       order.MerchantPlan,
		"promotion":           order.Promotion,
	}
}

func serializePublicPaymentConfig(config *model.MerchantPaymentConfig) gin.H {
	if config == nil {
		return gin.H{}
	}
	return gin.H{
		"mode":           config.Mode,
		"channel":        config.Channel,
		"account_name":   config.AccountName,
		"account_no":     config.AccountNo,
		"alipay_qr_code": config.AlipayQRCode,
		"wechat_qr_code": config.WechatQRCode,
		"contact_phone":  config.ContactPhone,
		"status":         config.Status,
		"audit_status":   config.AuditStatus,
	}
}

func parseOrderItems(raw string) []service.StoreOrderItemSnapshot {
	if strings.TrimSpace(raw) == "" {
		return []service.StoreOrderItemSnapshot{}
	}
	var items []service.StoreOrderItemSnapshot
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []service.StoreOrderItemSnapshot{}
	}
	return items
}

func parseOrderLogs(raw string) []service.OrderOperationLog {
	if strings.TrimSpace(raw) == "" {
		return []service.OrderOperationLog{}
	}
	var logs []service.OrderOperationLog
	if err := json.Unmarshal([]byte(raw), &logs); err != nil {
		return []service.OrderOperationLog{}
	}
	return logs
}

func currentMerchantOrderMerchantID(c *gin.Context) (uint, bool) {
	value, exists := c.Get("merchant_id")
	if !exists || value == nil {
		return 0, false
	}

	switch v := value.(type) {
	case *uint:
		if v == nil {
			return 0, false
		}
		return *v, true
	case uint:
		return v, true
	default:
		return 0, false
	}
}
