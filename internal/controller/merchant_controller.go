package controller

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type MerchantController struct {
	merchant  *service.MerchantService
	admin     *service.AdminService
	alipay    *service.AlipayService
	audit     *service.AuditService
	paymentQR *merchantPaymentQRStore
	appURL    string
}

func NewMerchantController(merchant *service.MerchantService, admin *service.AdminService, alipay *service.AlipayService, audit *service.AuditService, appURLs ...string) *MerchantController {
	appURL := ""
	if len(appURLs) > 0 {
		appURL = appURLs[0]
	}
	return &MerchantController{merchant: merchant, admin: admin, alipay: alipay, audit: audit, paymentQR: newDefaultMerchantPaymentQRStore(), appURL: appURL}
}

func (ctl *MerchantController) Register(c *gin.Context) {
	var req dto.MerchantRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "\u8bf7\u5b8c\u6574\u586b\u5199\u5546\u5bb6\u6ce8\u518c\u4fe1\u606f")
		return
	}

	merchant, user, token, err := ctl.merchant.Register(req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{
		"merchant": merchant,
		"user":     sanitizeAuthUser(user),
		"token":    token,
	})
}

func (ctl *MerchantController) SendRegisterCode(c *gin.Context) {
	var req dto.SendSMSCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "\u8bf7\u8f93\u5165 11 \u4f4d\u624b\u673a\u53f7")
		return
	}

	if err := ctl.merchant.SendRegisterCode(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"sent": true, "scene": "merchant_register"})
}

func (ctl *MerchantController) Login(c *gin.Context) {
	var req dto.MerchantLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "\u8bf7\u8f93\u5165\u624b\u673a\u53f7\u548c\u5bc6\u7801")
		return
	}

	merchant, user, token, err := ctl.merchant.Login(req)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	utils.Success(c, gin.H{
		"merchant": merchant,
		"user":     sanitizeAuthUser(user),
		"token":    token,
	})
}

func (ctl *MerchantController) SendPasswordResetCode(c *gin.Context) {
	var req dto.MerchantSendPasswordResetCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "璇疯緭鍏?11 浣嶆墜鏈哄彿")
		return
	}
	if err := ctl.merchant.SendPasswordResetCode(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"sent": true, "scene": "merchant_reset_password"})
}

func (ctl *MerchantController) ResetPassword(c *gin.Context) {
	var req dto.MerchantResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "璇峰畬鏁村～鍐欐墜鏈哄彿銆侀獙璇佺爜鍜屾柊瀵嗙爜")
		return
	}
	if err := ctl.merchant.ResetPassword(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"reset": true})
}

func (ctl *MerchantController) ChangePassword(c *gin.Context) {
	var req dto.MerchantChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "璇峰畬鏁村～鍐欏師瀵嗙爜鍜屾柊瀵嗙爜")
		return
	}
	if err := ctl.merchant.ChangePassword(c.GetUint("user_id"), req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"changed": true})
}

func (ctl *MerchantController) Info(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	merchant, err := ctl.merchant.GetInfo(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"merchant": merchant})
}

func (ctl *MerchantController) MaintenanceNotice(c *gin.Context) {
	notice, err := ctl.admin.ActiveMaintenanceNotice()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "维护公告暂时无法读取")
		return
	}
	utils.Success(c, gin.H{"notice": notice})
}

func (ctl *MerchantController) Turnover(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var storeID uint
	if raw := strings.TrimSpace(c.Query("store_id")); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "store_id 无效")
			return
		}
		storeID = uint(parsed)
	}
	data, err := ctl.merchant.MerchantTurnover(merchantID, c.Query("start_date"), c.Query("end_date"), storeID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "营业额暂时无法读取")
		return
	}
	utils.Success(c, data)
}

func (ctl *MerchantController) Subscription(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	data, err := ctl.merchant.SubscriptionInfo(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, data)
}

func (ctl *MerchantController) CheckSubscription(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	valid, err := ctl.merchant.CheckSubscription(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"valid": valid})
}

