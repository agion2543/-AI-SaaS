package controller

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	var request struct {
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid coupon lookup request")
		return
	}
	storeID := uint(atoi(c.Param("storeId")))
	coupons, err := ctl.service.PublicStoreCoupons(storeID, request.Phone)
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
	accessToken := order.CustomerAccessToken
	publicOrder := serializePublicCustomerOrder(order)
	if order.TotalAmount == 0 {
		utils.Success(c, gin.H{"order": publicOrder, "access_token": accessToken, "payment_required": false})
		return
	}

	if req.PaymentChannel != "" && req.PaymentChannel != "alipay" {
		utils.Success(c, gin.H{"order": publicOrder, "access_token": accessToken})
		return
	}
	if order.Status != "pending" {
		utils.Success(c, gin.H{"order": publicOrder, "access_token": accessToken, "payment_required": false})
		return
	}

	payment, err := ctl.alipay.BuildCheckoutPayload(order, req.PayMode, accessToken)
	if err != nil {
		utils.Success(c, gin.H{
			"order":         publicOrder,
			"access_token":  accessToken,
			"payment_error": err.Error(),
			"retry_path":    "/customer/orders/" + url.PathEscape(order.OrderNo) + "#access_token=" + url.QueryEscape(accessToken),
		})
		return
	}

	utils.Success(c, gin.H{"order": publicOrder, "access_token": accessToken, "payment": payment})
}

func (ctl *OrderController) RecoverCustomerOrder(c *gin.Context) {
	var req dto.RecoverCustomerOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	order, err := ctl.service.RecoverCustomerStoreOrder(req.RequestKey, req.AccessToken)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "order not found")
		return
	}
	utils.Success(c, gin.H{"order": serializePublicCustomerOrder(order), "access_token": req.AccessToken, "recovered": true})
}

func (ctl *OrderController) CustomerOrderDetail(c *gin.Context) {
	order, err := ctl.service.PublicOrderDetail(c.Param("orderNo"), c.GetHeader("X-Order-Access-Token"))
	if err != nil {
		utils.Error(c, http.StatusNotFound, err.Error())
		return
	}
	paymentConfig, err := ctl.service.PublicPaymentConfigForOrder(order)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializePublicCustomerOrder(order), "payment_config": serializePublicPaymentConfig(paymentConfig)})
}

func (ctl *OrderController) MarkCustomerOrderPaid(c *gin.Context) {
	order, err := ctl.service.MarkCustomerStoreOrderPaid(c.Param("orderNo"), c.GetHeader("X-Order-Access-Token"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	paymentConfig, _ := ctl.service.PublicPaymentConfigForOrder(order)
	utils.Success(c, gin.H{"order": serializePublicCustomerOrder(order), "payment_config": serializePublicPaymentConfig(paymentConfig), "marked": true})
}

func (ctl *OrderController) AppendCustomerOrderItems(c *gin.Context) {
	var req dto.AppendCustomerOrderItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	order, err := ctl.service.AppendCustomerStoreOrderItems(c.Param("orderNo"), c.GetHeader("X-Order-Access-Token"), req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	paymentConfig, _ := ctl.service.PublicPaymentConfigForOrder(order)
	utils.Success(c, gin.H{"order": serializePublicCustomerOrder(order), "payment_config": serializePublicPaymentConfig(paymentConfig), "appended": true})
}

func (ctl *OrderController) RecoverCustomerOrderAppend(c *gin.Context) {
	var req dto.RecoverCustomerOrderAppendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	order, err := ctl.service.RecoverCustomerStoreOrderAppend(c.Param("orderNo"), c.GetHeader("X-Order-Access-Token"), req.RequestKey)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "order not found")
		return
	}
	utils.Success(c, gin.H{"order": serializePublicCustomerOrder(order), "recovered": true})
}

func (ctl *OrderController) RetryCustomerOrderPayment(c *gin.Context) {
	var req dto.RetryCustomerOrderPaymentRequest
	_ = c.ShouldBindJSON(&req)

	accessToken := c.GetHeader("X-Order-Access-Token")
	order, err := ctl.service.PrepareCustomerOrderRetry(c.Param("orderNo"), accessToken)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	order.CustomerAccessToken = accessToken
	payment, err := ctl.alipay.BuildCheckoutPayload(order, req.PayMode, accessToken)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"order": serializePublicCustomerOrder(order), "payment": payment})
}

