package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

// AI 成本控制超管端接口（P1 MVP，仅 /api/v1 超管受保护路由；legacy /api 不动）。
// 估算成本与供应商实际账单在返回字段上严格分开，绝不把估算说成账单。

func (ctl *AdminController) AIUsageStats(c *gin.Context) {
	start, end, ok := parseAICostPeriodQuery(c)
	if !ok {
		return
	}
	stats, err := service.NewAICostGuard(ctl.admin.DB()).AIUsageStats(start, end)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "AI 使用量统计暂不可用")
		return
	}
	utils.Success(c, stats)
}

func (ctl *AdminController) ListAIModelPrices(c *gin.Context) {
	rows, err := ctl.admin.ListModelPrices()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "模型价格表暂不可用")
		return
	}
	utils.Success(c, rows)
}

func (ctl *AdminController) SaveAIModelPrice(c *gin.Context) {
	var req dto.SaveAIModelPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, _, err := ctl.admin.SaveModelPrice(req, service.AuditActorFromContext(c))
	if err != nil {
		if errors.Is(err, service.ErrInvalidAICostControl) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "模型价格保存失败")
		}
		return
	}
	utils.Success(c, row)
}

func (ctl *AdminController) CreateAIVendorBill(c *gin.Context) {
	var req dto.CreateAIVendorBillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	bill, created, err := ctl.admin.CreateAIVendorBill(req, service.AuditActorFromContext(c))
	if err != nil {
		if errors.Is(err, service.ErrInvalidAICostControl) || errors.Is(err, service.ErrAIVendorBillKeyConflict) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "供应商账单录入失败")
		}
		return
	}
	utils.Success(c, gin.H{"created": created, "bill": bill})
}

func (ctl *AdminController) ListAIVendorBills(c *gin.Context) {
	start, end, ok := parseAICostPeriodQuery(c)
	if !ok {
		return
	}
	rows, err := ctl.admin.ListAIVendorBills(c.Query("vendor"), start, end.AddDate(0, 0, -1))
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "供应商账单暂不可用")
		return
	}
	utils.Success(c, rows)
}

func (ctl *AdminController) ReconcileAIBills(c *gin.Context) {
	start, end, ok := parseAICostPeriodQuery(c)
	if !ok {
		return
	}
	result, err := ctl.admin.ReconcileAIBills(start, end.AddDate(0, 0, -1))
	if err != nil {
		if errors.Is(err, service.ErrInvalidAICostControl) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "账单核对暂不可用")
		}
		return
	}
	utils.Success(c, result)
}

func (ctl *AdminController) ListMerchantAIBudgets(c *gin.Context) {
	rows, err := ctl.admin.ListMerchantBudgets()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "商家预算暂不可用")
		return
	}
	utils.Success(c, rows)
}

func (ctl *AdminController) MerchantAIBudgetStatus(c *gin.Context) {
	merchantID, ok := parseMerchantIDParam(c)
	if !ok {
		return
	}
	status, budget, err := ctl.admin.MerchantBudgetStatus(merchantID)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "商家预算状态暂不可用")
		return
	}
	utils.Success(c, gin.H{"status": status, "budget": budget})
}

func (ctl *AdminController) SaveMerchantAIBudget(c *gin.Context) {
	merchantID, ok := parseMerchantIDParam(c)
	if !ok {
		return
	}
	var req dto.SaveMerchantAIBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	budget, err := ctl.admin.SaveMerchantBudget(merchantID, req, service.AuditActorFromContext(c))
	if err != nil {
		if errors.Is(err, service.ErrInvalidAICostControl) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "商家预算保存失败")
		}
		return
	}
	utils.Success(c, budget)
}

func (ctl *AdminController) ListAIBudgetAlerts(c *gin.Context) {
	merchantID := uint(0)
	if raw := c.Query("merchant_id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "merchant_id 无效")
			return
		}
		merchantID = uint(parsed)
	}
	threshold := 0
	if raw := c.Query("threshold"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || (parsed != 70 && parsed != 90 && parsed != 100) {
			utils.Error(c, http.StatusBadRequest, "threshold 只能为 70、90 或 100")
			return
		}
		threshold = parsed
	}
	var start, end time.Time
	if raw := c.Query("start_date"); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "start_date 必须使用 YYYY-MM-DD 格式")
			return
		}
		start = parsed
	}
	if raw := c.Query("end_date"); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "end_date 必须使用 YYYY-MM-DD 格式")
			return
		}
		end = parsed
	}
	rows, err := ctl.admin.ListBudgetAlerts(merchantID, c.Query("period_type"), threshold, start, end)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "预算预警暂不可用")
		return
	}
	utils.Success(c, rows)
}

func (ctl *AdminController) GetAIEmergencySwitch(c *gin.Context) {
	disabled, row, err := service.NewAICostGuard(ctl.admin.DB()).GetAIEmergencySwitch()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "紧急关闭开关暂不可用")
		return
	}
	utils.Success(c, gin.H{"disabled": disabled, "switch": row})
}

func (ctl *AdminController) SetAIEmergencySwitch(c *gin.Context) {
	var req dto.SetAIEmergencySwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := ctl.admin.SetAIEmergencySwitch(req.Disabled, req.Reason, service.AuditActorFromContext(c))
	if err != nil {
		if errors.Is(err, service.ErrInvalidAICostControl) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "紧急关闭开关设置失败")
		}
		return
	}
	utils.Success(c, row)
}

// parseAICostPeriodQuery 解析 start_date/end_date 查询参数（默认最近 30 天，
// 含今天）。返回左闭右开的 [start, end)。
func parseAICostPeriodQuery(c *gin.Context) (time.Time, time.Time, bool) {
	now := time.Now().In(time.Local)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	start := today.AddDate(0, 0, -29)
	end := today
	if raw := c.Query("start_date"); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil || parsed.Format("2006-01-02") != raw {
			utils.Error(c, http.StatusBadRequest, "start_date 必须使用 YYYY-MM-DD 格式")
			return time.Time{}, time.Time{}, false
		}
		start = parsed
	}
	if raw := c.Query("end_date"); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil || parsed.Format("2006-01-02") != raw {
			utils.Error(c, http.StatusBadRequest, "end_date 必须使用 YYYY-MM-DD 格式")
			return time.Time{}, time.Time{}, false
		}
		end = parsed
	}
	if end.Before(start) {
		utils.Error(c, http.StatusBadRequest, "end_date 不能早于 start_date")
		return time.Time{}, time.Time{}, false
	}
	if end.After(today) {
		utils.Error(c, http.StatusBadRequest, "end_date 不能晚于今天")
		return time.Time{}, time.Time{}, false
	}
	if end.AddDate(0, 0, 1).After(start.AddDate(0, 0, 366)) {
		utils.Error(c, http.StatusBadRequest, "查询周期不能超过 366 天")
		return time.Time{}, time.Time{}, false
	}
	return start, end.AddDate(0, 0, 1), true
}

func parseMerchantIDParam(c *gin.Context) (uint, bool) {
	parsed, err := strconv.ParseUint(c.Param("merchantId"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "merchantId 无效")
		return 0, false
	}
	return uint(parsed), true
}
