package controller

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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

func TestMerchantOrderDTOJSONContract(t *testing.T) {
	paidAt := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	storeID, merchantID, userID, settlementID := uint(9), uint(10), uint(11), uint(12)
	packageID, merchantPlanID, promotionID := uint(13), uint(14), uint(15)
	order := &model.Order{
		BaseModel:      model.BaseModel{ID: 25, CreatedAt: paidAt, UpdatedAt: paidAt},
		OrderNo:        "SYNTHETIC-MERCHANT-ORDER-001",
		OrderType:      "store_order",
		UserID:         &userID,
		User:           &model.User{},
		MerchantID:     &merchantID,
		Merchant:       &model.Merchant{},
		SettlementID:   &settlementID,
		Settlement:     &model.MerchantSettlement{},
		StoreID:        &storeID,
		Store:          &model.Store{BaseModel: model.BaseModel{ID: storeID}, MerchantID: merchantID, Name: "合成门店", Address: "private address", ContactPhone: "13300009999", Status: "active", BusinessHours: "09:00-18:00"},
		CustomerPhone:  "13800138000",
		PackageID:      &packageID,
		Package:        &model.MembershipPackage{},
		MerchantPlanID: &merchantPlanID,
		MerchantPlan:   &model.MerchantPlan{},
		PromotionID:    &promotionID,
		Promotion:      &model.Promotion{},
		Amount:         1200,
		TotalAmount:    1200,
		RefundStatus:   "none",
		Items:          `[{"product_id":41,"name":"合成商品","price":1200,"quantity":1,"line_amount":1200,"description":"商品描述","image_url":"/synthetic.png","transaction_no":"hidden-item-transaction","user":{"phone":"hidden-item-phone"}}]`,
		CustomerNote:   "synthetic customer note",
		MerchantNote:   "synthetic internal note",
		OperationLogs:  `[{"action":"accepted","text":"合成接单记录","time":"2026-11-15T00:00:00Z","actor_phone":"hidden-log-phone","transaction_no":"hidden-log-transaction"}]`,
		CustomerRating: 5,
		CustomerReview: "synthetic review",
		Status:         "completed",
		PaymentChannel: "alipay",
		PaidAt:         &paidAt,
		TransactionNo:  "synthetic-private-transaction",
	}

	summaryJSON, err := json.Marshal(serializeMerchantOrderSummary(order))
	if err != nil {
		t.Fatalf("marshal merchant order summary: %v", err)
	}
	summary := decodeControllerJSON(t, summaryJSON)
	if got := summary["customer_phone"]; got != "138****8000" {
		t.Errorf("summary phone = %#v, want masked value", got)
	}
	for _, key := range []string{
		"transaction_no", "customer_note", "merchant_note", "operation_logs", "user", "merchant", "package", "merchant_plan", "promotion", "settlement",
		"user_id", "merchant_id", "package_id", "merchant_plan_id", "promotion_id", "referral_coupon_id", "coupon_no", "customer_rating", "customer_review", "reviewed_at", "auto_renew", "subscription_end_at", "manual_review", "manual_review_audit",
	} {
		if _, exists := summary[key]; exists {
			t.Errorf("merchant summary must not expose %q", key)
		}
	}
	if strings.Contains(string(summaryJSON), order.CustomerPhone) || strings.Contains(string(summaryJSON), "synthetic-private-transaction") {
		t.Error("merchant summary leaked full customer phone or raw transaction number")
	}
	if summary["has_customer_note"] != true || summary["has_merchant_note"] != true {
		t.Errorf("summary note flags should retain only whether notes exist: %#v", summary)
	}
	store, ok := summary["store"].(map[string]any)
	if !ok || len(store) != 2 || store["id"] != float64(storeID) || store["name"] != "合成门店" {
		t.Errorf("store relation must be only id/name summary, got %#v", summary["store"])
	}
	items, ok := summary["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("summary items = %#v, want one whitelisted item", summary["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["name"] != "合成商品" || item["quantity"] != float64(1) {
		t.Fatalf("summary item fields = %#v", items[0])
	}
	for _, key := range []string{"description", "image_url", "transaction_no", "user"} {
		if _, exists := item[key]; exists {
			t.Errorf("list item summary must not expose %q", key)
		}
	}

	detailJSON, err := json.Marshal(serializeMerchantOrderDetail(order))
	if err != nil {
		t.Fatalf("marshal merchant order detail: %v", err)
	}
	detail := decodeControllerJSON(t, detailJSON)
	if detail["customer_phone"] != order.CustomerPhone || detail["customer_note"] != order.CustomerNote || detail["merchant_note"] != order.MerchantNote {
		t.Errorf("audited detail should retain full phone and notes: %#v", detail)
	}
	if _, exists := detail["transaction_no"]; exists {
		t.Error("merchant order detail must not expose raw transaction number")
	}
	for _, key := range []string{"user", "merchant", "package", "merchant_plan", "promotion", "settlement"} {
		if _, exists := detail[key]; exists {
			t.Errorf("merchant detail must not expose nested association %q", key)
		}
	}
	if strings.Contains(string(detailJSON), "hidden-item-transaction") || strings.Contains(string(detailJSON), "hidden-log-transaction") || strings.Contains(string(detailJSON), "hidden-item-phone") || strings.Contains(string(detailJSON), "hidden-log-phone") {
		t.Error("merchant detail leaked fields outside the allowed item/log snapshots")
	}
	logs, ok := detail["operation_logs"].([]any)
	if !ok || len(logs) != 1 {
		t.Fatalf("detail operation logs = %#v, want one sanitized log", detail["operation_logs"])
	}
	logEntry, ok := logs[0].(map[string]any)
	if !ok || len(logEntry) != 3 || logEntry["action"] != "accepted" || logEntry["text"] != "合成接单记录" {
		t.Errorf("detail log fields must be only action/text/time, got %#v", logs[0])
	}
	if itemDetail, ok := detail["items"].([]any); !ok || len(itemDetail) != 1 || itemDetail[0].(map[string]any)["description"] != "商品描述" {
		t.Errorf("audited detail should retain product description snapshot, got %#v", detail["items"])
	}
}

