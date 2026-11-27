package service

import (
	"testing"
	"time"

	"go-web-gin-health/internal/model"
)

// MerchantTurnover counts only the merchant's own valid trades (paid_at set
// and total_amount > 0), split by channel, minus successful refunds. Unpaid
// orders, zero-amount orders and other merchants' orders must not move it.
func TestMerchantTurnover(t *testing.T) {
	db := newTradeAmountTestDB(t)
	now := time.Now()
	merchant := model.Merchant{Name: "turnover-merchant", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatal(err)
	}
	otherMerchant := model.Merchant{Name: "turnover-other-merchant", Status: "active"}
	if err := db.Create(&otherMerchant).Error; err != nil {
		t.Fatal(err)
	}
	store1 := model.Store{MerchantID: merchant.ID, Name: "turnover-store-1", Status: "active", IsOpen: true}
	if err := db.Create(&store1).Error; err != nil {
		t.Fatal(err)
	}
	store2 := model.Store{MerchantID: merchant.ID, Name: "turnover-store-2", Status: "active", IsOpen: true}
	if err := db.Create(&store2).Error; err != nil {
		t.Fatal(err)
	}

	insertTradeOrder(t, db, "TURN-PAID-QR", merchant.ID, store1.ID, "store_order", 1000, &now, "store_qr")
	insertTradeOrder(t, db, "TURN-PAID-TABLE", merchant.ID, store1.ID, "store_order", 2000, &now, "table")
	insertTradeOrder(t, db, "TURN-PAID-STORE2", merchant.ID, store2.ID, "store_order", 400, &now, "share")
	insertTradeOrder(t, db, "TURN-UNPAID", merchant.ID, store1.ID, "store_order", 5000, nil, "store_qr")
	insertTradeOrder(t, db, "TURN-ZERO", merchant.ID, store1.ID, "store_order", 0, &now, "store_qr")
	insertTradeOrder(t, db, "TURN-OTHER-MERCHANT", otherMerchant.ID, 0, "store_order", 99999, &now, "store_qr")

	refundStore1 := model.RefundRecord{MerchantID: &merchant.ID, StoreID: &store1.ID, RefundNo: "TURN-REFUND-1", Amount: 300, Status: "success"}
	if err := db.Create(&refundStore1).Error; err != nil {
		t.Fatal(err)
	}
	refundStore2 := model.RefundRecord{MerchantID: &merchant.ID, StoreID: &store2.ID, RefundNo: "TURN-REFUND-2", Amount: 100, Status: "success"}
	if err := db.Create(&refundStore2).Error; err != nil {
		t.Fatal(err)
	}
	pendingRefund := model.RefundRecord{MerchantID: &merchant.ID, StoreID: &store1.ID, RefundNo: "TURN-REFUND-PENDING", Amount: 700, Status: "processing"}
	if err := db.Create(&pendingRefund).Error; err != nil {
		t.Fatal(err)
	}

	service := NewMerchantService(nil, db)

	all, err := service.MerchantTurnover(merchant.ID, "2000-01-01", "2999-01-01", 0)
	if err != nil {
		t.Fatalf("MerchantTurnover (all stores): %v", err)
	}
	if got := all["order_count"]; got != int64(3) {
		t.Fatalf("all order_count=%v, want 3", got)
	}
	if got := all["trade_amount"]; got != int64(3400) {
		t.Fatalf("all trade_amount=%v, want 3400", got)
	}
	if got := all["refund_amount"]; got != int64(400) {
		t.Fatalf("all refund_amount=%v, want 400", got)
	}
	if got := all["net_amount"]; got != int64(3000) {
		t.Fatalf("all net_amount=%v, want 3000", got)
	}
	byChannel, ok := all["by_channel"].([]merchantTurnoverChannelRow)
	if !ok {
		t.Fatalf("by_channel has unexpected type %T", all["by_channel"])
	}
	channelAmounts := map[string]int64{}
	for _, row := range byChannel {
		channelAmounts[row.Channel] = row.TradeAmount
	}
	if len(channelAmounts) != 3 || channelAmounts["store_qr"] != 1000 || channelAmounts["table"] != 2000 || channelAmounts["share"] != 400 {
		t.Fatalf("by_channel=%v, want store_qr=1000 table=2000 share=400", channelAmounts)
	}

	oneStore, err := service.MerchantTurnover(merchant.ID, "2000-01-01", "2999-01-01", store1.ID)
	if err != nil {
		t.Fatalf("MerchantTurnover (one store): %v", err)
	}
	if got := oneStore["order_count"]; got != int64(2) {
		t.Fatalf("store order_count=%v, want 2", got)
	}
	if got := oneStore["trade_amount"]; got != int64(3000) {
		t.Fatalf("store trade_amount=%v, want 3000", got)
	}
	if got := oneStore["refund_amount"]; got != int64(300) {
		t.Fatalf("store refund_amount=%v, want 300", got)
	}
	if got := oneStore["net_amount"]; got != int64(2700) {
		t.Fatalf("store net_amount=%v, want 2700", got)
	}

	// A store_id of another merchant can never widen the scope: the
	// merchant_id filter still applies, so the result is empty, not leaked.
	foreignStore := model.Store{MerchantID: otherMerchant.ID, Name: "turnover-foreign-store", Status: "active", IsOpen: true}
	if err := db.Create(&foreignStore).Error; err != nil {
		t.Fatal(err)
	}
	leak, err := service.MerchantTurnover(merchant.ID, "2000-01-01", "2999-01-01", foreignStore.ID)
	if err != nil {
		t.Fatalf("MerchantTurnover (foreign store): %v", err)
	}
	if got := leak["trade_amount"]; got != int64(0) {
		t.Fatalf("foreign-store trade_amount=%v, want 0", got)
	}
	if got := leak["order_count"]; got != int64(0) {
		t.Fatalf("foreign-store order_count=%v, want 0", got)
	}
}
