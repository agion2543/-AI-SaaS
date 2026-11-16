package schema

import (
	"strings"
	"testing"
)

func TestWholesaleRetailV6DefinitionIsChecksumPinned(t *testing.T) {
	definition, err := loadWholesaleRetailV6()
	if err != nil {
		t.Fatalf("load wholesale/retail v6 definition: %v", err)
	}
	if definition.version != WholesaleRetailV6Version || definition.checksum != WholesaleRetailV6Checksum {
		t.Fatalf("loaded v6 identity=%q/%q, want %q/%q", definition.version, definition.checksum, WholesaleRetailV6Version, WholesaleRetailV6Checksum)
	}
	for _, column := range []string{"sales_unit", "specification", "minimum_order_quantity", "price_tiers"} {
		if !strings.Contains(definition.ddl, "`"+column+"`") {
			t.Errorf("v6 DDL is missing %s: %s", column, definition.ddl)
		}
	}
	if strings.Contains(strings.ToLower(definition.ddl), "drop ") || strings.Contains(strings.ToLower(definition.ddl), "update ") {
		t.Fatalf("v6 must be additive only: %s", definition.ddl)
	}
}
