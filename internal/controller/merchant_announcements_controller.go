package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

// 公告与 P3 数据底座（P2）商家端接口（仅 /api/v1 商家受保护路由）。
// merchant_id 一律取自登录上下文，绝不信任客户端传入的商家 ID；更新类
// 操作先验记录归属再执行。

// ListVisibleAnnouncements returns the published announcements visible to
// this merchant.
func (ctl *MerchantController) ListVisibleAnnouncements(c *gin.Context) {
	rows, err := ctl.admin.ListVisibleAnnouncements(20)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "公告暂不可用")
		return
	}
	utils.Success(c, gin.H{"rows": rows})
}

// MerchantMaintenanceStatus returns the currently active maintenance plan,
// if any, so the merchant console can show it.
func (ctl *MerchantController) MerchantMaintenanceStatus(c *gin.Context) {
	plan, err := ctl.admin.ActiveMaintenancePlan()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "维护状态暂不可用")
		return
	}
	utils.Success(c, gin.H{"maintenance": plan != nil, "plan": plan})
}

func (ctl *MerchantController) CreateCampaignTarget(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
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

func (ctl *MerchantController) ListCampaignTargets(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
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

func (ctl *MerchantController) UpdateCampaignTarget(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	existing, err := ctl.admin.GetCampaignTarget(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "活动目标不存在")
		return
	}
	if existing.MerchantID != merchantID {
		utils.Error(c, http.StatusForbidden, "无权修改他人的活动目标")
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

func (ctl *MerchantController) ListCampaignPostmortems(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
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

func (ctl *MerchantController) ListAISuggestionExecutions(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
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

func (ctl *MerchantController) UpdateAISuggestionExecution(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	id, ok := parseAnnouncementIDParam(c)
	if !ok {
		return
	}
	existing, err := ctl.admin.GetAISuggestionExecution(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "建议记录不存在")
		return
	}
	if existing.MerchantID != merchantID {
		utils.Error(c, http.StatusForbidden, "无权修改他人的建议记录")
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

func (ctl *MerchantController) CreateGroupBuyManualRedemption(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
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

func (ctl *MerchantController) ListGroupBuyManualRedemptions(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
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
