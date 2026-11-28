package adminaccess

import "testing"

// The platform cost centre uses its own permission key, separate from the
// ordinary finance key used by subscription/payment routes. The routes stay
// super-admin gated today; the key is recorded on each route so a future
// role split can replace the gate without re-mapping routes.
func TestPlatformFinanceRoutesCarryDistinctPermissionKey(t *testing.T) {
	found := false
	for _, info := range PermissionCatalog() {
		if info.Code == PermissionPlatformFinance {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("platform_finance is missing from the permission catalog")
	}
	routes := []string{
		"GET /admin/platform-costs/overview",
		"POST /admin/platform-costs/expenses",
		"POST /admin/platform-costs/expenses/:id/reversals",
		"GET /admin/platform-costs/expenses/stats",
		"GET /admin/ai-cost-control/usage-stats",
		"GET /admin/ai-cost-control/model-prices",
		"POST /admin/ai-cost-control/model-prices",
		"POST /admin/ai-cost-control/vendor-bills",
		"GET /admin/ai-cost-control/vendor-bills",
		"GET /admin/ai-cost-control/bills/reconciliation",
		"GET /admin/ai-cost-control/budgets",
		"GET /admin/ai-cost-control/budgets/:merchantId/status",
		"PUT /admin/ai-cost-control/budgets/:merchantId",
		"GET /admin/ai-cost-control/budget-alerts",
		"GET /admin/ai-cost-control/emergency-switch",
		"PUT /admin/ai-cost-control/emergency-switch",
		"GET /admin/platform-finance/time-logs",
		"POST /admin/platform-finance/time-logs",
		"GET /admin/platform-finance/income-records",
		"POST /admin/platform-finance/income-records",
		"PUT /admin/platform-finance/income-records/:id",
		"GET /admin/platform-finance/mrr",
		"GET /admin/platform-finance/breakeven",
	}
	for _, route := range routes {
		access, ok := adminRouteAccess[route]
		if !ok {
			t.Fatalf("finance route %q has no authorization mapping", route)
		}
		if !access.SuperAdmin {
			t.Fatalf("finance route %q is not super-admin gated", route)
		}
		keyed := false
		for _, permission := range access.AllPermissions {
			if permission == PermissionPlatformFinance {
				keyed = true
				break
			}
		}
		if !keyed {
			t.Fatalf("finance route %q does not carry the platform_finance key", route)
		}
	}
	// Ordinary finance routes keep their own key and must not carry the
	// platform-finance key.
	ordinary, ok := adminRouteAccess["GET /admin/merchant-plans"]
	if !ok {
		t.Fatal("ordinary finance route GET /admin/merchant-plans has no mapping")
	}
	for _, permission := range append(append([]string(nil), ordinary.AnyPermission...), ordinary.AllPermissions...) {
		if permission == PermissionPlatformFinance {
			t.Fatal("ordinary finance route carries the platform_finance key")
		}
	}
	if ordinary.SuperAdmin {
		t.Fatal("ordinary finance route unexpectedly became super-admin gated")
	}
}