func (ctl *OrderController) ReviewCustomerOrder(c *gin.Context) {
	var req dto.ReviewCustomerOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	order, err := ctl.service.ReviewCustomerOrder(c.Param("orderNo"), c.GetHeader("X-Order-Access-Token"), req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializePublicCustomerOrder(order), "reviewed": true})
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
			amount, err := utils.YuanToFen(values.Get("total_amount"))
			if err != nil {
				c.String(http.StatusBadRequest, "failure")
				return
			}
			transactionNo := values.Get("trade_no")
			if transactionNo == "" {
				transactionNo = values.Get("out_trade_no") + "-closed"
			}
			if _, err := ctl.service.HandlePaymentCallback(dto.PaymentCallbackRequest{
				OrderNo:        values.Get("out_trade_no"),
				TransactionNo:  transactionNo,
				PaymentChannel: "alipay",
				Amount:         amount,
				Status:         "failed",
				RawPayload:     values,
			}); err != nil {
				c.String(http.StatusBadRequest, "failure")
				return
			}
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

	utils.Success(c, gin.H{"list": serializeMerchantOrders(list), "total": total})
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
	if err := ctl.audit.RecordSensitiveRead(c, "merchant_order_detail_view", "order", order.ID, merchantID, "履约与售后查询"); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to record sensitive access audit")
		return
	}
	utils.Success(c, gin.H{"order": serializeMerchantOrderDetail(order)})
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
	order, err := ctl.service.ConfirmMerchantOrderPayment(merchantID, orderID, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_order_payment_confirm", TargetType: "order"},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializeMerchantOrderSummary(order), "confirmed": true})
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
	utils.Success(c, gin.H{"order": serializeMerchantOrderSummary(order), "updated": true})
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
	order, err := ctl.service.RefundMerchantOrder(merchantID, orderID, req, c.GetUint("user_id"), "merchant", service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_order_refund", TargetType: "order", Detail: gin.H{
			"amount": req.Amount, "full_refund": req.Full, "reason": req.Reason,
		}},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializeMerchantOrderSummary(order), "refunded": true})
}

func (ctl *OrderController) RefundAdminOrder(c *gin.Context) {
	var req dto.RefundOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	orderID := uint(atoi(c.Param("id")))
	order, err := ctl.service.RefundAdminOrder(orderID, req, c.GetUint("user_id"), service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "admin_order_refund", TargetType: "order", Detail: gin.H{
			"amount": req.Amount, "full_refund": req.Full, "reason": req.Reason,
		}},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializeAdminOrder(order, false), "refunded": true})
}

func (ctl *OrderController) ListAll(c *gin.Context) {
	includeContact, allowed := authorizeAdminCustomerContactRequest(c)
	if !allowed {
		return
	}
	orders, err := ctl.service.AllOrders()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if includeContact && !recordAdminCustomerContactRead(c, ctl.audit, "platform_orders", 0, "平台订单联系方式查看") {
		return
	}
	utils.Success(c, serializeAdminOrders(orders, includeContact))
}

func (ctl *OrderController) Payments(c *gin.Context) {
	list, err := ctl.service.AllPayments()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, serializeAdminPayments(list))
}

func (ctl *OrderController) ConfirmMerchantSubscriptionPayment(c *gin.Context) {
	var req dto.ConfirmSubscriptionPaymentRequest
	_ = c.ShouldBindJSON(&req)
	orderID := uint(atoi(c.Param("id")))
	order, err := ctl.service.ConfirmMerchantSubscriptionPayment(orderID, req.Remark, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_subscription_payment_confirm", TargetType: "order", Detail: gin.H{"remark": req.Remark}},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": serializeAdminOrder(order, false), "confirmed": true})
}