func (ctl *MerchantController) CreateSubscriptionOrder(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req dto.CreateMerchantSubscriptionOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	order, err := ctl.merchant.CreateSubscriptionOrder(c.GetUint("user_id"), merchantID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.PaymentChannel != "" && req.PaymentChannel != "alipay" {
		utils.Success(c, gin.H{"order": order})
		return
	}
	payment, err := ctl.alipay.BuildCheckoutPayload(order, req.PayMode)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": order, "payment": payment})
}

func (ctl *MerchantController) SubscriptionOrderStatus(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	data, err := ctl.merchant.SubscriptionOrderStatus(merchantID, c.Param("orderNo"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, data)
}

func (ctl *MerchantController) MarkSubscriptionOrderPaid(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	order, err := ctl.merchant.MarkSubscriptionOrderPaid(merchantID, c.Param("orderNo"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"order": order, "marked": true})
}

func (ctl *MerchantController) RedeemSubscriptionCard(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req dto.MerchantRedeemCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid card code")
		return
	}
	merchant, card, err := ctl.merchant.RedeemSubscriptionCard(c.GetUint("user_id"), merchantID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"merchant": merchant, "card": card, "redeemed": true})
}

func (ctl *MerchantController) AIInsights(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	data, err := ctl.admin.GenerateMerchantInsights(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, data)
}

func (ctl *MerchantController) Leads(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	leads, total, err := ctl.admin.ListMerchantLeads(merchantID, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := ctl.audit.RecordSensitiveRead(c, "merchant_customer_lead_list_view", "customer_lead_collection", merchantID, merchantID, "客户跟进"); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to record sensitive access audit")
		return
	}
	utils.Success(c, gin.H{"list": serializeMerchantLeadSummaries(leads), "total": total})
}

type merchantLeadSummary struct {
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	Status        string `json:"status"`
}

func serializeMerchantLeadSummaries(leads []model.CustomerLead) []merchantLeadSummary {
	rows := make([]merchantLeadSummary, 0, len(leads))
	for _, lead := range leads {
		rows = append(rows, merchantLeadSummary{
			CustomerName:  lead.CustomerName,
			CustomerPhone: lead.CustomerPhone,
			Status:        lead.Status,
		})
	}
	return rows
}

func (ctl *MerchantController) LeadStats(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	stats, err := ctl.admin.MerchantLeadStats(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, stats)
}

func (ctl *MerchantController) UpdateInfo(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.UpdateMerchantInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	merchant, err := ctl.merchant.UpdateInfo(merchantID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"merchant": merchant, "updated": true})
}

// ResubmitOnboarding clears a rejection back to a clean pending state
// (P3-1). Only a pending merchant with a rejection record can resubmit.
func (ctl *MerchantController) ResubmitOnboarding(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	merchant, err := ctl.merchant.ResubmitOnboarding(merchantID, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_onboarding_resubmitted", TargetType: "merchant", MerchantID: &merchantID, Detail: gin.H{
			"status": "pending",
		}},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"merchant": merchant, "resubmitted": true})
}

// CreateSupportTicket files a merchant service request (P3-2, v18). The
// merchant identity comes from the token only; the route sits outside the
// subscription-protected group so an expired merchant can still reach
// support when it needs help most.
func (ctl *MerchantController) CreateSupportTicket(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req dto.CreateSupportTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ticket, err := ctl.merchant.CreateSupportTicket(merchantID, req, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "support_ticket_create", TargetType: "support_ticket", MerchantID: &merchantID},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"ticket": ticket})
}

// ListSupportTickets returns the merchant's own tickets, newest first
// (P3-2, v18). The scope is forced in the service layer.
func (ctl *MerchantController) ListSupportTickets(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	rows, total, err := ctl.merchant.ListSupportTickets(merchantID, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": rows, "total": total})
}

// GetSupportTicket returns one of the merchant's own tickets (P3-2,
// v18). A ticket of another merchant reads as "工单不存在".
func (ctl *MerchantController) GetSupportTicket(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	ticket, err := ctl.merchant.GetSupportTicket(merchantID, uint(atoi(c.Param("id"))))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"ticket": ticket})
}

func (ctl *MerchantController) PaymentConfig(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	config, err := ctl.merchant.PaymentConfig(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"config": config})
}

type saveMerchantPaymentConfigWithQRUploads struct {
	dto.SaveMerchantPaymentConfigRequest
	AlipayQRUploadID string `json:"alipay_qr_upload_id"`
	WechatQRUploadID string `json:"wechat_qr_upload_id"`
}

