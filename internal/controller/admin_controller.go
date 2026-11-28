package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/repository"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type AdminController struct {
	admin      *service.AdminService
	dashboard  *service.DashboardService
	audit      *service.AuditService
	platformQR *platformPaymentQRStore
	appURL     string
}

func NewAdminController(admin *service.AdminService, dashboard *service.DashboardService, audit *service.AuditService, appURLs ...string) *AdminController {
	appURL := "http://localhost:8080"
	if len(appURLs) > 0 {
		appURL = appURLs[0]
	}
	return &AdminController{admin: admin, dashboard: dashboard, audit: audit, platformQR: newDefaultPlatformPaymentQRStore(), appURL: appURL}
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
	includeContact, allowed := authorizeAdminCustomerContactRequest(c)
	if !allowed {
		return
	}
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
	if includeContact && !recordAdminCustomerContactRead(c, ctl.audit, "user_directory", 0, "用户目录联系方式查看") {
		return
	}
	rows := make([]gin.H, 0, len(users))
	for _, user := range users {
		rows = append(rows, serializeAdminUser(user, includeContact))
	}
	utils.Success(c, gin.H{"list": rows, "total": total})
}

func (ctl *AdminController) Customers(c *gin.Context) {
	includeContact, allowed := authorizeAdminCustomerContactRequest(c)
	if !allowed {
		return
	}
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
	if includeContact && !recordAdminCustomerContactRead(c, ctl.audit, "customer_profiles", 0, "顾客档案联系方式查看") {
		return
	}
	if !includeContact {
		for i := range customers {
			customers[i].CustomerPhone = utils.MaskPhone(customers[i].CustomerPhone)
		}
	}
	utils.Success(c, gin.H{"list": customers, "total": total})
}

func (ctl *AdminController) ExportCustomerProfiles(c *gin.Context) {
	includeContact, allowed := authorizeAdminCustomerContactRequest(c)
	if !allowed {
		return
	}
	customers, total, err := ctl.admin.ListCustomerProfiles(1, 10000, service.CustomerProfileFilter{
		Keyword: c.Query("keyword"), AITag: c.Query("ai_tag"), Status: c.Query("status"),
		SortBy: c.Query("sort_by"), SortOrder: c.Query("sort_order"),
	})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !recordAdminCustomerOrderExport(c, ctl.audit, "customer_profiles") {
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=platform-customer-profiles.xlsx")
	if total > int64(len(customers)) {
		c.Header("X-Export-Truncated", "true")
	}
	headers := []string{"Customer Phone", "Merchant", "Store", "Order Count", "Total Amount (Yuan)", "Discount Amount (Yuan)", "Last Order At", "Last Status"}
	rows := make([][]string, 0, len(customers))
	for _, customer := range customers {
		phone := customer.CustomerPhone
		if !includeContact {
			phone = utils.MaskPhone(phone)
		}
		rows = append(rows, []string{
			phone, customer.MerchantName, customer.StoreName, strconv.FormatInt(customer.OrderCount, 10),
			utils.FenToYuan(customer.TotalAmount), utils.FenToYuan(customer.DiscountAmount),
			customer.LastOrderAt.Format("2006-01-02 15:04:05"), customer.LastStatus,
		})
	}
	if total > int64(len(customers)) {
		rows = append(rows, []string{"已达到 10,000 条单次导出上限；剩余记录未导出，请缩小筛选范围后重试。"})
	}
	if err := utils.WriteXLSX(c.Writer, "Customer Profiles", headers, rows); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to generate customer export")
		return
	}
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
	plan, err := ctl.admin.SaveMerchantPlan(id, req, service.AuditActorFromContext(c))
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
	utils.Success(c, gin.H{"list": serializeAdminOrders(orders, false)})
}

func (ctl *AdminController) MerchantStoreOrders(c *gin.Context) {
	includeContact, allowed := authorizeAdminCustomerContactRequest(c)
	if !allowed {
		return
	}
	id := uint(atoi(c.Param("id")))
	orders, err := ctl.admin.ListMerchantStoreOrders(id, atoi(c.DefaultQuery("limit", "10")))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if includeContact && !recordAdminCustomerContactRead(c, ctl.audit, "merchant_orders", id, "商家维度订单联系方式查看") {
		return
	}
	utils.Success(c, gin.H{"list": serializeAdminOrders(orders, includeContact)})
}