func (ctl *OrderController) ManualReviewLegacyMerchantSubscription(c *gin.Context) {
	var req dto.ManualReviewLegacyMerchantSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	roleValue, _ := c.Get("role")
	role, _ := roleValue.(string)
	result, err := ctl.service.ReviewLegacyMerchantSubscriptionOrder(
		uint(atoi(c.Param("id"))),
		req,
		service.ManualSubscriptionReviewActor{ID: c.GetUint("user_id"), Role: role},
	)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{
		"order":                      serializeAdminOrder(result.Order, false),
		"manual_review":              result.Review,
		"audit":                      result.Audit,
		"idempotent":                 result.Idempotent,
		"external_payment_attested":  result.Review.PaymentOccurredAttested,
		"provider_payment_confirmed": false,
	})
}

func (ctl *OrderController) ManualSubscriptionReviews(c *gin.Context) {
	orders, err := ctl.service.AllOrders()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	list := make([]dto.ManualSubscriptionReviewOrder, 0)
	for _, order := range orders {
		if order.OrderType != "merchant_subscription" || order.Status != "manually_verified" || order.ManualReview == nil || order.ManualReviewAudit == nil {
			continue
		}
		review := order.ManualReview
		audit := order.ManualReviewAudit
		item := dto.ManualSubscriptionReviewOrder{
			OrderID:           order.ID,
			OrderNo:           order.OrderNo,
			MerchantID:        order.MerchantID,
			Status:            order.Status,
			SubscriptionEndAt: order.SubscriptionEndAt,
			CreatedAt:         order.CreatedAt,
			ManualReview: dto.ManualSubscriptionReviewRecord{
				ID:                      review.ID,
				OrderID:                 review.OrderID,
				OrderNo:                 review.OrderNo,
				MerchantID:              review.MerchantID,
				ActorID:                 review.ActorID,
				ActorRole:               review.ActorRole,
				PlanName:                review.PlanName,
				AmountCents:             review.AmountCents,
				DurationDays:            review.DurationDays,
				PaymentOccurredAttested: review.PaymentOccurredAttested,
				EvidenceReference:       review.EvidenceReference,
				Reason:                  review.Reason,
				ReviewStatus:            review.ReviewStatus,
				CreatedAt:               review.CreatedAt,
				UpdatedAt:               review.UpdatedAt,
			},
			ManualReviewAudit: dto.ManualSubscriptionReviewAuditRecord{
				ID:                         audit.ID,
				ReviewID:                   audit.ReviewID,
				OrderID:                    audit.OrderID,
				MerchantID:                 audit.MerchantID,
				ActorID:                    audit.ActorID,
				ActorRole:                  audit.ActorRole,
				EvidenceReference:          audit.EvidenceReference,
				Reason:                     audit.Reason,
				PlanName:                   audit.PlanName,
				AmountCents:                audit.AmountCents,
				DurationDays:               audit.DurationDays,
				PaymentOccurredAttested:    audit.PaymentOccurredAttested,
				OrderStatusBefore:          audit.OrderStatusBefore,
				OrderStatusAfter:           audit.OrderStatusAfter,
				OrderPaidAtBefore:          audit.OrderPaidAtBefore,
				OrderPaidAtAfter:           audit.OrderPaidAtAfter,
				OrderUpdatedAtBefore:       audit.OrderUpdatedAtBefore,
				OrderUpdatedAtAfter:        audit.OrderUpdatedAtAfter,
				SubscriptionStatusBefore:   audit.SubscriptionStatusBefore,
				SubscriptionStatusAfter:    audit.SubscriptionStatusAfter,
				SubscriptionPlanBefore:     audit.SubscriptionPlanBefore,
				SubscriptionPlanAfter:      audit.SubscriptionPlanAfter,
				SubscriptionExpireAtBefore: audit.SubscriptionExpireAtBefore,
				SubscriptionExpireAtAfter:  audit.SubscriptionExpireAtAfter,
				ReviewedAt:                 audit.ReviewedAt,
				CreatedAt:                  audit.CreatedAt,
			},
		}
		if order.Merchant != nil && order.MerchantID != nil {
			item.Merchant = &dto.ManualReviewMerchantSummary{ID: *order.MerchantID, Name: order.Merchant.Name}
		}
		list = append(list, item)
	}
	utils.Success(c, gin.H{"list": list})
}