func (ctl *MerchantController) SavePaymentConfig(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req saveMerchantPaymentConfigWithQRUploads
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "payment account info is required")
		return
	}
	refs, err := ctl.paymentQR.ReserveUploads(merchantID, map[string]string{
		"alipay": req.AlipayQRUploadID,
		"wechat": req.WechatQRUploadID,
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	finalized := make([]finalizedMerchantQRCode, 0, len(refs))
	for _, ref := range refs {
		file, finalizeErr := ctl.paymentQR.Finalize(ref)
		if finalizeErr != nil {
			ctl.paymentQR.Abort(refs, finalized)
			utils.Error(c, http.StatusInternalServerError, "收款码定稿失败，配置未保存")
			return
		}
		finalized = append(finalized, file)
		qrURL := merchantQRCodeURL(ctl.appURL, file.URLPath)
		switch ref.Channel {
		case "alipay":
			req.AlipayQRCode = qrURL
		case "wechat":
			req.WechatQRCode = qrURL
		}
	}
	merchantRef := merchantID
	config, err := ctl.merchant.SavePaymentConfig(merchantID, req.SaveMerchantPaymentConfigRequest, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{
			Action:     "merchant_payment_config_save",
			TargetType: "merchant_payment_config",
			MerchantID: &merchantRef,
		},
	})
	if err != nil {
		ctl.paymentQR.Abort(refs, finalized)
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.paymentQR.Commit(refs)
	utils.Success(c, gin.H{"config": config, "updated": true})
}

func (ctl *MerchantController) UploadPaymentQRCode(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "请选择要上传的收款码图片")
		return
	}
	if file.Size > merchantQRCodeMaxSize {
		utils.Error(c, http.StatusBadRequest, "收款码图片不能超过 3MB")
		return
	}
	channel := strings.TrimSpace(c.PostForm("channel"))
	if channel != "alipay" && channel != "wechat" {
		utils.Error(c, http.StatusBadRequest, "请指定支付宝或微信收款渠道")
		return
	}
	extension := strings.ToLower(filepath.Ext(file.Filename))
	if !validMerchantQRCodeExtensions[extension] {
		utils.Error(c, http.StatusBadRequest, "仅支持 jpg、png、webp 图片")
		return
	}
	source, err := file.Open()
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "读取收款码图片失败")
		return
	}
	defer source.Close()
	id, err := ctl.paymentQR.Stage(merchantID, channel, extension, source)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{
		"upload_id":          id,
		"channel":            channel,
		"expires_in_seconds": int64(ctl.paymentQR.ttl.Seconds()),
	})
}

func (ctl *MerchantController) AdminPaymentConfig(c *gin.Context) {
	merchantID := uint(atoi(c.Param("id")))
	config, err := ctl.merchant.PaymentConfig(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"config": config})
}

func (ctl *MerchantController) ReviewPaymentConfig(c *gin.Context) {
	merchantID := uint(atoi(c.Param("id")))
	var req dto.ReviewMerchantPaymentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "audit status is required")
		return
	}
	config, err := ctl.merchant.ReviewPaymentConfig(merchantID, req, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_payment_config_review", TargetType: "merchant_payment_config", MerchantID: &merchantID, Detail: gin.H{
			"audit_status": req.AuditStatus, "status": req.Status, "audit_remark": req.AuditRemark,
		}},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"config": config, "reviewed": true})
}

