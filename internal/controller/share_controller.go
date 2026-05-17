package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type ShareController struct {
	shares *service.ShareService
}

func NewShareController(shares *service.ShareService) *ShareController {
	return &ShareController{shares: shares}
}

func (ctl *ShareController) CreateOrderShare(c *gin.Context) {
	campaign, err := ctl.shares.EnsureCampaignForOrder(c.Param("orderNo"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"campaign": campaign})
}

func (ctl *ShareController) TrackShareScan(c *gin.Context) {
	campaign, err := ctl.shares.TrackScan(c.Param("code"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"campaign": campaign})
}

func (ctl *ShareController) MerchantShareStats(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	config, err := ctl.shares.GetActivityConfig(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	params, err := parseShareStatsParams(c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	stats, campaigns, coupons, err := ctl.shares.StatsByMerchant(merchantID, params)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"stats": stats, "list": campaigns, "coupons": coupons, "config": config})
}

func parseShareStatsParams(startDate, endDate string) (service.ShareStatsParams, error) {
	var params service.ShareStatsParams
	if strings.TrimSpace(startDate) != "" {
		start, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(startDate), time.Local)
		if err != nil {
			return params, err
		}
		params.StartAt = &start
	}
	if strings.TrimSpace(endDate) != "" {
		end, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(endDate), time.Local)
		if err != nil {
			return params, err
		}
		end = end.AddDate(0, 0, 1).Add(-time.Nanosecond)
		params.EndAt = &end
	}
	return params, nil
}

func (ctl *ShareController) MerchantShareConfig(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	config, err := ctl.shares.GetActivityConfig(merchantID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"config": config})
}

func (ctl *ShareController) SaveMerchantShareConfig(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req service.ShareActivityConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	config, err := ctl.shares.UpdateActivityConfig(merchantID, req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"config": config})
}

func (ctl *ShareController) MerchantCoupons(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	stats, coupons, total, err := ctl.shares.ListMerchantCoupons(merchantID, service.CouponListParams{
		Phone:    c.Query("phone"),
		Status:   c.Query("status"),
		Keyword:  c.Query("keyword"),
		StoreID:  uint(atoi(c.Query("store_id"))),
		Page:     atoi(c.DefaultQuery("page", "1")),
		PageSize: atoi(c.DefaultQuery("page_size", "20")),
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"stats": stats, "list": coupons, "total": total})
}

func (ctl *ShareController) RedeemMerchantCoupon(c *gin.Context) {
	ctl.updateMerchantCoupon(c, "redeem")
}

func (ctl *ShareController) VoidMerchantCoupon(c *gin.Context) {
	ctl.updateMerchantCoupon(c, "void")
}

func (ctl *ShareController) updateMerchantCoupon(c *gin.Context, action string) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req struct {
		Remark string `json:"remark"`
	}
	_ = c.ShouldBindJSON(&req)
	couponID := uint(atoi(c.Param("id")))
	var (
		coupon interface{}
		err    error
	)
	if action == "redeem" {
		coupon, err = ctl.shares.RedeemMerchantCoupon(merchantID, couponID, strings.TrimSpace(req.Remark))
	} else {
		coupon, err = ctl.shares.VoidMerchantCoupon(merchantID, couponID, strings.TrimSpace(req.Remark))
	}
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"coupon": coupon})
}