func TestSerializeMerchantRefundsOmitsOrderPIIAndRawPayload(t *testing.T) {
	storeID, merchantID := uint(9), uint(10)
	createdAt := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	refund := model.RefundRecord{
		BaseModel:     model.BaseModel{ID: 3, CreatedAt: createdAt},
		OrderID:       25,
		Order:         model.Order{OrderNo: "SYNTHETIC-ORDER-REFUND", CustomerPhone: "15500001111", TransactionNo: "hidden-order-transaction", StoreID: &storeID, Store: &model.Store{MerchantID: merchantID, Name: "private store", ContactPhone: "hidden-store-phone"}, User: &model.User{}, Merchant: &model.Merchant{}},
		MerchantID:    &merchantID,
		StoreID:       &storeID,
		RefundNo:      "SYNTHETIC-REFUND-001",
		TransactionNo: "hidden-refund-transaction",
		Amount:        300,
		Reason:        "synthetic refund reason",
		OperatorRole:  "merchant",
		OperatorID:    88,
		Status:        "success",
		RawPayload:    `{"mode":"manual_record","private_payload":"hidden-refund-payload"}`,
	}
	responseJSON, err := json.Marshal(serializeMerchantRefunds([]model.RefundRecord{refund}))
	if err != nil {
		t.Fatalf("marshal merchant refund summary: %v", err)
	}
	var response []map[string]any
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		t.Fatalf("decode merchant refund summary: %v", err)
	}
	if !strings.Contains(string(responseJSON), `"manual_record":true`) || !strings.Contains(string(responseJSON), refund.RefundNo) || !strings.Contains(string(responseJSON), refund.Reason) {
		t.Errorf("refund summary omitted visible finance fields: %s", responseJSON)
	}
	for _, marker := range []string{refund.Order.CustomerPhone, refund.Order.TransactionNo, refund.TransactionNo, "hidden-refund-payload", `"raw_payload"`, `"operator_id"`, `"idempotency_key"`, `"merchant_id"`, `"user":`, `"merchant":`, `"contact_phone"`} {
		if strings.Contains(string(responseJSON), marker) {
			t.Errorf("merchant refund summary leaked %q: %s", marker, responseJSON)
		}
	}
	if len(response) != 1 {
		t.Errorf("refund summary should be one row, got %#v", response)
	}
}

func decodeControllerJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode JSON object: %v", err)
	}
	return result
}