func (ctl *MerchantController) Stores(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	list, total, err := ctl.merchant.ListStores(merchantID, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows := make([]gin.H, 0, len(list))
	for _, item := range list {
		rows = append(rows, gin.H{
			"id":             item.ID,
			"merchant_id":    item.MerchantID,
			"name":           item.Name,
			"address":        item.Address,
			"contact_phone":  item.ContactPhone,
			"status":         item.Status,
			"is_open":        item.IsOpen,
			"business_hours": item.BusinessHours,
			"pause_reason":   item.PauseReason,
			"order_mode":     item.OrderMode,
			"auto_accept":    item.AutoAccept,
			"qr_url":         ctl.merchant.CustomerStoreURL(item.ID),
			"created_at":     item.CreatedAt,
			"updated_at":     item.UpdatedAt,
		})
	}

	utils.Success(c, gin.H{"list": rows, "total": total})
}

func (ctl *MerchantController) CreateStore(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.CreateStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	store, err := ctl.merchant.CreateStore(merchantID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{
		"store": gin.H{
			"id":             store.ID,
			"merchant_id":    store.MerchantID,
			"name":           store.Name,
			"address":        store.Address,
			"contact_phone":  store.ContactPhone,
			"status":         store.Status,
			"is_open":        store.IsOpen,
			"business_hours": store.BusinessHours,
			"pause_reason":   store.PauseReason,
			"order_mode":     store.OrderMode,
			"auto_accept":    store.AutoAccept,
			"qr_url":         ctl.merchant.CustomerStoreURL(store.ID),
			"created_at":     store.CreatedAt,
			"updated_at":     store.UpdatedAt,
		},
		"created": true,
	})
}

func (ctl *MerchantController) UpdateStore(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.UpdateStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	storeID := uint(atoi(c.Param("id")))
	store, err := ctl.merchant.UpdateStore(merchantID, storeID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{
		"store": gin.H{
			"id":             store.ID,
			"merchant_id":    store.MerchantID,
			"name":           store.Name,
			"address":        store.Address,
			"contact_phone":  store.ContactPhone,
			"status":         store.Status,
			"is_open":        store.IsOpen,
			"business_hours": store.BusinessHours,
			"pause_reason":   store.PauseReason,
			"order_mode":     store.OrderMode,
			"auto_accept":    store.AutoAccept,
			"qr_url":         ctl.merchant.CustomerStoreURL(store.ID),
			"created_at":     store.CreatedAt,
			"updated_at":     store.UpdatedAt,
		},
		"updated": true,
	})
}

func (ctl *MerchantController) DeleteStore(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	storeID := uint(atoi(c.Param("id")))
	if err := ctl.merchant.DisableStore(merchantID, storeID, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_store_disable", TargetType: "store"},
	}); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"deleted": true})
}

func (ctl *MerchantController) StoreProducts(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	storeID := uint(atoi(c.Param("storeId")))
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	list, total, err := ctl.merchant.ListStoreProductsFiltered(merchantID, storeID, page, pageSize, c.Query("keyword"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"list": list, "total": total})
}

func (ctl *MerchantController) CreateStoreProduct(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.SaveStoreProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	product, err := ctl.merchant.SaveStoreProduct(merchantID, 0, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"product": product, "created": true})
}

func (ctl *MerchantController) UploadProductImage(c *gin.Context) {
	if _, ok := currentMerchantID(c); !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "璇烽€夋嫨瑕佷笂浼犵殑鍟嗗搧鍥剧墖")
		return
	}
	if file.Size > 3*1024*1024 {
		utils.Error(c, http.StatusBadRequest, "鍥剧墖涓嶈兘瓒呰繃 3MB")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowed[ext] {
		utils.Error(c, http.StatusBadRequest, "浠呮敮鎸?jpg銆乸ng銆亀ebp 鍥剧墖")
		return
	}

	dir := filepath.Join("uploads", "products")
	if err := os.MkdirAll(dir, 0755); err != nil {
		utils.Error(c, http.StatusInternalServerError, "鍒涘缓涓婁紶鐩綍澶辫触")
		return
	}
	filename := fmt.Sprintf("product_%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(dir, filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		utils.Error(c, http.StatusInternalServerError, "鍥剧墖淇濆瓨澶辫触")
		return
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/uploads/products/%s", scheme, c.Request.Host, filename)
	utils.Success(c, gin.H{"url": url})
}

func (ctl *MerchantController) UpdateStoreProduct(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.SaveStoreProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	productID := uint(atoi(c.Param("id")))
	product, err := ctl.merchant.SaveStoreProduct(merchantID, productID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"product": product, "updated": true})
}

func (ctl *MerchantController) DeleteStoreProduct(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	productID := uint(atoi(c.Param("id")))
	if err := ctl.merchant.DisableStoreProduct(merchantID, productID, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_product_disable", TargetType: "store_product"},
	}); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"deleted": true})
}

func (ctl *MerchantController) Promotions(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	list, total, err := ctl.merchant.ListPromotions(merchantID, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total})
}

func (ctl *MerchantController) CreatePromotion(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.SavePromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	promotion, err := ctl.merchant.SavePromotion(merchantID, 0, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"promotion": promotion, "created": true})
}

