package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

// 公告、维护计划与 P3 数据底座（P2）超管端接口（仅 /api/v1 超管受保护路由）。
// 公告只有超管能发布；商家端只读已发布列表（见 merchant_announcements_controller.go）。

func parseAnnouncementIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		utils.Error(c, http.StatusBadRequest, "id 无效")
		return 0, false
	}
	return uint(id), true
}

func parseMerchantIDQuery(c *gin.Context) (uint, bool) {
	raw := c.Query("merchant_id")
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || id == 0 {
		utils.Error(c, http.StatusBadRequest, "merchant_id 必填")
		return 0, false
	}
	return uint(id), true
}

// --- 公告 ---

func (ctl *AdminController) ListAnnouncements(c *gin.Context) {
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	rows, err := ctl.admin.ListAnnouncements(c.Query("status"), pageSize, (page-1)*pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "公告列表暂不可用")
		return
	}
	utils.Success(c, gin.H{"rows": rows, "page": page, "page_size": pageSize})
}

func (ctl *AdminController) CreateAnnouncement(c *gin.Context) {
	var req dto.CreateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.CreateAnnouncement(req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) GetAnnouncement(c *gin.Context) {
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	row, err := ctl.admin.GetAnnouncement(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "公告不存在")
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) UpdateAnnouncement(c *gin.Context) {
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	var req dto.UpdateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.UpdateAnnouncement(id, req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) PublishAnnouncement(c *gin.Context) {
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	row, err := ctl.admin.PublishAnnouncement(id, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) ArchiveAnnouncement(c *gin.Context) {
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	row, err := ctl.admin.ArchiveAnnouncement(id, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

// --- 维护计划 ---

func (ctl *AdminController) ListMaintenancePlans(c *gin.Context) {
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	rows, err := ctl.admin.ListMaintenancePlans(c.Query("status"), pageSize, (page-1)*pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "维护计划列表暂不可用")
		return
	}
	utils.Success(c, gin.H{"rows": rows, "page": page, "page_size": pageSize})
}

func (ctl *AdminController) CreateMaintenancePlan(c *gin.Context) {
	var req dto.CreateMaintenancePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.CreateMaintenancePlan(req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) GetMaintenancePlan(c *gin.Context) {
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	row, err := ctl.admin.GetMaintenancePlan(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "维护计划不存在")
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) SetMaintenancePlanStatus(c *gin.Context) {
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	var req dto.SetMaintenancePlanStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.SetMaintenancePlanStatus(id, req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

// AdminMaintenanceStatus returns the currently active maintenance plan, if
// any, for the super-admin console.
func (ctl *AdminController) AdminMaintenanceStatus(c *gin.Context) {
	plan, err := ctl.admin.ActiveMaintenancePlan()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "维护状态暂不可用")
		return
	}
	utils.Success(c, gin.H{"maintenance": plan != nil, "plan": plan})
}

// PublicMaintenanceStatus is the unauthenticated maintenance probe for the
// customer order pages: it only reports whether ordering is blocked and when
// it is expected to recover, so customers see a clear message instead of a
// bare order failure.
func (ctl *AdminController) PublicMaintenanceStatus(c *gin.Context) {
	blocked, plan, err := service.MaintenanceBlocksModule(ctl.admin.DB(), service.MaintenanceModuleOrdering)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "维护状态暂不可用")
		return
	}
	if !blocked || plan == nil {
		utils.Success(c, gin.H{"maintenance": false, "plan": nil})
		return
	}
	utils.Success(c, gin.H{"maintenance": true, "plan": gin.H{
		"title":            plan.Title,
		"window_end":       plan.WindowEnd,
		"affected_modules": plan.AffectedModules,
	}})
}

// --- P3 数据底座（超管端，merchant_id 由查询参数指定）---

func (ctl *AdminController) CreateCampaignTarget(c *gin.Context) {
	merchantID, ok := parseMerchantIDQuery(c)
	if !ok {
		return
	}
	var req dto.CreateCampaignTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.CreateCampaignTarget(merchantID, req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) ListCampaignTargets(c *gin.Context) {
	merchantID, ok := parseMerchantIDQuery(c)
	if !ok {
		return
	}
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	rows, err := ctl.admin.ListCampaignTargets(merchantID, c.Query("campaign_type"), pageSize, (page-1)*pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "活动目标暂不可用")
		return
	}
	utils.Success(c, gin.H{"rows": rows, "page": page, "page_size": pageSize})
}

func (ctl *AdminController) UpdateCampaignTarget(c *gin.Context) {
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	var req dto.CreateCampaignTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.UpdateCampaignTarget(id, req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) CreateCampaignPostmortem(c *gin.Context) {
	merchantID, ok := parseMerchantIDQuery(c)
	if !ok {
		return
	}
	var req dto.CreateCampaignPostmortemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.CreateCampaignPostmortem(merchantID, req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) ListCampaignPostmortems(c *gin.Context) {
	merchantID, ok := parseMerchantIDQuery(c)
	if !ok {
		return
	}
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	rows, err := ctl.admin.ListCampaignPostmortems(merchantID, c.Query("campaign_type"), pageSize, (page-1)*pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "活动复盘暂不可用")
		return
	}
	utils.Success(c, gin.H{"rows": rows, "page": page, "page_size": pageSize})
}

func (ctl *AdminController) RecordAISuggestionExecution(c *gin.Context) {
	merchantID, ok := parseMerchantIDQuery(c)
	if !ok {
		return
	}
	var req dto.RecordAISuggestionExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.RecordAISuggestionExecution(merchantID, req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) ListAISuggestionExecutions(c *gin.Context) {
	merchantID, ok := parseMerchantIDQuery(c)
	if !ok {
		return
	}
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	rows, err := ctl.admin.ListAISuggestionExecutions(merchantID, c.Query("status"), pageSize, (page-1)*pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "建议执行记录暂不可用")
		return
	}
	utils.Success(c, gin.H{"rows": rows, "page": page, "page_size": pageSize})
}

func (ctl *AdminController) UpdateAISuggestionExecution(c *gin.Context) {
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	var req dto.UpdateAISuggestionExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.UpdateAISuggestionExecution(id, req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) CreateGroupBuyManualRedemption(c *gin.Context) {
	merchantID, ok := parseMerchantIDQuery(c)
	if !ok {
		return
	}
	var req dto.CreateGroupBuyManualRedemptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.CreateGroupBuyManualRedemption(merchantID, req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) ListGroupBuyManualRedemptions(c *gin.Context) {
	merchantID, ok := parseMerchantIDQuery(c)
	if !ok {
		return
	}
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	var linkID uint
	if raw := c.Query("link_id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "link_id 无效")
			return
		}
		linkID = uint(parsed)
	}
	list, err := ctl.admin.ListGroupBuyManualRedemptions(merchantID, linkID, pageSize, (page-1)*pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "手工核销记录暂不可用")
		return
	}
	utils.Success(c, gin.H{"rows": list.Rows, "total_count": list.TotalCount, "total_amount_cents": list.TotalAmountCents, "source_note": list.SourceNote, "page": page, "page_size": pageSize})
}
