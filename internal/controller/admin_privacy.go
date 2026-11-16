package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/adminaccess"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

func adminRequestsFullCustomerContact(c *gin.Context) bool {
	return c.Query("include_contact") == "true"
}

func authorizeAdminCustomerContactRequest(c *gin.Context) (bool, bool) {
	if !adminRequestsFullCustomerContact(c) {
		return false, true
	}
	if c.GetString("role") == "super_admin" {
		return true, true
	}
	permissions, _ := c.Get("admin_permissions")
	stored, _ := permissions.(string)
	if adminaccess.HasPermission(stored, adminaccess.PermissionViewCustomerContact) {
		return true, true
	}
	utils.Error(c, http.StatusForbidden, "permission denied")
	c.Abort()
	return false, false
}

func recordAdminCustomerContactRead(c *gin.Context, audit *service.AuditService, targetType string, targetID uint, purpose string) bool {
	if audit == nil || audit.RecordSensitiveRead(c, "admin_customer_contact_view", targetType, targetID, 0, purpose) != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to record sensitive access audit")
		c.Abort()
		return false
	}
	return true
}

func recordAdminCustomerOrderExport(c *gin.Context, audit *service.AuditService, targetType string) bool {
	if audit == nil {
		utils.Error(c, http.StatusInternalServerError, "failed to record sensitive access audit")
		c.Abort()
		return false
	}
	purpose := "导出平台客户/订单数据（脱敏联系方式）"
	if adminRequestsFullCustomerContact(c) {
		purpose = "导出平台客户/订单数据（完整联系方式）"
	}
	if err := audit.RecordSensitiveRead(c, "admin_customer_order_export", targetType, 0, 0,
		purpose); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to record sensitive access audit")
		c.Abort()
		return false
	}
	return true
}

func serializeAdminCustomerLead(lead model.CustomerLead, includeContact bool) gin.H {
	name, phone := lead.CustomerName, lead.CustomerPhone
	row := gin.H{
		"id": lead.ID, "merchant_id": lead.MerchantID, "store_id": lead.StoreID,
		"customer_name": name, "customer_phone": phone, "source": lead.Source,
		"status": lead.Status, "created_at": lead.CreatedAt, "updated_at": lead.UpdatedAt,
	}
	if !includeContact {
		row["customer_name"] = maskCustomerName(name)
		row["customer_phone"] = utils.MaskPhone(phone)
	} else {
		row["message"] = lead.Message
		row["follow_up_note"] = lead.FollowUpNote
	}
	if lead.Store.ID > 0 {
		row["store"] = gin.H{"id": lead.Store.ID, "name": lead.Store.Name}
	}
	return row
}

func serializeAdminUser(user model.User, includeContact bool) gin.H {
	phone, email := user.Phone, user.Email
	if !includeContact {
		phone = utils.MaskPhone(phone)
		email = utils.MaskEmail(email)
	}
	return gin.H{
		"id": user.ID, "uuid": user.UUID, "phone": phone, "email": email,
		"display_name": user.DisplayName, "role": user.Role, "status": user.Status,
		"member_level": user.MemberLevel, "current_package_id": user.CurrentPackageID,
		"expired_at": user.ExpiredAt, "remaining_quota": user.RemainingQuota,
		"auto_renew": user.AutoRenew, "created_at": user.CreatedAt, "updated_at": user.UpdatedAt,
	}
}

func maskCustomerName(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return ""
	}
	if len(runes) == 1 {
		return "*"
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-1)
}