func (ctl *OrderController) Refunds(c *gin.Context) {
	list, err := ctl.service.AllRefunds()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, serializeAdminRefunds(list))
}

func (ctl *OrderController) AdminUnknownRefundReservations(c *gin.Context) {
	list, err := ctl.service.UnknownRefundReservations()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": list})
}

func (ctl *OrderController) AdminRefundReconciliationDetail(c *gin.Context) {
	detail, err := ctl.service.RefundReconciliationDetail(uint(atoi(c.Param("id"))))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "refund reconciliation not found")
		return
	}
	utils.Success(c, gin.H{"detail": detail})
}

func (ctl *OrderController) MarkStaleAdminRefundForManualReview(c *gin.Context) {
	var req dto.RefundManualReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	detail, err := ctl.service.MarkStaleProcessingRefundUnknown(uint(atoi(c.Param("id"))), req.Reason, c.GetUint("user_id"), c.GetString("role"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"detail": detail})
}

func (ctl *OrderController) ResolveAdminRefundReconciliation(c *gin.Context) {
	var req dto.RefundReconciliationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	detail, err := ctl.service.ResolveUnknownRefund(uint(atoi(c.Param("id"))), req, c.GetUint("user_id"), c.GetString("role"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"detail": detail})
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
	utils.Success(c, serializeMerchantRefunds(list))
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

func serializeMerchantOrders(orders []model.Order) []merchantOrderResponseDTO {
	rows := make([]merchantOrderResponseDTO, 0, len(orders))
	for i := range orders {
		rows = append(rows, serializeMerchantOrderSummary(&orders[i]))
	}
	return rows
}

type merchantOrderStoreSummaryDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type merchantOrderItemDTO struct {
	ProductID              uint                     `json:"product_id"`
	Name                   string                   `json:"name"`
	Price                  int64                    `json:"price"`
	BasePrice              int64                    `json:"base_price,omitempty"`
	Quantity               int                      `json:"quantity"`
	LineAmount             int64                    `json:"line_amount,omitempty"`
	Description            string                   `json:"description,omitempty"`
	ImageURL               string                   `json:"image_url,omitempty"`
	SalesUnit              string                   `json:"sales_unit,omitempty"`
	Specification          string                   `json:"specification,omitempty"`
	MinimumOrderQuantity   int                      `json:"minimum_order_quantity,omitempty"`
	AppliedTierMinQuantity int                      `json:"applied_tier_min_quantity,omitempty"`
	PriceTiers             []model.ProductPriceTier `json:"price_tiers,omitempty"`
}

type merchantOrderResponseDTO struct {
	ID              uint                          `json:"id"`
	OrderNo         string                        `json:"order_no"`
	OrderType       string                        `json:"order_type"`
	SettlementID    *uint                         `json:"settlement_id,omitempty"`
	StoreID         *uint                         `json:"store_id,omitempty"`
	TableNo         int                           `json:"table_no"`
	TableLabel      string                        `json:"table_label,omitempty"`
	Store           *merchantOrderStoreSummaryDTO `json:"store,omitempty"`
	CustomerPhone   string                        `json:"customer_phone"`
	Amount          int64                         `json:"amount"`
	TotalAmount     int64                         `json:"total_amount"`
	DiscountAmount  int64                         `json:"discount_amount"`
	RefundedAmount  int64                         `json:"refunded_amount"`
	RefundStatus    string                        `json:"refund_status"`
	Items           []merchantOrderItemDTO        `json:"items"`
	HasCustomerNote bool                          `json:"has_customer_note"`
	HasMerchantNote bool                          `json:"has_merchant_note"`
	CustomerNote    string                        `json:"customer_note,omitempty"`
	MerchantNote    string                        `json:"merchant_note,omitempty"`
	OperationLogs   []service.OrderOperationLog   `json:"operation_logs,omitempty"`
	Status          string                        `json:"status"`
	PaymentChannel  string                        `json:"payment_channel"`
	PaidAt          *time.Time                    `json:"paid_at,omitempty"`
	CreatedAt       time.Time                     `json:"created_at"`
	UpdatedAt       time.Time                     `json:"updated_at"`
	CanRefund       bool                          `json:"can_refund"`
}

func serializeMerchantOrderSummary(order *model.Order) merchantOrderResponseDTO {
	if order == nil {
		return merchantOrderResponseDTO{Items: []merchantOrderItemDTO{}}
	}
	row := merchantOrderResponseDTO{
		ID:              order.ID,
		OrderNo:         order.OrderNo,
		OrderType:       order.OrderType,
		SettlementID:    order.SettlementID,
		StoreID:         order.StoreID,
		TableNo:         order.TableNo,
		TableLabel:      order.TableLabel,
		CustomerPhone:   utils.MaskPhone(order.CustomerPhone),
		Amount:          order.Amount,
		TotalAmount:     order.TotalAmount,
		DiscountAmount:  order.DiscountAmount,
		RefundedAmount:  order.RefundedAmount,
		RefundStatus:    order.RefundStatus,
		Items:           serializeMerchantOrderItems(order.Items, false),
		HasCustomerNote: strings.TrimSpace(order.CustomerNote) != "",
		HasMerchantNote: strings.TrimSpace(order.MerchantNote) != "",
		Status:          order.Status,
		PaymentChannel:  order.PaymentChannel,
		PaidAt:          order.PaidAt,
		CreatedAt:       order.CreatedAt,
		UpdatedAt:       order.UpdatedAt,
		CanRefund:       merchantOrderCanRefund(order),
	}
	if order.Store != nil {
		row.Store = &merchantOrderStoreSummaryDTO{ID: order.Store.ID, Name: order.Store.Name}
	}
	return row
}

func serializeMerchantOrderDetail(order *model.Order) merchantOrderResponseDTO {
	row := serializeMerchantOrderSummary(order)
	if order == nil {
		return row
	}
	row.CustomerPhone = order.CustomerPhone
	row.CustomerNote = order.CustomerNote
	row.MerchantNote = order.MerchantNote
	row.Items = serializeMerchantOrderItems(order.Items, true)
	row.OperationLogs = serializeOrderOperationLogs(order.OperationLogs)
	return row
}

func serializeMerchantOrderItems(raw string, includeDetails bool) []merchantOrderItemDTO {
	snapshots := parseOrderItems(raw)
	items := make([]merchantOrderItemDTO, 0, len(snapshots))
	for _, snapshot := range snapshots {
		item := merchantOrderItemDTO{
			ProductID:              snapshot.ProductID,
			Name:                   snapshot.Name,
			Price:                  snapshot.Price,
			BasePrice:              snapshot.BasePrice,
			Quantity:               snapshot.Quantity,
			SalesUnit:              snapshot.SalesUnit,
			Specification:          snapshot.Specification,
			MinimumOrderQuantity:   snapshot.MinimumOrderQuantity,
			AppliedTierMinQuantity: snapshot.AppliedTierMinQuantity,
			PriceTiers:             snapshot.PriceTiers,
		}
		if includeDetails {
			item.LineAmount = snapshot.LineAmount
			item.Description = snapshot.Description
			item.ImageURL = snapshot.ImageURL
		}
		items = append(items, item)
	}
	return items
}

func merchantOrderCanRefund(order *model.Order) bool {
	if order == nil || order.OrderType != "store_order" || order.RefundStatus == "processing" || order.PaidAt == nil || order.TotalAmount <= 0 || order.TotalAmount-order.RefundedAmount <= 0 || order.PaymentChannel != "alipay" || strings.TrimSpace(order.TransactionNo) == "" {
		return false
	}
	switch order.Status {
	case "received", "accepted", "completed", "closed":
		return true
	default:
		return false
	}
}

type merchantRefundOrderSummaryDTO struct {
	ID      uint   `json:"id"`
	OrderNo string `json:"order_no"`
	StoreID *uint  `json:"store_id,omitempty"`
}

type merchantRefundResponseDTO struct {
	ID           uint                          `json:"id"`
	OrderID      uint                          `json:"order_id"`
	StoreID      *uint                         `json:"store_id,omitempty"`
	RefundNo     string                        `json:"refund_no"`
	Amount       int64                         `json:"amount"`
	Reason       string                        `json:"reason"`
	OperatorRole string                        `json:"operator_role"`
	Status       string                        `json:"status"`
	CreatedAt    time.Time                     `json:"created_at"`
	ManualRecord bool                          `json:"manual_record"`
	Order        merchantRefundOrderSummaryDTO `json:"order"`
}

func serializeMerchantRefunds(refunds []model.RefundRecord) []merchantRefundResponseDTO {
	rows := make([]merchantRefundResponseDTO, 0, len(refunds))
	for _, refund := range refunds {
		storeID := refund.StoreID
		if refund.Order.StoreID != nil {
			storeID = refund.Order.StoreID
		}
		rows = append(rows, merchantRefundResponseDTO{
			ID:           refund.ID,
			OrderID:      refund.OrderID,
			StoreID:      storeID,
			RefundNo:     refund.RefundNo,
			Amount:       refund.Amount,
			Reason:       refund.Reason,
			OperatorRole: refund.OperatorRole,
			Status:       refund.Status,
			CreatedAt:    refund.CreatedAt,
			ManualRecord: merchantRefundIsManualRecord(refund.RawPayload),
			Order: merchantRefundOrderSummaryDTO{
				ID:      refund.OrderID,
				OrderNo: refund.Order.OrderNo,
				StoreID: storeID,
			},
		})
	}
	return rows
}

func merchantRefundIsManualRecord(raw string) bool {
	var payload struct {
		Mode string `json:"mode"`
	}
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &payload) != nil {
		return false
	}
	return payload.Mode == "manual_record"
}

