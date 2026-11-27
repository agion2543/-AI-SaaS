package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

// P3 商家 AI 经营看板（仅 /api/v1 商家受保护路由）：每日总结、商品体检、
// 沉睡顾客、内容脚本、建议生成、商家自助生成活动复盘。merchant_id 一律
// 取自登录上下文，绝不信任客户端传入的商家 ID；数字口径与证据来源由
// service 层保证，本层只做参数校验与归属校验。

// OpsDailySummary returns one merchant's daily ops summary (Beijing date,
// default today). Trades are attributed by paid_at under the frozen trade
// definition; Douyin redemptions are merchant-entered and reported apart.
func (ctl *MerchantController) OpsDailySummary(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	summary, err := ctl.merchant.OpsDailySummary(merchantID, c.Query("date"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, summary)
}

// OpsProductHealth returns the product health check for a window (default
// 30 days, max 90) computed from order_items detail rows.
func (ctl *MerchantController) OpsProductHealth(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	health, err := ctl.merchant.OpsProductHealth(merchantID, atoi(c.Query("days")))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, health)
}

// OpsDormantCustomers returns dormant customers grouped in three tiers by
// days since their last valid trade. Phones are masked samples only.
func (ctl *MerchantController) OpsDormantCustomers(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	dormant, err := ctl.merchant.OpsDormantCustomers(merchantID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "沉睡顾客暂不可用")
		return
	}
	utils.Success(c, dormant)
}

// OpsContentScripts returns one content script for a scenario. When no
// model is wired the service returns the template with fallback=true;
// the page must show the “模板兜底” label in that case.
func (ctl *MerchantController) OpsContentScripts(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	script, err := ctl.merchant.OpsContentScripts(c.Request.Context(), merchantID, c.Query("scenario"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, script)
}

// GenerateOpsSuggestions derives suggestions from the merchant's own data
// (slow/unsold products, dormant tiers, missed campaign targets) and opens
// the execution loop for each. Identical pending suggestions are skipped.
func (ctl *MerchantController) GenerateOpsSuggestions(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	result, err := ctl.merchant.GenerateOpsSuggestions(merchantID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "生成建议暂不可用")
		return
	}
	utils.Success(c, result)
}

// CreateCampaignPostmortem lets a merchant generate a postmortem snapshot
// for their own campaign. Actuals are computed once from orders under the
// frozen trade definition and stored; generated_by is forced to manual
// (a merchant-triggered snapshot, not a system run).
func (ctl *MerchantController) CreateCampaignPostmortem(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req dto.CreateCampaignPostmortemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.GeneratedBy = model.SuggestionGeneratedByManual
	row, err := ctl.admin.CreateCampaignPostmortem(merchantID, req, service.AuditActorFromContext(c))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, row)
}