func (ctl *AdminController) MerchantSettlementPrepare(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	data, err := ctl.admin.PrepareMerchantSettlement(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{
		"merchant_id": data.MerchantID, "order_count": data.OrderCount,
		"total_amount_cents": data.TotalAmountCents, "refund_amount_cents": data.RefundAmountCents,
		"net_amount_cents": data.NetAmountCents, "settlement_period_start": data.PeriodStart,
		"settlement_period_end": data.PeriodEnd, "orders": serializeAdminOrders(data.Orders, false),
	})
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
	settlement, orders, err := ctl.admin.CreateMerchantSettlement(id, req, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_settlement_create", TargetType: "merchant_settlement", MerchantID: &id, Detail: gin.H{"remark": req.Remark}},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"settlement": settlement, "orders": orders})
}

func (ctl *AdminController) MarkMerchantSettlementPaid(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.MarkMerchantSettlementPaidRequest
	_ = c.ShouldBindJSON(&req)
	settlement, err := ctl.admin.MarkMerchantSettlementPaid(id, req, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_settlement_paid", TargetType: "merchant_settlement", Detail: gin.H{"remark": req.Remark}},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
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
	store, err := ctl.admin.SaveMerchantStore(id, 0, req, service.AuditEvent{Actor: service.AuditActorFromContext(c)})
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
	store, err := ctl.admin.SaveMerchantStore(id, storeID, req, service.AuditEvent{Actor: service.AuditActorFromContext(c)})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, store)
}

func (ctl *AdminController) DisableMerchantStore(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	storeID := uint(atoi(c.Param("store_id")))
	if err := ctl.admin.DisableMerchantStore(id, storeID, service.AuditEvent{Actor: service.AuditActorFromContext(c)}); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"updated": true})
}

func (ctl *AdminController) MerchantLeads(c *gin.Context) {
	includeContact, allowed := authorizeAdminCustomerContactRequest(c)
	if !allowed {
		return
	}
	id := uint(atoi(c.Param("id")))
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	leads, total, err := ctl.admin.ListMerchantLeads(id, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if includeContact && !recordAdminCustomerContactRead(c, ctl.audit, "merchant_customer_leads", id, "商家顾客线索联系方式查看") {
		return
	}
	rows := make([]gin.H, 0, len(leads))
	for _, lead := range leads {
		rows = append(rows, serializeAdminCustomerLead(lead, includeContact))
	}
	utils.Success(c, gin.H{"list": rows, "total": total})
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
	includeContact, allowed := authorizeAdminCustomerContactRequest(c)
	if !allowed {
		return
	}
	id := uint(atoi(c.Param("id")))
	data, err := ctl.admin.GenerateMerchantInsights(id)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	profiles, _ := data["customer_profiles"].([]service.CustomerProfileRow)
	if includeContact {
		if !recordAdminCustomerContactRead(c, ctl.audit, "merchant_customer_profiles", id, "商家顾客画像联系方式查看") {
			return
		}
	} else {
		for i := range profiles {
			profiles[i].CustomerPhone = utils.MaskPhone(profiles[i].CustomerPhone)
		}
		data["customer_profiles"] = profiles
	}
	utils.Success(c, data)
}

func (ctl *AdminController) UpdateMerchantLead(c *gin.Context) {
	includeContact, allowed := authorizeAdminCustomerContactRequest(c)
	if !allowed {
		return
	}
	id := uint(atoi(c.Param("id")))
	leadID := uint(atoi(c.Param("lead_id")))
	var req dto.UpdateCustomerLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if includeContact && !recordAdminCustomerContactRead(c, ctl.audit, "merchant_customer_lead", leadID, "商家顾客线索联系方式查看") {
		return
	}
	lead, err := ctl.admin.UpdateMerchantLead(id, leadID, req, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"lead": serializeAdminCustomerLead(*lead, includeContact), "updated": true})
}

func (ctl *AdminController) UpdateMerchantStatus(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.UpdateMerchantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := ctl.admin.UpdateMerchantStatus(id, req.Status, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_status_update", TargetType: "merchant", MerchantID: &id, Detail: gin.H{"status": req.Status}},
	}); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
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
	row, err := ctl.admin.CreateMerchantFollowUp(id, c.GetUint("user_id"), operatorRole, req, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_follow_up_create", TargetType: "merchant_follow_up", MerchantID: &id},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"follow_up": row})
}