func (ctl *MerchantController) GeneratePromotionDraft(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.GeneratePromotionDraftRequest
	_ = c.ShouldBindJSON(&req)
	promotion, err := ctl.merchant.GeneratePromotionDraft(merchantID, req)
	if err != nil {
		utils.Error(c, aiUnavailableHTTPStatus(err), err.Error())
		return
	}
	utils.Success(c, gin.H{"promotion": promotion, "created": true})
}

func (ctl *MerchantController) GenerateAIMarketingCopy(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req dto.GenerateAIMarketingCopyRequest
	_ = c.ShouldBindJSON(&req)
	result, err := ctl.merchant.GenerateAIMarketingCopy(merchantID, req)
	if err != nil {
		utils.Error(c, aiUnavailableHTTPStatus(err), err.Error())
		return
	}
	quota, _ := ctl.merchant.AIQuota(merchantID)
	utils.Success(c, gin.H{"result": result, "quota": quota})
}

func (ctl *MerchantController) GenerateAIReferralCopy(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req dto.GenerateAIReferralCopyRequest
	_ = c.ShouldBindJSON(&req)
	result, err := ctl.merchant.GenerateAIReferralCopies(merchantID, req)
	if err != nil {
		utils.Error(c, aiUnavailableHTTPStatus(err), err.Error())
		return
	}
	utils.Success(c, result)
}

func (ctl *MerchantController) GenerateAIShareReview(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req dto.GenerateAIShareReviewRequest
	_ = c.ShouldBindJSON(&req)
	result, err := ctl.merchant.GenerateAIShareReview(merchantID, req)
	if err != nil {
		utils.Error(c, aiUnavailableHTTPStatus(err), err.Error())
		return
	}
	utils.Success(c, result)
}

func aiUnavailableHTTPStatus(err error) int {
	if errors.Is(err, service.ErrAIUnavailable) {
		return http.StatusServiceUnavailable
	}
	return http.StatusBadRequest
}

func (ctl *MerchantController) AIQuota(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	quota, err := ctl.merchant.AIQuota(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, quota)
}

func (ctl *MerchantController) UpdatePromotion(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}

	var req dto.SavePromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	promotionID := uint(atoi(c.Param("id")))
	promotion, err := ctl.merchant.SavePromotion(merchantID, promotionID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"promotion": promotion, "updated": true})
}

func (ctl *MerchantController) DeletePromotion(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	promotionID := uint(atoi(c.Param("id")))
	if err := ctl.merchant.DeletePromotion(merchantID, promotionID, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_promotion_delete", TargetType: "promotion"},
	}); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"deleted": true})
}

func (ctl *MerchantController) PublicStore(c *gin.Context) {
	storeID := uint(atoi(c.Param("id")))
	store, err := ctl.merchant.GetPublicStore(storeID)
	if err != nil {
		utils.Error(c, http.StatusNotFound, err.Error())
		return
	}
	promotions, err := ctl.merchant.ListPublicPromotions(storeID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	groupBuyLinks, err := ctl.merchant.ListPublicGroupBuyLinks(storeID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{
		"store": gin.H{
			"id":             store.ID,
			"merchant_id":    store.MerchantID,
			"name":           store.Name,
			"address":        store.Address,
			"contact_phone":  store.ContactPhone,
			"status":         store.Status,
			"is_open":        store.IsOpen,
			"business_hours": store.BusinessHours,
			"pause_reason":   store.PauseReason,
			"order_mode":     store.OrderMode,
			"auto_accept":    store.AutoAccept,
			"created_at":     store.CreatedAt,
			"updated_at":     store.UpdatedAt,
		},
		"merchant": gin.H{
			"id":            store.Merchant.ID,
			"name":          store.Merchant.Name,
			"contact_phone": store.Merchant.ContactPhone,
			"status":        store.Merchant.Status,
		},
		"promotions":      promotions,
		"group_buy_links": groupBuyLinks,
	})
}

func (ctl *MerchantController) CreateCustomerLead(c *gin.Context) {
	storeID := uint(atoi(c.Param("id")))
	var req dto.CreateCustomerLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "\u8bf7\u8f93\u5165 11 \u4f4d\u624b\u673a\u53f7")
		return
	}

	_, err := ctl.merchant.CreateCustomerLead(storeID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"submitted": true})
}

func currentMerchantID(c *gin.Context) (uint, bool) {
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