func serializeOrder(order *model.Order) gin.H {
	return gin.H{
		"id":                                   order.ID,
		"order_no":                             order.OrderNo,
		"order_type":                           order.OrderType,
		"user_id":                              order.UserID,
		"merchant_id":                          order.MerchantID,
		"settlement_id":                        order.SettlementID,
		"store_id":                             order.StoreID,
		"table_no":                             order.TableNo,
		"table_label":                          order.TableLabel,
		"customer_phone":                       order.CustomerPhone,
		"package_id":                           order.PackageID,
		"merchant_plan_id":                     order.MerchantPlanID,
		"merchant_plan_name_snapshot":          order.PlanNameSnapshot,
		"merchant_plan_price_cents_snapshot":   order.PlanPriceCentsSnapshot,
		"merchant_plan_duration_days_snapshot": order.PlanDurationDaysSnapshot,
		"amount":                               order.Amount,
		"total_amount":                         order.TotalAmount,
		"discount_amount":                      order.DiscountAmount,
		"refunded_amount":                      order.RefundedAmount,
		"refund_status":                        order.RefundStatus,
		"promotion_id":                         order.PromotionID,
		"referral_coupon_id":                   order.CouponID,
		"coupon_no":                            order.CouponNo,
		"items":                                parseOrderItems(order.Items),
		"customer_note":                        order.CustomerNote,
		"merchant_note":                        order.MerchantNote,
		"customer_rating":                      order.CustomerRating,
		"customer_review":                      order.CustomerReview,
		"reviewed_at":                          order.ReviewedAt,
		"operation_logs":                       serializeOrderOperationLogs(order.OperationLogs),
		"status":                               order.Status,
		"expired_unpaid":                       orderIsExpiredUnpaid(order),
		"payment_review_required":              orderRequiresManualPaymentReview(order),
		"payment_review_status":                paymentReviewStatus(order),
		"payment_channel":                      order.PaymentChannel,
		"auto_renew":                           order.AutoRenew,
		"paid_at":                              order.PaidAt,
		"transaction_no":                       order.TransactionNo,
		"subscription_end_at":                  order.SubscriptionEndAt,
		"created_at":                           order.CreatedAt,
		"updated_at":                           order.UpdatedAt,
		"user":                                 order.User,
		"merchant":                             order.Merchant,
		"store":                                order.Store,
		"package":                              order.Package,
		"merchant_plan":                        order.MerchantPlan,
		"promotion":                            order.Promotion,
		"manual_review":                        order.ManualReview,
		"manual_review_audit":                  order.ManualReviewAudit,
	}
}