func (ctl *AdminController) UpdateMerchantFollowUpStatus(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.UpdateMerchantFollowUpStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.UpdateMerchantFollowUpStatus(id, req.Status, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_follow_up_status_update", TargetType: "merchant_follow_up"},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"follow_up": row, "updated": true})
}

func (ctl *AdminController) OpenMerchantSubscription(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.OpenMerchantSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	merchant, err := ctl.admin.OpenMerchantSubscription(id, req, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_subscription_open", TargetType: "merchant", MerchantID: &id},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"merchant": merchant})
}

func (ctl *AdminController) StopMerchantSubscription(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	merchant, err := ctl.admin.StopMerchantSubscription(id, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "merchant_subscription_stop", TargetType: "merchant", MerchantID: &id},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"merchant": merchant})
}

func (ctl *AdminController) PauseMerchantSubscription(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.PauseMerchantSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	merchant, err := ctl.admin.PauseMerchantSubscription(id, req.Reason, merchantSubscriptionActor(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"merchant": merchant})
}

func (ctl *AdminController) ResumeMerchantSubscription(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	merchant, err := ctl.admin.ResumeMerchantSubscription(id, merchantSubscriptionActor(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"merchant": merchant})
}

func merchantSubscriptionActor(c *gin.Context) service.MerchantSubscriptionActor {
	role, _ := c.Get("role")
	roleText, _ := role.(string)
	return service.MerchantSubscriptionActor{
		ID:        c.GetUint("user_id"),
		Type:      roleText,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}
}

func (ctl *AdminController) UpdateUserStatus(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := ctl.admin.UpdateUserStatus(id, req.Status, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "user_status_update", TargetType: "user", Detail: gin.H{"status": req.Status}},
	}); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"updated": true})
}

func (ctl *AdminController) ManualOpenMember(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.ManualOpenMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	user, err := ctl.admin.AdjustUserMember(id, req, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
		Input: service.AuditRecordInput{Action: "user_member_adjust", TargetType: "user", Detail: gin.H{
			"package_id": req.PackageID, "member_level": req.MemberLevel, "duration_days": req.DurationDays,
			"expired_at": req.ExpiredAt, "quota_delta": req.QuotaDelta, "auto_renew": req.AutoRenew,
		}},
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if user == nil {
		utils.Success(c, gin.H{"updated": true})
		return
	}
	utils.Success(c, gin.H{"updated": true, "user": serializeAdminUser(*user, false)})
}

func (ctl *AdminController) UpdateUserPhone(c *gin.Context) {
	id := uint(atoi(c.Param("id")))
	var req dto.UpdateUserPhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := ctl.admin.UpdateUserPhone(id, req.Phone, service.AuditEvent{
		Actor: service.AuditActorFromContext(c),
	}); err != nil {
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
	cards, err := ctl.admin.GenerateCards(req, service.AuditActorFromContext(c))
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

func (ctl *AdminController) Configs(c *gin.Context) {
	list, err := ctl.admin.ListSystemConfigs()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, list)
}

// GetRefundReviewThreshold returns the M2 refund-review threshold in
// cents (default 100000 = 1000 yuan) and its value in yuan for display.
// Super-admin only.
func (ctl *AdminController) GetRefundReviewThreshold(c *gin.Context) {
	cents, err := ctl.admin.GetRefundReviewThreshold()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to load refund review threshold")
		return
	}
	utils.Success(c, gin.H{
		"threshold_cents": cents,
		"threshold_yuan":  fmt.Sprintf("%d.%02d", cents/100, cents%100),
		"default_cents":   service.RefundReviewThresholdDefaultCents,
		"max_cents":       service.RefundReviewThresholdMaxCents,
	})
}

// UpdateRefundReviewThreshold stores the M2 refund-review threshold in
// cents (0..100000000; 0 means every refund needs review). Super-admin
// only; the change is audited.
func (ctl *AdminController) UpdateRefundReviewThreshold(c *gin.Context) {
	var req dto.RefundReviewThresholdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := ctl.admin.SetRefundReviewThreshold(req.ThresholdCents, service.AuditActorFromContext(c)); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"threshold_cents": req.ThresholdCents})
}

