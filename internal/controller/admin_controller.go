package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/repository"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type AdminController struct {
	admin     *service.AdminService
	dashboard *service.DashboardService
	audit     *service.AuditService
}

func NewAdminController(admin *service.AdminService, dashboard *service.DashboardService, audit *service.AuditService) *AdminController {
	return &AdminController{admin: admin, dashboard: dashboard, audit: audit}
}

func (ctl *AdminController) Dashboard(c *gin.Context) {
	data, err := ctl.dashboard.SummaryWithRange(c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, data)
}

func (ctl *AdminController) AuditLogs(c *gin.Context) {
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	logs, total, err := ctl.audit.List(service.AuditListFilter{
		Action:       c.Query("action"),
		ActorType:    c.Query("actor_type"),
		TargetType:   c.Query("target_type"),
		ReviewStatus: c.Query("review_status"),
		Keyword:      c.Query("keyword"),
		MerchantID:   uint(atoi(c.Query("merchant_id"))),
	}, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": logs, "total": total})
}

func (ctl *AdminController) ReviewAuditLog(c *gin.Context) {
	var req struct {
		Remark string `json:"remark"`
	}
	_ = c.ShouldBindJSON(&req)
	logID := uint(atoi(c.Param("id")))
	log, err := ctl.audit.MarkReviewed(logID, c.GetUint("user_id"), req.Remark)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"log": log, "reviewed": true})
}

