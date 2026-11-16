package adminaccess

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

const (
	PermissionReadOnly            = "read_only"
	PermissionOperations          = "operations"
	PermissionFinance             = "finance"
	PermissionSystem              = "system_admin"
	PermissionViewCustomerContact = "customer_contact_view"
	PermissionExportCustomerOrder = "customer_order_export"
)

const (
	accessSession    = "@session"
	accessSuperAdmin = "@super_admin"
)

type PermissionInfo struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

var permissionCatalog = []PermissionInfo{
	{Code: PermissionReadOnly, Name: "只读查询", Description: "查看经营概览、商家、顾客、团购入口及运营资料，不执行修改操作。"},
	{Code: PermissionOperations, Name: "运营管理", Description: "处理商家、顾客、门店、线索及团购审核等日常运营动作。"},
	{Code: PermissionFinance, Name: "财务管理", Description: "查看和处理套餐、订阅、订单、支付、退款、结算及卡密。"},
	{Code: PermissionSystem, Name: "系统管理", Description: "查看审计与安全信息，维护站点配置和 AI 策略；管理员账号与角色仍仅限超管。"},
	{Code: PermissionViewCustomerContact, Name: "查看完整客户联系方式", Description: "按需查看客户与订单中的完整手机号、邮箱等联系方式；查看行为会写入审计。"},
	{Code: PermissionExportCustomerOrder, Name: "导出客户/订单数据", Description: "下载客户档案或订单数据；导出行为会写入审计，完整联系方式仍需单独授权。"},
}

func PermissionCatalog() []PermissionInfo {
	result := make([]PermissionInfo, len(permissionCatalog))
	copy(result, permissionCatalog)
	return result
}

func NormalizePermissions(values []string) (string, error) {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		code := strings.TrimSpace(value)
		if !isKnownPermission(code) {
			return "", errors.New("包含无效的管理员权限")
		}
		seen[code] = struct{}{}
	}
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return strings.Join(codes, ","), nil
}

