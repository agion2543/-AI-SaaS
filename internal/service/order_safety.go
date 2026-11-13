package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func newCustomerOrderAccessToken() (string, string, error) {
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(secret[:])
	return token, hashCustomerOrderAccessToken(token), nil
}

func hashCustomerOrderAccessToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func customerOrderAccessTokenMatches(order *model.Order, token string) bool {
	if order == nil || token == "" || order.CustomerAccessTokenHash == "" {
		return false
	}
	providedHash := hashCustomerOrderAccessToken(token)
	return subtle.ConstantTimeCompare([]byte(order.CustomerAccessTokenHash), []byte(providedHash)) == 1
}

func expectedPaymentCallbackAmount(order model.Order) (int64, error) {
	if order.OrderType == "store_order" || order.StoreID != nil {
		if order.TotalAmount <= 0 {
			return 0, errors.New("store order payable amount must be greater than zero")
		}
		return order.TotalAmount, nil
	}

	expectedAmount := order.TotalAmount
	if expectedAmount <= 0 {
		expectedAmount = order.Amount
	}
	if expectedAmount <= 0 {
		return 0, errors.New("payment amount must be greater than zero")
	}
	return expectedAmount, nil
}

func validatePaymentCallbackAmount(order model.Order, callbackAmount int64) error {
	expectedAmount, err := expectedPaymentCallbackAmount(order)
	if err != nil {
		return err
	}
	if expectedAmount != callbackAmount {
		return errors.New("payment amount mismatch")
	}
	return nil
}

func orderPaymentSatisfied(order model.Order) bool {
	return order.TotalAmount == 0 || order.PaidAt != nil
}

// Deferred-settlement store orders may enter fulfillment without payment, but
// fulfillment must never create a payment fact; settlement still requires PaidAt.
func orderCanProceedToFulfillment(order model.Order) bool {
	if orderPaymentSatisfied(order) {
		return true
	}
	return order.OrderType == "store_order" && order.TotalAmount > 0 && orderHasAction(order.OperationLogs, "submitted")
}

func orderHasConfirmedPaidAmount(order model.Order) bool {
	return order.TotalAmount > 0 && order.PaidAt != nil
}

func isSettlementEligible(order model.Order) bool {
	if order.OrderType != "store_order" || order.SettlementID != nil || !orderHasConfirmedPaidAmount(order) {
		return false
	}
	return order.Status == "accepted" || order.Status == "completed"
}

func settlementOrderAmount(order model.Order) int64 {
	if order.TotalAmount < 0 {
		return 0
	}
	return order.TotalAmount
}

func resolveRefundAmount(order model.Order, req dto.RefundOrderRequest) (int64, error) {
	return resolveRefundAmountAgainstRemaining(order, req, order.TotalAmount-order.RefundedAmount)
}

func resolveRefundAmountAgainstRemaining(order model.Order, req dto.RefundOrderRequest, remaining int64) (int64, error) {
	if req.Amount < 0 {
		return 0, errors.New("refund amount must not be negative")
	}
	if !orderHasConfirmedPaidAmount(order) {
		return 0, errors.New("only confirmed paid orders can be refunded")
	}
	if remaining <= 0 {
		return 0, errors.New("order has no refundable amount")
	}
	amount := req.Amount
	if req.Full {
		amount = remaining
	} else if amount <= 0 {
		return 0, errors.New("refund amount must be greater than zero")
	}
	if amount > remaining {
		return 0, errors.New("refund amount exceeds remaining refundable amount")
	}
	return amount, nil
}