func (ctl *AdminController) Users(c *gin.Context) {
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	users, total, err := ctl.admin.ListUsersFiltered(page, pageSize, repository.UserListFilter{
		Keyword:     c.Query("keyword"),
		MemberLevel: c.Query("member_level"),
		Status:      c.Query("status"),
		Role:        c.Query("role"),
		SortBy:      c.Query("sort_by"),
		SortOrder:   c.Query("sort_order"),
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": users, "total": total})
}

func (ctl *AdminController) Customers(c *gin.Context) {
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	customers, total, err := ctl.admin.ListCustomerProfiles(page, pageSize, service.CustomerProfileFilter{
		Keyword:   c.Query("keyword"),
		AITag:     c.Query("ai_tag"),
		Status:    c.Query("status"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": customers, "total": total})
}

func (ctl *AdminController) Merchants(c *gin.Context) {
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	merchants, total, err := ctl.admin.ListMerchants(page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": merchants, "total": total})
}

func (ctl *AdminController) MerchantPlans(c *gin.Context) {
	plans, err := ctl.admin.ListMerchantPlans()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, plans)
}

func (ctl *AdminController) SaveMerchantPlan(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.SaveMerchantPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	plan, err := ctl.admin.SaveMerchantPlan(id, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"plan": plan})
}

func (ctl *AdminController) MerchantDetail(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	merchant, err := ctl.admin.GetMerchant(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, merchant)
}

func (ctl *AdminController) MerchantSubscriptionOrders(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	orders, err := ctl.admin.ListMerchantSubscriptionOrders(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": orders})
}

func (ctl *AdminController) MerchantStoreOrders(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	orders, err := ctl.admin.ListMerchantStoreOrders(id, atoi(c.DefaultQuery("limit", "10")))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": orders})
}

func (ctl *AdminController) MerchantSettlementPrepare(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	data, err := ctl.admin.PrepareMerchantSettlement(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, data)
}

func (ctl *AdminController) MerchantSettlements(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	list, err := ctl.admin.ListMerchantSettlements(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": list})
}

func (ctl *AdminController) CreateMerchantSettlement(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.CreateMerchantSettlementRequest
	_ = c.ShouldBindJSON(&req)
	settlement, orders, err := ctl.admin.CreateMerchantSettlement(id, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_settlement_create",
		TargetType: "merchant_settlement",
		TargetID:   settlement.ID,
		MerchantID: &id,
		Detail: gin.H{
			"order_count": settlement.OrderCount,
			"net_amount":  settlement.NetAmountCents,
			"remark":      req.Remark,
		},
	})
	utils.Success(c, gin.H{"settlement": settlement, "orders": orders})
}

func (ctl *AdminController) MarkMerchantSettlementPaid(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.MarkMerchantSettlementPaidRequest
	_ = c.ShouldBindJSON(&req)
	settlement, err := ctl.admin.MarkMerchantSettlementPaid(id, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_settlement_paid",
		TargetType: "merchant_settlement",
		TargetID:   settlement.ID,
		MerchantID: &settlement.MerchantID,
		Detail: gin.H{
			"net_amount": settlement.NetAmountCents,
			"remark":     req.Remark,
		},
	})
	utils.Success(c, gin.H{"settlement": settlement})
}

func (ctl *AdminController) ExportMerchantSettlement(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	settlement, orders, err := ctl.admin.GetMerchantSettlement(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=merchant-settlement-"+strconv.Itoa(int(settlement.ID))+".xlsx")
	headers := []string{"Settlement ID", "Merchant", "Order No", "Store", "Paid Amount (Yuan)", "Refund Amount (Yuan)", "Net Income (Yuan)", "Order Status", "Created At"}
	rows := make([][]string, 0, len(orders))
	for _, order := range orders {
		total := order.TotalAmount
		if total <= 0 {
			total = order.Amount
		}
		net := total - order.RefundedAmount
		if net < 0 {
			net = 0
		}
		storeName := ""
		if order.Store != nil {
			storeName = order.Store.Name
		}
		merchantName := ""
		if settlement.Merchant.Name != "" {
			merchantName = settlement.Merchant.Name
		}
		rows = append(rows, []string{
			strconv.Itoa(int(settlement.ID)),
			merchantName,
			order.OrderNo,
			storeName,
			utils.FenToYuan(total),
			utils.FenToYuan(order.RefundedAmount),
			utils.FenToYuan(net),
			order.Status,
			order.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	if err := utils.WriteXLSX(c.Writer, "Merchant Settlement", headers, rows); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
}

func (ctl *AdminController) MerchantStores(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	stores, total, err := ctl.admin.ListMerchantStores(id, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": stores, "total": total})
}

func (ctl *AdminController) CreateMerchantStore(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.AdminSaveStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	store, err := ctl.admin.SaveMerchantStore(id, 0, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, store)
}

func (ctl *AdminController) UpdateMerchantStore(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	storeID := uint(atoi(c.Param("store_id")))
	var req dto.AdminSaveStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	store, err := ctl.admin.SaveMerchantStore(id, storeID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, store)
}

func (ctl *AdminController) DisableMerchantStore(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	storeID := uint(atoi(c.Param("store_id")))
	if err := ctl.admin.DisableMerchantStore(id, storeID); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"updated": true})
}

func (ctl *AdminController) MerchantLeads(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	leads, total, err := ctl.admin.ListMerchantLeads(id, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": leads, "total": total})
}

func (ctl *AdminController) MerchantLeadStats(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	stats, err := ctl.admin.MerchantLeadStats(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, stats)
}

func (ctl *AdminController) MerchantAIInsights(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	data, err := ctl.admin.GenerateMerchantInsights(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, data)
}

func (ctl *AdminController) UpdateMerchantLead(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	leadID := uint(atoi(c.Param("lead_id")))
	var req dto.UpdateCustomerLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	lead, err := ctl.admin.UpdateMerchantLead(id, leadID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"lead": lead, "updated": true})
}

func (ctl *AdminController) UpdateMerchantStatus(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.UpdateMerchantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := ctl.admin.UpdateMerchantStatus(id, req.Status); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	merchant, _ := ctl.admin.GetMerchant(id)
	targetName := ""
	if merchant != nil {
		targetName = merchant.Name
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_status_update",
		TargetType: "merchant",
		TargetID:   id,
		TargetName: targetName,
		MerchantID: &id,
		Detail:     gin.H{"status": req.Status},
	})
	utils.Success(c, gin.H{"updated": true})
}

func (ctl *AdminController) MerchantFollowUps(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	rows, err := ctl.admin.ListMerchantFollowUps(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": rows})
}

func (ctl *AdminController) MerchantFollowUpTodos(c *gin.Context) {
	rows, err := ctl.admin.ListAllMerchantFollowUps(c.Query("status"), c.Query("priority"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": rows})
}

func (ctl *AdminController) CreateMerchantFollowUp(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.CreateMerchantFollowUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	role, _ := c.Get("role")
	operatorRole, _ := role.(string)
	row, err := ctl.admin.CreateMerchantFollowUp(id, c.GetUint("user_id"), operatorRole, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	merchant, _ := ctl.admin.GetMerchant(id)
	targetName := ""
	if merchant != nil {
		targetName = merchant.Name
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_follow_up_create",
		TargetType: "merchant_follow_up",
		TargetID:   row.ID,
		TargetName: targetName,
		MerchantID: &id,
		Detail: gin.H{
			"type":           row.Type,
			"priority":       row.Priority,
			"content":        row.Content,
			"source":         row.Source,
			"source_id":      row.SourceID,
			"order_id":       row.OrderID,
			"order_no":       row.OrderNo,
			"next_follow_at": row.NextFollowAt,
		},
	})
	utils.Success(c, gin.H{"follow_up": row})
}

func (ctl *AdminController) UpdateMerchantFollowUpStatus(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.UpdateMerchantFollowUpStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.UpdateMerchantFollowUpStatus(id, req.Status)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_follow_up_status_update",
		TargetType: "merchant_follow_up",
		TargetID:   row.ID,
		MerchantID: &row.MerchantID,
		Detail:     gin.H{"status": req.Status},
	})
	utils.Success(c, gin.H{"follow_up": row, "updated": true})
}

func (ctl *AdminController) OpenMerchantSubscription(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.OpenMerchantSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	merchant, err := ctl.admin.OpenMerchantSubscription(id, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_subscription_open",
		TargetType: "merchant",
		TargetID:   id,
		TargetName: merchant.Name,
		MerchantID: &id,
		Detail: gin.H{
			"plan":          req.Plan,
			"duration_days": req.DurationDays,
			"note":          req.Note,
			"expire_at":     merchant.SubscriptionExpireAt,
		},
	})
	utils.Success(c, gin.H{"merchant": merchant})
}

func (ctl *AdminController) StopMerchantSubscription(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	merchant, err := ctl.admin.StopMerchantSubscription(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "merchant_subscription_stop",
		TargetType: "merchant",
		TargetID:   id,
		TargetName: merchant.Name,
		MerchantID: &id,
	})
	utils.Success(c, gin.H{"merchant": merchant})
}

func (ctl *AdminController) UpdateUserStatus(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := ctl.admin.UpdateUserStatus(id, req.Status); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "user_status_update",
		TargetType: "user",
		TargetID:   id,
		Detail:     gin.H{"status": req.Status},
	})
	utils.Success(c, gin.H{"updated": true})
}

func (ctl *AdminController) ManualOpenMember(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.ManualOpenMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	user, err := ctl.admin.AdjustUserMember(id, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.audit.Record(c, service.AuditRecordInput{
		Action:     "user_member_adjust",
		TargetType: "user",
		TargetID:   id,
		TargetName: user.DisplayName,
		Detail: gin.H{
			"package_id":    req.PackageID,
			"member_level":  req.MemberLevel,
			"duration_days": req.DurationDays,
			"expired_at":    req.ExpiredAt,
			"quota_delta":   req.QuotaDelta,
			"auto_renew":    req.AutoRenew,
		},
	})
	utils.Success(c, gin.H{"updated": true, "user": user})
}

func (ctl *AdminController) UpdateUserPhone(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.UpdateUserPhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := ctl.admin.UpdateUserPhone(id, req.Phone); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"updated": true})
}

func (ctl *AdminController) GenerateCards(c *gin.Context) {
	var req dto.BatchCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	cards, err := ctl.admin.GenerateCards(req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, cards)
}

func (ctl *AdminController) Cards(c *gin.Context) {
	list, err := ctl.admin.ListCards()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, list)
}

func (ctl *AdminController) SaveConfigs(c *gin.Context) {
	var req dto.SaveSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := ctl.admin.SaveSystemConfig(req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"saved": true})
}

