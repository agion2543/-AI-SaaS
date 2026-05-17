package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type MerchantController struct {
	merchant *service.MerchantService
	admin    *service.AdminService
	alipay   *service.AlipayService
	audit    *service.AuditService
}

func NewMerchantController(merchant *service.MerchantService, admin *service.AdminService, alipay *service.AlipayService, audit *service.AuditService) *MerchantController {
	return &MerchantController{merchant: merchant, admin: admin, alipay: alipay, audit: audit}
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
	utils.Success(c, gin.H{"list": leads, "total": total})
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

func (ctl *MerchantController) SavePaymentConfig(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req dto.SaveMerchantPaymentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "payment account info is required")
		return
	}
	config, err := ctl.merchant.SavePaymentConfig(merchantID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"config": config, "updated": true})
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
	config, err := ctl.merchant.ReviewPaymentConfig(merchantID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_payment_config_review",
		TargetType: "merchant_payment_config",
		TargetID:   config.ID,
		TargetName: config.AccountName,
		MerchantID: &merchantID,
		Detail: gin.H{
			"audit_status": req.AuditStatus,
			"status":       req.Status,
			"audit_remark": req.AuditRemark,
		},
	})
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
	if err := ctl.merchant.DisableStore(merchantID, storeID); err != nil {
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
	list, total, err := ctl.merchant.ListStoreProducts(merchantID, storeID, page, pageSize)
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
	if err := ctl.merchant.DisableStoreProduct(merchantID, productID); err != nil {
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
		utils.Error(c, http.StatusBadRequest, err.Error())
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
		utils.Error(c, http.StatusBadRequest, err.Error())
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
		utils.Error(c, http.StatusBadRequest, err.Error())
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
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, result)
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
	if err := ctl.merchant.DeletePromotion(merchantID, promotionID); err != nil {
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
			"created_at":     store.CreatedAt,
			"updated_at":     store.UpdatedAt,
		},
		"merchant": gin.H{
			"id":            store.Merchant.ID,
			"name":          store.Merchant.Name,
			"contact_phone": store.Merchant.ContactPhone,
			"contact_email": store.Merchant.ContactEmail,
			"status":        store.Merchant.Status,
		},
		"promotions": promotions,
	})
}

func (ctl *MerchantController) CreateCustomerLead(c *gin.Context) {
	storeID := uint(atoi(c.Param("id")))
	var req dto.CreateCustomerLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "\u8bf7\u8f93\u5165 11 \u4f4d\u624b\u673a\u53f7")
		return
	}

	lead, err := ctl.merchant.CreateCustomerLead(storeID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	utils.Success(c, gin.H{"lead": lead, "submitted": true})
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
