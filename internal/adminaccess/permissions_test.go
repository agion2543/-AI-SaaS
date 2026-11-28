package adminaccess

import (
	"reflect"
	"strings"
	"testing"
)

func TestParsePermissionsUsesOnlyKnownExplicitCodes(t *testing.T) {
	if got := ParsePermissions("operations,invalid,finance,operations"); !reflect.DeepEqual(got, []string{"finance", "operations"}) {
		t.Fatalf("ParsePermissions() = %#v", got)
	}
	if got := ParsePermissions(`["system_admin","read_only","unknown"]`); !reflect.DeepEqual(got, []string{"read_only", "system_admin"}) {
		t.Fatalf("ParsePermissions(JSON) = %#v", got)
	}
	if got := ParsePermissions("dashboard,users,packages"); len(got) != 0 {
		t.Fatalf("legacy unused permission names granted access: %#v", got)
	}
}

func TestNormalizePermissionsRejectsUnknownCodesAndDeduplicates(t *testing.T) {
	got, err := NormalizePermissions([]string{PermissionOperations, PermissionReadOnly, PermissionOperations})
	if err != nil {
		t.Fatalf("NormalizePermissions() error = %v", err)
	}
	if got != "operations,read_only" {
		t.Fatalf("NormalizePermissions() = %q", got)
	}
	if _, err := NormalizePermissions([]string{"admin_all"}); err == nil {
		t.Fatal("unknown privilege code unexpectedly accepted")
	}
}

func TestAdminRouteMapIsExplicitByMethodAndPattern(t *testing.T) {
	cases := []struct {
		method  string
		path    string
		want    []string
		wantAll []string
		wantOK  bool
		super   bool
		session bool
	}{
		{method: "GET", path: "/api/v1/admin/merchants/:id", want: []string{PermissionReadOnly, PermissionOperations}, wantOK: true},
		{method: "PUT", path: "/api/admin/merchants/:id/status", want: []string{PermissionOperations}, wantOK: true},
		{method: "POST", path: "/api/admin/merchants/:id/stores", want: []string{PermissionOperations}, wantOK: true},
		{method: "PUT", path: "/api/admin/merchants/:id/stores/:store_id", want: []string{PermissionOperations}, wantOK: true},
		{method: "DELETE", path: "/api/admin/merchants/:id/stores/:store_id", want: []string{PermissionOperations}, wantOK: true},
		{method: "POST", path: "/api/v1/admin/merchants/:id/stores", want: []string{PermissionOperations}, wantOK: true},
		{method: "PUT", path: "/api/v1/admin/merchants/:id/stores/:store_id", want: []string{PermissionOperations}, wantOK: true},
		{method: "DELETE", path: "/api/v1/admin/merchants/:id/stores/:store_id", want: []string{PermissionOperations}, wantOK: true},
		{method: "POST", path: "/api/v1/admin/orders/:id/refund", want: []string{PermissionFinance}, wantOK: true},
		{method: "GET", path: "/api/v1/admin/customers/export", want: []string{PermissionReadOnly, PermissionOperations}, wantAll: []string{PermissionExportCustomerOrder}, wantOK: true},
		{method: "GET", path: "/api/admin/orders/export", want: []string{PermissionFinance}, wantAll: []string{PermissionExportCustomerOrder}, wantOK: true},
		{method: "GET", path: "/api/v1/admin/audit-logs", want: []string{PermissionSystem}, wantOK: true},
		{method: "GET", path: "/api/v1/admin/maintenance-notice", want: []string{PermissionSystem}, wantOK: true},
		{method: "PUT", path: "/api/admin/maintenance-notice", want: []string{PermissionSystem}, wantOK: true},
		{method: "GET", path: "/api/v1/admin/access/admins", wantOK: true, super: true},
		{method: "GET", path: "/api/v1/admin/refunds/pending-review", wantOK: true, super: true},
		{method: "POST", path: "/api/v1/admin/refunds/:id/review", wantOK: true, super: true},
		{method: "GET", path: "/api/v1/admin/refunds/review-threshold", wantOK: true, super: true},
		{method: "PUT", path: "/api/v1/admin/refunds/review-threshold", wantOK: true, super: true},
		{method: "GET", path: "/api/admin/refunds/pending-review", wantOK: true, super: true},
		{method: "POST", path: "/api/admin/refunds/:id/review", wantOK: true, super: true},
		{method: "GET", path: "/api/admin/refunds/review-threshold", wantOK: true, super: true},
		{method: "PUT", path: "/api/admin/refunds/review-threshold", wantOK: true, super: true},
		{method: "GET", path: "/api/v1/admin/refunds", want: []string{PermissionFinance}, wantOK: true},
		{method: "GET", path: "/api/v1/admin/auth/me", wantOK: true, session: true},
		{method: "POST", path: "/api/v1/admin/merchants/:id/status", wantOK: false},
		{method: "GET", path: "/api/v1/admin/not-mapped", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			access, ok := ResolveAdminRoute(tc.method, tc.path)
			if ok != tc.wantOK {
				t.Fatalf("ResolveAdminRoute() ok=%v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if !reflect.DeepEqual(access.AnyPermission, tc.want) || !reflect.DeepEqual(access.AllPermissions, tc.wantAll) || access.SuperAdmin != tc.super || access.SessionOnly != tc.session {
				t.Fatalf("route access = %#v; want permissions=%#v super=%v session=%v", access, tc.want, tc.super, tc.session)
			}
		})
	}
}

func TestRegisteredAdminPrefixesResolveIdenticalAuthorization(t *testing.T) {
	for route, expected := range adminRouteAccess {
		method, path, found := strings.Cut(route, " ")
		if !found {
			t.Fatalf("invalid registered admin route key %q", route)
		}
		for _, prefix := range []string{"/api", "/api/v1"} {
			access, ok := ResolveAdminRoute(method, prefix+path)
			if !ok {
				t.Errorf("registered route %q was not resolved through %s", route, prefix)
				continue
			}
			if !reflect.DeepEqual(access, expected) {
				t.Errorf("route %q through %s resolved to %#v, want %#v", route, prefix, access, expected)
			}
		}
	}
}
