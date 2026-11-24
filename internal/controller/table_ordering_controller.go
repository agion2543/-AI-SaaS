package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

type TableOrderingController struct {
	service     *service.TableOrderingService
	frontendURL string
}

func NewTableOrderingController(tableService *service.TableOrderingService, frontendURLs ...string) *TableOrderingController {
	frontendURL := ""
	if len(frontendURLs) > 0 {
		frontendURL = strings.TrimRight(strings.TrimSpace(frontendURLs[0]), "/")
	}
	return &TableOrderingController{service: tableService, frontendURL: frontendURL}
}

func (ctl *TableOrderingController) GetMerchantTables(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	storeID := uint(atoi(c.Param("storeId")))
	view, err := ctl.service.StoreTableConfig(merchantID, storeID, func(tableNo int, code string) string {
		return ctl.tableURL(storeID, tableNo, code)
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, view)
}

func (ctl *TableOrderingController) ConfigureMerchantTables(c *gin.Context) {
	merchantID, ok := currentMerchantID(c)
	if !ok {
		utils.Error(c, http.StatusForbidden, "merchant context missing")
		return
	}
	var req dto.ConfigureStoreTablesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "桌数需设置为0到300的整数")
		return
	}
	storeID := uint(atoi(c.Param("storeId")))
	view, err := ctl.service.ConfigureStoreTables(merchantID, storeID, req.TableCount, req.Tables, func(tableNo int, code string) string {
		return ctl.tableURL(storeID, tableNo, code)
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, view)
}

func (ctl *TableOrderingController) Join(c *gin.Context) {
	var req dto.JoinTableSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "无效桌码，请重新扫码")
		return
	}
	snapshot, err := ctl.service.JoinTableSession(req)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"session": snapshot})
}

func (ctl *TableOrderingController) Snapshot(c *gin.Context) {
	snapshot, err := ctl.service.SessionSnapshot(uint(atoi(c.Param("sessionId"))), c.GetHeader("X-Table-Code"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"session": snapshot})
}

func (ctl *TableOrderingController) UpdateItem(c *gin.Context) {
	var req dto.UpdateTableCartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "购物车操作无效")
		return
	}
	snapshot, err := ctl.service.UpdateCartItem(uint(atoi(c.Param("sessionId"))), c.GetHeader("X-Table-Code"), req)
	if err != nil {
		utils.Error(c, http.StatusConflict, err.Error())
		return
	}
	utils.Success(c, gin.H{"session": snapshot})
}

func (ctl *TableOrderingController) Clear(c *gin.Context) {
	snapshot, err := ctl.service.ClearCart(uint(atoi(c.Param("sessionId"))), c.GetHeader("X-Table-Code"))
	if err != nil {
		utils.Error(c, http.StatusConflict, err.Error())
		return
	}
	utils.Success(c, gin.H{"session": snapshot})
}

func (ctl *TableOrderingController) Checkout(c *gin.Context) {
	var req dto.TableCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "结算信息无效")
		return
	}
	order, created, err := ctl.service.Checkout(uint(atoi(c.Param("sessionId"))), c.GetHeader("X-Table-Code"), req)
	if err != nil {
		utils.Error(c, http.StatusConflict, err.Error())
		return
	}
	order.CustomerAccessToken = req.AccessToken
	publicOrder := serializePublicCustomerOrder(order)
	paymentMethod := "none"
	paymentRequired := order.TotalAmount > 0 && order.Status == "pending"
	paymentConfig := gin.H{}
	var paymentConfigError string
	if order.TotalAmount > 0 {
		if order.PaymentChannel == "merchant_qr" {
			paymentMethod = "merchant_qr"
			config, configErr := ctl.service.PublicPaymentConfigForOrder(order)
			if configErr != nil {
				paymentConfigError = "商家收款信息暂时无法读取，请联系门店核实"
			} else {
				paymentConfig = serializePublicPaymentConfig(config)
			}
		} else {
			paymentMethod = "unavailable"
			paymentConfigError = "此桌位订单未绑定商家收款码，请联系门店核实；请勿使用平台支付入口"
		}
	}
	response := gin.H{
		"order":                          publicOrder,
		"access_token":                   req.AccessToken,
		"payment_required":               paymentRequired,
		"payment_method":                 paymentMethod,
		"payment_config":                 paymentConfig,
		"manual_reconciliation_required": paymentMethod == "merchant_qr" && order.TotalAmount > 0,
		"created":                        created,
	}
	if paymentConfigError != "" {
		response["payment_config_error"] = paymentConfigError
	}
	utils.Success(c, response)
}

func (ctl *TableOrderingController) tableURL(storeID uint, tableNo int, code string) string {
	base := ctl.frontendURL
	if base == "" {
		base = "/"
	}
	values := url.Values{}
	values.Set("table_no", fmt.Sprint(tableNo))
	values.Set("table_code", code)
	return strings.TrimRight(base, "/") + fmt.Sprintf("/customer/store/%d", storeID) + "?" + values.Encode()
}