func orderRequiresManualPaymentReview(order *model.Order) bool {
	if order == nil {
		return false
	}
	for _, entry := range parseOrderLogs(order.OperationLogs) {
		if entry.Action == "late_payment_exception" {
			return true
		}
	}
	return false
}

func orderIsExpiredUnpaid(order *model.Order) bool {
	if order == nil || order.Status != "closed" || order.PaidAt != nil {
		return false
	}
	for _, entry := range parseOrderLogs(order.OperationLogs) {
		if entry.Action == "checkout_expired" {
			return true
		}
	}
	return false
}

func paymentReviewStatus(order *model.Order) string {
	if orderRequiresManualPaymentReview(order) {
		return model.PaymentStatusPaidReviewRequired
	}
	return ""
}

func serializePublicCustomerOrder(order *model.Order) gin.H {
	items := make([]gin.H, 0)
	for _, item := range parseOrderItems(order.Items) {
		items = append(items, gin.H{
			"product_id":                item.ProductID,
			"name":                      item.Name,
			"product_name":              item.Name,
			"price":                     item.Price,
			"base_price":                item.BasePrice,
			"quantity":                  item.Quantity,
			"line_amount":               item.LineAmount,
			"description":               item.Description,
			"image_url":                 item.ImageURL,
			"sales_unit":                item.SalesUnit,
			"specification":             item.Specification,
			"minimum_order_quantity":    item.MinimumOrderQuantity,
			"applied_tier_min_quantity": item.AppliedTierMinQuantity,
			"price_tiers":               item.PriceTiers,
		})
	}
	logs := make([]gin.H, 0)
	for _, log := range parseOrderLogs(order.OperationLogs) {
		switch log.Action {
		case "submitted", "auto_accepted", "payment_retry", "customer_paid", "merchant_payment_confirmed", "accepted", "completed", "closed", "refund", "customer_reviewed", "items_appended":
			logs = append(logs, gin.H{"action": log.Action, "time": log.Time})
		}
	}
	var store gin.H
	if order.Store != nil {
		store = gin.H{
			"id":             order.Store.ID,
			"name":           order.Store.Name,
			"address":        order.Store.Address,
			"contact_phone":  order.Store.ContactPhone,
			"business_hours": order.Store.BusinessHours,
			"order_mode":     order.Store.OrderMode,
		}
	}
	return gin.H{
		"id":          order.ID,
		"order_no":    order.OrderNo,
		"order_type":  order.OrderType,
		"store_id":    order.StoreID,
		"table_no":    order.TableNo,
		"table_label": order.TableLabel,
		"store_name": func() string {
			if order.Store != nil {
				return order.Store.Name
			}
			return ""
		}(),
		"store":                   store,
		"amount":                  order.TotalAmount,
		"subtotal_amount":         order.Amount,
		"total_amount":            order.TotalAmount,
		"discount_amount":         order.DiscountAmount,
		"refunded_amount":         order.RefundedAmount,
		"refund_status":           order.RefundStatus,
		"items":                   items,
		"customer_note":           order.CustomerNote,
		"remark":                  order.CustomerNote,
		"customer_rating":         order.CustomerRating,
		"customer_review":         order.CustomerReview,
		"reviewed_at":             order.ReviewedAt,
		"operation_logs":          logs,
		"status":                  order.Status,
		"expire_time":             service.CheckoutOrderExpireTime(order),
		"expired_unpaid":          orderIsExpiredUnpaid(order),
		"payment_review_required": orderRequiresManualPaymentReview(order),
		"payment_review_status":   paymentReviewStatus(order),
		"payment_channel":         order.PaymentChannel,
		"pay_type":                order.PaymentChannel,
		"paid_at":                 order.PaidAt,
		"created_at":              order.CreatedAt,
		"updated_at":              order.UpdatedAt,
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

func serializeOrderOperationLogs(raw string) []service.OrderOperationLog {
	logs := parseOrderLogs(raw)
	for i := range logs {
		logs[i].AppendRequestKeyHash = ""
		logs[i].AppendRequestFingerprint = ""
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
