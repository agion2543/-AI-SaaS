package schema

import (
	"strings"
	"testing"
)

func TestLoadAdminAuthV3IsChecksumPinned(t *testing.T) {
	definition, err := loadAdminAuthV3()
	if err != nil {
		t.Fatalf("load admin auth v3 definition: %v", err)
	}
	if definition.version != AdminAuthV3Version || definition.checksum != AdminAuthV3Checksum {
		t.Fatalf("loaded v3 identity=%q/%q, want %q/%q", definition.version, definition.checksum, AdminAuthV3Version, AdminAuthV3Checksum)
	}
	if !strings.Contains(definition.ddl, "`admin_users`") || !strings.Contains(definition.ddl, "`auth_version` bigint unsigned NOT NULL DEFAULT 1") {
		t.Fatalf("unexpected admin auth v3 DDL: %s", definition.ddl)
	}
}