func (ctl *AdminController) MaintenanceNotice(c *gin.Context) {
	notice, err := ctl.admin.GetMaintenanceNotice()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, notice)
}

func (ctl *AdminController) SaveMaintenanceNotice(c *gin.Context) {
	var req dto.MerchantMaintenanceNotice
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	notice, err := ctl.admin.SaveMaintenanceNotice(req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"saved": true, "notice": notice})
}

func (ctl *AdminController) SecurityCheck(c *gin.Context) {
	utils.Success(c, ctl.admin.SecurityCheck())
}

func (ctl *AdminController) AIConfigStatus(c *gin.Context) {
	utils.Success(c, ctl.admin.AIConfigStatus())
}

func (ctl *AdminController) SaveAIConfigPolicy(c *gin.Context) {
	var req dto.SaveAIConfigPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	policy, err := ctl.admin.SaveAIConfigPolicy(req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"saved": true, "policy": policy})
}

func (ctl *AdminController) AIUsageOverview(c *gin.Context) {
	data, err := ctl.admin.AIUsageOverview()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, data)
}

func (ctl *AdminController) PlatformCostOverview(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	var data interface{}
	var err error
	if startDate == "" && endDate == "" {
		data, err = ctl.admin.PlatformCostOverview()
	} else {
		if startDate == "" || endDate == "" {
			utils.Error(c, http.StatusBadRequest, "start_date 和 end_date 必须同时提供")
			return
		}
		start, startErr := time.ParseInLocation("2006-01-02", startDate, time.Local)
		end, endErr := time.ParseInLocation("2006-01-02", endDate, time.Local)
		if startErr != nil || endErr != nil || start.Format("2006-01-02") != startDate || end.Format("2006-01-02") != endDate {
			utils.Error(c, http.StatusBadRequest, "日期必须使用 YYYY-MM-DD 格式")
			return
		}
		data, err = ctl.admin.PlatformCostOverviewForPeriod(start, end)
	}
	if err != nil {
		if errors.Is(err, service.ErrInvalidPlatformExpense) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "平台核算数据暂不可用")
		}
		return
	}
	utils.Success(c, data)
}

func (ctl *AdminController) CreatePlatformExpense(c *gin.Context) {
	var request dto.CreatePlatformExpenseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.Error(c, http.StatusBadRequest, "费用字段无效，请检查分类、金额、币种、日期、备注和幂等键")
		return
	}
	entry, created, err := ctl.admin.CreatePlatformExpense(request, service.AuditActorFromContext(c))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPlatformExpense):
			utils.Error(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrPlatformExpenseIdempotencyConflict):
			utils.Error(c, http.StatusConflict, "该提交编号已用于不同费用记录，请刷新后重试")
		default:
			utils.Error(c, http.StatusInternalServerError, "实际费用未能安全保存，请稍后重试")
		}
		return
	}
	utils.Success(c, gin.H{"expense": entry, "created": created})
}

func (ctl *AdminController) CreatePlatformExpenseReversal(c *gin.Context) {
	expenseID, parseErr := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if parseErr != nil || expenseID == 0 {
		utils.Error(c, http.StatusBadRequest, "原费用编号无效")
		return
	}
	var request dto.CreatePlatformExpenseReversalRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.Error(c, http.StatusBadRequest, "冲正字段无效，请检查冲正日期、金额、原因和幂等键")
		return
	}
	entry, created, err := ctl.admin.CreatePlatformExpenseReversal(uint(expenseID), request, service.AuditActorFromContext(c))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPlatformExpense):
			utils.Error(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrPlatformExpenseNotFound):
			utils.Error(c, http.StatusNotFound, "原费用记录不存在")
		case errors.Is(err, service.ErrPlatformExpenseReversalExceedsOriginal):
			utils.Error(c, http.StatusConflict, "冲正金额超过原费用尚未冲正的余额")
		case errors.Is(err, service.ErrPlatformExpenseReversalKeyConflict):
			utils.Error(c, http.StatusConflict, "该提交编号已用于不同冲正记录，请刷新后重试")
		default:
			utils.Error(c, http.StatusInternalServerError, "冲正记录未能安全保存，请稍后重试")
		}
		return
	}
	utils.Success(c, gin.H{"reversal": entry, "created": created})
}

func atoi(v string) int {
	value, _ := strconv.Atoi(v)
	return value
}