func (ctl *AdminController) Configs(c *gin.Context) {
	list, err := ctl.admin.ListSystemConfigs()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, list)
}

func (ctl *AdminController) UploadPaymentQRCode(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "请选择要上传的收款码图片")
		return
	}
	if file.Size > 3*1024*1024 {
		utils.Error(c, http.StatusBadRequest, "收款码图片不能超过 3MB")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowed[ext] {
		utils.Error(c, http.StatusBadRequest, "仅支持 jpg、png、webp 图片")
		return
	}

	dir := filepath.Join("uploads", "platform-payment")
	if err := os.MkdirAll(dir, 0755); err != nil {
		utils.Error(c, http.StatusInternalServerError, "创建上传目录失败")
		return
	}
	filename := fmt.Sprintf("platform_qr_%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(dir, filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		utils.Error(c, http.StatusInternalServerError, "收款码保存失败")
		return
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/uploads/platform-payment/%s", scheme, c.Request.Host, filename)
	utils.Success(c, gin.H{"url": url})
}

func (ctl *AdminController) SecurityCheck(c *gin.Context) {
	utils.Success(c, ctl.admin.SecurityCheck())
}

func (ctl *AdminController) AIConfigStatus(c *gin.Context) {
	utils.Success(c, ctl.admin.AIConfigStatus())
}

func (ctl *AdminController) AIUsageOverview(c *gin.Context) {
	data, err := ctl.admin.AIUsageOverview()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, data)
}

func atoi(v string) int {
	value, _ := strconv.Atoi(v)
	return value
}
