package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

// Platform-finance handlers (v14): time logs, income records, MRR and the
// breakeven trial calculation. All of these are super-admin only (see
// internal/adminaccess); no finance figure is exposed on any merchant,
// customer or public route.

func (ctl *AdminController) ListPlatformTimeLogs(c *gin.Context) {
	data, err := ctl.admin.ListPlatformTimeLogs(c.Query("month"), c.Query("work_type"))
	if err != nil {
		if errors.Is(err, service.ErrInvalidPlatformTimeLog) || errors.Is(err, service.ErrInvalidPlatformFinancePeriod) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "时间成本记录暂不可用")
		}
		return
	}
	utils.Success(c, data)
}

func (ctl *AdminController) CreatePlatformTimeLog(c *gin.Context) {
	var request dto.CreatePlatformTimeLogRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.Error(c, http.StatusBadRequest, "时间成本字段无效，请检查日期、工作类型、内容、小时数、参考小时价值和幂等键")
		return
	}
	entry, created, err := ctl.admin.CreatePlatformTimeLog(request, service.AuditActorFromContext(c))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPlatformTimeLog):
			utils.Error(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrPlatformTimeLogIdempotencyConflict):
			utils.Error(c, http.StatusConflict, "该提交编号已用于不同时间成本记录，请刷新后重试")
		default:
			utils.Error(c, http.StatusInternalServerError, "时间成本记录未能安全保存，请稍后重试")
		}
		return
	}
	utils.Success(c, gin.H{"time_log": entry, "created": created})
}

func (ctl *AdminController) ListPlatformIncomeRecords(c *gin.Context) {
	data, err := ctl.admin.ListPlatformIncomeRecords(c.Query("status"), c.Query("month"))
	if err != nil {
		if errors.Is(err, service.ErrInvalidPlatformIncomeRecord) || errors.Is(err, service.ErrInvalidPlatformFinancePeriod) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "收入台账暂不可用")
		}
		return
	}
	utils.Success(c, data)
}

func (ctl *AdminController) CreatePlatformIncomeRecord(c *gin.Context) {
	var request dto.CreatePlatformIncomeRecordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.Error(c, http.StatusBadRequest, "收入字段无效，请检查收入类型、金额、币种、日期、状态和幂等键")
		return
	}
	entry, created, err := ctl.admin.CreatePlatformIncomeRecord(request, service.AuditActorFromContext(c))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPlatformIncomeRecord):
			utils.Error(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrPlatformIncomeIdempotencyConflict):
			utils.Error(c, http.StatusConflict, "该提交编号已用于不同收入记录，请刷新后重试")
		default:
			utils.Error(c, http.StatusInternalServerError, "收入记录未能安全保存，请稍后重试")
		}
		return
	}
	utils.Success(c, gin.H{"income_record": entry, "created": created})
}

func (ctl *AdminController) UpdatePlatformIncomeRecord(c *gin.Context) {
	recordID, parseErr := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if parseErr != nil || recordID == 0 {
		utils.Error(c, http.StatusBadRequest, "收入记录编号无效")
		return
	}
	var request dto.UpdatePlatformIncomeRecordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.Error(c, http.StatusBadRequest, "收入更新字段无效，请检查状态、收款日期、退款金额和备注")
		return
	}
	entry, err := ctl.admin.UpdatePlatformIncomeRecord(uint(recordID), request, service.AuditActorFromContext(c))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPlatformIncomeRecord):
			utils.Error(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrPlatformIncomeRecordNotFound):
			utils.Error(c, http.StatusNotFound, "收入记录不存在")
		case errors.Is(err, service.ErrPlatformIncomeTransitionNotAllowed):
			utils.Error(c, http.StatusConflict, err.Error())
		default:
			utils.Error(c, http.StatusInternalServerError, "收入记录未能安全更新，请稍后重试")
		}
		return
	}
	utils.Success(c, gin.H{"income_record": entry})
}

func (ctl *AdminController) GetPlatformMRR(c *gin.Context) {
	data, err := ctl.admin.PlatformMRR(c.Query("month"))
	if err != nil {
		if errors.Is(err, service.ErrInvalidPlatformFinancePeriod) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "MRR 暂不可用")
		}
		return
	}
	utils.Success(c, data)
}

func (ctl *AdminController) GetPlatformBreakeven(c *gin.Context) {
	revenue, revenueErr := parseOptionalPlatformFinanceCents(c.Query("merchant_monthly_revenue_cents"))
	variable, variableErr := parseOptionalPlatformFinanceCents(c.Query("merchant_monthly_variable_cents"))
	if revenueErr != nil || variableErr != nil {
		utils.Error(c, http.StatusBadRequest, "单商家月均收入与单商家月变动成本须为整数分")
		return
	}
	data, err := ctl.admin.PlatformBreakeven(c.Query("month"), revenue, variable)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPlatformFinancePeriod) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "保本测算暂不可用")
		}
		return
	}
	utils.Success(c, data)
}

func parseOptionalPlatformFinanceCents(raw string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (ctl *AdminController) GetPlatformExpenseStats(c *gin.Context) {
	data, err := ctl.admin.PlatformExpenseStats(c.Query("start_month"), c.Query("end_month"))
	if err != nil {
		if errors.Is(err, service.ErrInvalidPlatformFinancePeriod) {
			utils.Error(c, http.StatusBadRequest, err.Error())
		} else {
			utils.Error(c, http.StatusInternalServerError, "费用按类按月统计暂不可用")
		}
		return
	}
	utils.Success(c, data)
}
