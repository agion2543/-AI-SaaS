package controller

import (
	"fmt"
	"strings"
	"testing"

	"go-web-gin-health/internal/model"
)

func TestSerializePublicCustomerOrderOmitsInternalOrderFields(t *testing.T) {
	order := &model.Order{
		OrderNo:                 "SYNTHETIC-ORDER-001",
		CustomerPhone:           "15500001111",
		MerchantNote:            "internal merchant note",
		TransactionNo:           "internal transaction id",
		CustomerAccessToken:     "one-time token",
		CustomerAccessTokenHash: "token hash",
		TotalAmount:             0,
		OperationLogs:           `[{"action":"internal_note","time":"2026-11-12T00:00:00Z","detail":"internal log detail"}]`,
	}

	response := serializePublicCustomerOrder(order)
	for _, key := range []string{
		"customer_phone",
		"merchant_note",
		"transaction_no",
		"customer_access_token",
		"customer_access_token_hash",
		"user",
		"merchant",
		"package",
		"subscription",
	} {
		if _, exists := response[key]; exists {
			t.Errorf("public response must not expose %q", key)
		}
	}
	if got := response["total_amount"]; got != int64(0) {
		t.Errorf("zero payable amount must remain zero, got %#v", got)
	}
	if got := fmt.Sprint(response["operation_logs"]); strings.Contains(got, "internal") {
		t.Errorf("public operation log must omit internal details, got %s", got)
	}
}
