package service

import (
	"testing"
	"time"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestCustomerOrderAccessTokenIsRandomAndHashOnly(t *testing.T) {
	first, firstHash, err := newCustomerOrderAccessToken()
	if err != nil {
		t.Fatal(err)
	}
	second, secondHash, err := newCustomerOrderAccessToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || firstHash == secondHash {
		t.Fatal("separate orders must receive separate access tokens")
	}
	if first == firstHash || len(firstHash) != 64 {
		t.Fatal("access token hash must be a 64-character SHA-256 hex digest")
	}
	order := &model.Order{CustomerAccessTokenHash: firstHash}
	if !customerOrderAccessTokenMatches(order, first) {
		t.Fatal("issued token must match its stored hash")
	}
	if customerOrderAccessTokenMatches(order, "") || customerOrderAccessTokenMatches(order, second) {
		t.Fatal("missing or unrelated token must not authorize access")
	}
	if customerOrderAccessTokenMatches(&model.Order{}, first) {
		t.Fatal("legacy order without a stored hash must not authorize access")
	}
}

func TestZeroPayableOrderDoesNotRequirePayment(t *testing.T) {
	free := model.Order{TotalAmount: 0}
	unpaid := model.Order{TotalAmount: 2468}
	paidAt := time.Now()
	paid := model.Order{TotalAmount: 2468, PaidAt: &paidAt}
	if !orderPaymentSatisfied(free) || orderPaymentSatisfied(unpaid) || !orderPaymentSatisfied(paid) {
		t.Fatal("payment satisfaction must distinguish free, unpaid, and paid orders")
	}
	if orderHasConfirmedPaidAmount(free) || orderHasConfirmedPaidAmount(unpaid) || !orderHasConfirmedPaidAmount(paid) {
		t.Fatal("only positive-value orders with paid_at may count as confirmed receipts")
	}
}

func TestPaymentCallbackAmountRejectsZeroPayableStoreOrder(t *testing.T) {
	cases := []struct {
		name    string
		order   model.Order
		want    int64
		wantErr bool
	}{
		{name: "positive store order", order: model.Order{OrderType: "store_order", TotalAmount: 1234, Amount: 2468}, want: 1234},
		{name: "legacy store order with StoreID", order: model.Order{StoreID: new(uint), TotalAmount: 1234, Amount: 2468}, want: 1234},
		{name: "zero store order", order: model.Order{OrderType: "store_order", Amount: 2468}, wantErr: true},
		{name: "legacy zero store order with StoreID", order: model.Order{StoreID: new(uint), Amount: 2468}, wantErr: true},
		{name: "negative store order", order: model.Order{OrderType: "store_order", TotalAmount: -1, Amount: 2468}, wantErr: true},
		{name: "legacy non-store fallback", order: model.Order{OrderType: "subscription", Amount: 500}, want: 500},
		{name: "missing non-store amount", order: model.Order{OrderType: "subscription"}, wantErr: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := expectedPaymentCallbackAmount(test.order)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("amount = %d, want %d", got, test.want)
			}
		})
	}
}

func TestPaymentCallbackAmountMustMatchForSuccessAndFailure(t *testing.T) {
	order := model.Order{OrderType: "store_order", TotalAmount: 2345, Amount: 2468}
	cases := []struct {
		name           string
		callbackAmount int64
		wantErr        bool
	}{
		{name: "matching success callback", callbackAmount: 2345},
		{name: "matching failure callback", callbackAmount: 2345},
		{name: "mismatched success callback", callbackAmount: 0, wantErr: true},
		{name: "mismatched failure callback", callbackAmount: 0, wantErr: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := validatePaymentCallbackAmount(order, test.callbackAmount)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestSettlementRequiresPositiveConfirmedReceipt(t *testing.T) {
	paidAt := time.Now()
	paid := model.Order{OrderType: "store_order", Status: "accepted", TotalAmount: 2468, PaidAt: &paidAt}
	unpaid := paid
	unpaid.PaidAt = nil
	free := paid
	free.TotalAmount = 0
	wrongStatus := paid
	wrongStatus.Status = "submitted"
	settled := paid
	settled.SettlementID = new(uint)
	if !isSettlementEligible(paid) {
		t.Fatal("confirmed paid accepted order should be settlement-eligible")
	}
	for name, order := range map[string]model.Order{"unpaid": unpaid, "free": free, "wrong status": wrongStatus, "already settled": settled} {
		if isSettlementEligible(order) {
			t.Errorf("%s order must not be settlement-eligible", name)
		}
	}
	if amount := settlementOrderAmount(free); amount != 0 {
		t.Fatalf("zero payable amount must remain zero, got %d", amount)
	}
}

func TestRefundAmountRequiresPositivePaidOrderAndPositivePartialAmount(t *testing.T) {
	paidAt := time.Now()
	paid := model.Order{TotalAmount: 2468, RefundedAmount: 468, PaidAt: &paidAt}
	cases := []struct {
		name    string
		order   model.Order
		request dto.RefundOrderRequest
		want    int64
		wantErr bool
	}{
		{name: "partial", order: paid, request: dto.RefundOrderRequest{Amount: 100}, want: 100},
		{name: "full", order: paid, request: dto.RefundOrderRequest{Full: true}, want: 2000},
		{name: "negative full input", order: paid, request: dto.RefundOrderRequest{Full: true, Amount: -1}, wantErr: true},
		{name: "zero partial", order: paid, request: dto.RefundOrderRequest{Amount: 0}, wantErr: true},
		{name: "negative partial", order: paid, request: dto.RefundOrderRequest{Amount: -1}, wantErr: true},
		{name: "too much", order: paid, request: dto.RefundOrderRequest{Amount: 2001}, wantErr: true},
		{name: "unpaid", order: model.Order{TotalAmount: 2468}, request: dto.RefundOrderRequest{Full: true}, wantErr: true},
		{name: "free", order: model.Order{TotalAmount: 0, PaidAt: &paidAt}, request: dto.RefundOrderRequest{Full: true}, wantErr: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveRefundAmount(test.order, test.request)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("amount = %d, want %d", got, test.want)
			}
		})
	}
}
