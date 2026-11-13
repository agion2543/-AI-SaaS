package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type GroupBuyController struct {
	groupBuy *service.MerchantGroupBuyService
}

func NewGroupBuyController(groupBuy *service.MerchantGroupBuyService) *GroupBuyController {
	return &GroupBuyController{groupBuy: groupBuy}
}

func (ctl *GroupBuyController) List(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	storeID, err := strconv.ParseUint(c.Query("store_id"), 10, 64)
	if err != nil || storeID == 0 {
		utils.Error(c, http.StatusBadRequest, "请选择有效门店")
		return
	}
	links, err := ctl.groupBuy.List(merchantID, uint(storeID))
	if err != nil {
		if errors.Is(err, service.ErrGroupBuyStoreAccess) {
			utils.Error(c, http.StatusForbidden, err.Error())
			return
		}
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, gin.H{"links": links})
}

func (ctl *GroupBuyController) Save(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req struct {
		StoreID        uint   `json:"store_id" binding:"required"`
		Channel        string `json:"channel" binding:"required"`
		DestinationURL string `json:"destination_url" binding:"required"`
		Enabled        bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "请填写门店、平台和团购链接")
		return
	}
	link, err := ctl.groupBuy.Save(merchantID, req.StoreID, req.Channel, req.DestinationURL, req.Enabled)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrGroupBuyStoreAccess) {
			status = http.StatusForbidden
		}
		utils.Error(c, status, err.Error())
		return
	}
	utils.Success(c, gin.H{"link": link, "saved": true})
}

func (ctl *GroupBuyController) Redirect(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Robots-Tag", "noindex, nofollow")
	destination, err := ctl.groupBuy.ResolveAndTrack(c.Param("code"))
	if err != nil {
		if errors.Is(err, service.ErrGroupBuyLinkNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Error(c, http.StatusNotFound, "团购入口不存在或已停用")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "团购入口暂时不可用")
		return
	}
	c.Redirect(http.StatusFound, destination)
}
