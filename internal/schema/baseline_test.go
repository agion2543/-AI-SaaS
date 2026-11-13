package schema

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"go-web-gin-health/sql/baselines"
)

func TestCandidateBaselineChecksumAndAuditTriggers(t *testing.T) {
	sum := sha256.Sum256(baselines.CandidateSchemaV1)
	if got := fmt.Sprintf("%x", sum); got != BaselineChecksum {
		t.Fatalf("baseline checksum = %s, want %s", got, BaselineChecksum)
	}
	definition, err := loadBaseline()
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.tables) != 29 {
		t.Fatalf("candidate table count = %d, want 29", len(definition.tables))
	}
	if len(definition.triggers) != 2 {
		t.Fatalf("candidate trigger count = %d, want 2", len(definition.triggers))
	}
	for name, event := range map[string]string{
		"trg_refund_reconciliation_audits_no_update": "UPDATE",
		"trg_refund_reconciliation_audits_no_delete": "DELETE",
	} {
		trigger := definition.triggers[name]
		if trigger.name == "" || trigger.event != event || trigger.timing != "BEFORE" || trigger.table != "refund_reconciliation_audits" || !strings.Contains(strings.ToUpper(trigger.stmt), "SIGNAL SQLSTATE '45000'") {
			t.Errorf("append-only trigger %s is not preserved: %+v", name, trigger)
		}
	}
}
