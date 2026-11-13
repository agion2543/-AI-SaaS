package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type PlatformGroupBuyController struct {
	service *service.PlatformGroupBuyService
}

func NewPlatformGroupBuyController(groupBuy *service.PlatformGroupBuyService) *PlatformGroupBuyController {
	return &PlatformGroupBuyController{service: groupBuy}
}

func (ctl *PlatformGroupBuyController) List(c *gin.Context) {
	page, pageSize := utils.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	rows, total, summary, err := ctl.service.List(service.PlatformGroupBuyFilter{
		Keyword:      c.Query("keyword"),
		Channel:      c.Query("channel"),
		ReviewStatus: c.Query("review_status"),
		ReviewMethod: c.Query("review_method"),
		EntryState:   c.Query("entry_state"),
	}, page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "团购入口列表暂时不可用")
		return
	}
	utils.Success(c, gin.H{"list": rows, "total": total, "summary": summary})
}

func (ctl *PlatformGroupBuyController) Detail(c *gin.Context) {
	id, ok := parsePositiveID(c.Param("id"))
	if !ok {
		utils.Error(c, http.StatusBadRequest, "团购入口编号无效")
		return
	}
	detail, err := ctl.service.GetDetail(id)
	if err != nil {
		if errors.Is(err, service.ErrAdminGroupBuyLinkNotFound) {
			utils.Error(c, http.StatusNotFound, err.Error())
			return
		}
		utils.Error(c, http.StatusInternalServerError, "团购入口详情暂时不可用")
		return
	}
	utils.Success(c, gin.H{"link": detail.Link, "history": detail.History})
}

func (ctl *PlatformGroupBuyController) Review(c *gin.Context) {
	id, ok := parsePositiveID(c.Param("id"))
	if !ok {
		utils.Error(c, http.StatusBadRequest, "团购入口编号无效")
		return
	}
	var req struct {
		Action string `json:"action" binding:"required,oneof=approve reject suspend resume"`
		Reason string `json:"reason" binding:"max=255"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "请提供有效的管理操作和原因（最多 255 个字符）")
		return
	}
	actorType, _ := c.Get("role")
	role, _ := actorType.(string)
	link, err := ctl.service.ApplyAdminAction(id, c.GetUint("user_id"), role, req.Action, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrAdminGroupBuyLinkNotFound) {
			utils.Error(c, http.StatusNotFound, err.Error())
			return
		}
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"link": link, "saved": true})
}

func parsePositiveID(raw string) (uint, bool) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), uint64(uint(id)) == id
}
