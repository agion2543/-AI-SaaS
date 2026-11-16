package controller

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/model"
)

func TestSerializeMerchantLeadSummariesOnlyIncludesVisibleFields(t *testing.T) {
	leads := []model.CustomerLead{{
		CustomerName:  "合成顾客",
		CustomerPhone: "15500001111",
		Message:       "synthetic customer note",
		FollowUpNote:  "synthetic follow-up note",
		Source:        "store_qr",
		Status:        "new",
		Merchant:      model.Merchant{Name: "synthetic merchant"},
		Store:         model.Store{Name: "synthetic store"},
	}}
	rows := serializeMerchantLeadSummaries(leads)
	if len(rows) != 1 || rows[0].CustomerName != "合成顾客" || rows[0].CustomerPhone != "15500001111" || rows[0].Status != "new" {
		t.Fatalf("unexpected lead summary: %+v", rows)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal lead summary: %v", err)
	}
	encoded := string(raw)
	for _, forbidden := range []string{"synthetic customer note", "synthetic follow-up note", "synthetic merchant", "synthetic store", "message", "follow_up_note", "source", "merchant", "store"} {
		if strings.Contains(encoded, forbidden) {
			t.Errorf("lead summary must not expose %q: %s", forbidden, encoded)
		}
	}
}