// ParsePermissions accepts the historical comma-separated format and JSON arrays,
// but returns only explicitly recognized permission codes.
func ParsePermissions(raw string) []string {
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		values = strings.Split(raw, ",")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		code := strings.TrimSpace(value)
		if isKnownPermission(code) {
			seen[code] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for code := range seen {
		result = append(result, code)
	}
	sort.Strings(result)
	return result
}

func HasPermission(raw, code string) bool {
	for _, allowed := range ParsePermissions(raw) {
		if allowed == code {
			return true
		}
	}
	return false
}

type RouteAccess struct {
	AnyPermission  []string
	AllPermissions []string
	SessionOnly    bool
	SuperAdmin     bool
}

var adminRouteAccess = buildAdminRouteAccess()

func buildAdminRouteAccess() map[string]RouteAccess {
	m := make(map[string]RouteAccess)
	set := func(permission string, entries ...string) {
		for _, entry := range entries {
			m[entry] = RouteAccess{AnyPermission: []string{permission}}
		}
	}
	setAny := func(permissions []string, entries ...string) {
		for _, entry := range entries {
			copyOfPermissions := append([]string(nil), permissions...)
			m[entry] = RouteAccess{AnyPermission: copyOfPermissions}
		}
	}
	setAnyAndAll := func(anyPermissions, allPermissions []string, entries ...string) {
		for _, entry := range entries {
			m[entry] = RouteAccess{
				AnyPermission:  append([]string(nil), anyPermissions...),
				AllPermissions: append([]string(nil), allPermissions...),
			}
		}
	}
	setSuper := func(entries ...string) {
		for _, entry := range entries {
			m[entry] = RouteAccess{SuperAdmin: true}
		}
	}

	// Session identity is available to every active admin so the UI can fail closed
	// on stale role data; it grants no business-module access.
	m["GET /admin/auth/me"] = RouteAccess{SessionOnly: true}

	set(PermissionReadOnly,
		"GET /admin/dashboard",
	)
	setAny([]string{PermissionReadOnly, PermissionOperations},
		"GET /admin/merchants",
		"GET /admin/merchants/:id",
		"GET /admin/merchant-follow-ups",
		"GET /admin/merchants/:id/follow-ups",
		"GET /admin/merchants/:id/stores",
		"GET /admin/merchants/:id/leads",
		"GET /admin/merchants/:id/leads/stats",
		"GET /admin/merchants/:id/ai/insights",
		"GET /admin/customers",
		"GET /admin/users",
		"GET /admin/group-buy-links",
		"GET /admin/group-buy-links/:id",
	)
	setAnyAndAll([]string{PermissionReadOnly, PermissionOperations}, []string{PermissionExportCustomerOrder},
		"GET /admin/customers/export",
	)
	set(PermissionOperations,
		"PUT /admin/merchants/:id/status",
		"POST /admin/merchants/:id/follow-ups",
		"PUT /admin/merchant-follow-ups/:id/status",
		"POST /admin/merchants/:id/stores",
		"PUT /admin/merchants/:id/stores/:store_id",
		"DELETE /admin/merchants/:id/stores/:store_id",
		"PUT /admin/merchants/:id/leads/:lead_id",
		"PUT /admin/users/:id/status",
		"PUT /admin/users/:id/phone",
		"PUT /admin/group-buy-links/:id/review",
	)

	set(PermissionFinance,
		"GET /admin/merchant-plans",
		"POST /admin/merchant-plans",
		"PUT /admin/merchant-plans/:id",
		"GET /admin/merchants/:id/payment-config",
		"PUT /admin/merchants/:id/payment-config",
		"POST /admin/merchants/:id/subscription/open",
		"POST /admin/merchants/:id/subscription/pause",
		"POST /admin/merchants/:id/subscription/resume",
		"POST /admin/merchants/:id/subscription/stop",
		"GET /admin/merchants/:id/subscription-orders",
		"GET /admin/merchants/:id/store-orders",
		"GET /admin/merchants/:id/settlement-prepare",
		"GET /admin/merchants/:id/settlements",
		"POST /admin/merchants/:id/settlements",
		"PUT /admin/settlements/:id/paid",
		"GET /admin/settlements/:id/export",
		"POST /admin/users/:id/member",
		"GET /admin/packages",
		"POST /admin/packages",
		"PUT /admin/packages/:id",
		"GET /admin/orders",
		"GET /admin/payments",
		"POST /admin/orders/:id/refund",
		"POST /admin/orders/:id/confirm-subscription-payment",
		"POST /admin/orders/:id/review-historical-subscription",
		"GET /admin/subscription-manual-reviews",
		"GET /admin/refunds",
		"GET /admin/refund-reconciliations",
		"GET /admin/refund-reconciliations/:id",
		"POST /admin/refund-reconciliations/:id/manual-review",
		"POST /admin/refund-reconciliations/:id/resolve",
		"GET /admin/cards",
		"POST /admin/cards/batch",
	)
	setAnyAndAll([]string{PermissionFinance}, []string{PermissionExportCustomerOrder},
		"GET /admin/orders/export",
	)

	set(PermissionSystem,
		"GET /admin/audit-logs",
		"POST /admin/audit-logs/:id/review",
		"GET /admin/security-check",
		"GET /admin/ai-config/status",
		"POST /admin/ai-config/policy",
		"GET /admin/ai-usage/overview",
		"POST /admin/payment-qrcode/upload",
		"GET /admin/configs",
		"POST /admin/configs",
	)
	setSuper(
		"GET /admin/access/permissions",
		"GET /admin/access/roles",
		"POST /admin/access/roles",
		"PUT /admin/access/roles/:id",
		"GET /admin/access/admins",
		"POST /admin/access/admins",
		"PUT /admin/access/admins/:id",
	)
	return m
}

// ResolveAdminRoute normalizes both supported API prefixes and then requires an
// exact HTTP-method plus registered Gin route-pattern match. Unknown routes deny.
func ResolveAdminRoute(method, fullPath string) (RouteAccess, bool) {
	path := fullPath
	for _, prefix := range []string{"/api/v1", "/api"} {
		if strings.HasPrefix(path, prefix+"/") {
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}
	access, ok := adminRouteAccess[method+" "+path]
	return access, ok
}

func isKnownPermission(code string) bool {
	for _, permission := range permissionCatalog {
		if permission.Code == code {
			return true
		}
	}
	return false
}
