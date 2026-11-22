package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

func validateCustomerOrderRequestCredentials(requestKey, accessToken string) error {
	if requestKey == "" {
		if accessToken != "" {
			return errors.New("access token requires an order request key")
		}
		return nil
	}
	for _, credential := range []struct {
		name  string
		value string
	}{{"request key", requestKey}, {"access token", accessToken}} {
		decoded, err := hex.DecodeString(credential.value)
		if err != nil || len(decoded) != 32 {
			return fmt.Errorf("customer order %s must be 32 random bytes encoded as hexadecimal", credential.name)
		}
	}
	return nil
}

func customerOrderNoForRequestKey(requestKey string) string {
	sum := sha256.Sum256([]byte(requestKey))
	// Keep the familiar SORD prefix and stay well within orders.order_no's
	// existing 50-character limit while retaining a 128-bit unique suffix.
	return "SORD" + hex.EncodeToString(sum[:16])
}

func (s *OrderService) findIdempotentCustomerOrder(req dto.CreateCustomerOrderRequest) (*model.Order, bool, error) {
	if req.RequestKey == "" {
		return nil, false, nil
	}
	order, err := s.orders.FindByOrderNo(customerOrderNoForRequestKey(req.RequestKey))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !customerOrderMatchesCreateRequest(order, req) || !customerOrderAccessTokenMatches(order, req.AccessToken) {
		return nil, false, errors.New("this order request key has already been used for a different or unauthorized request")
	}
	order.CustomerAccessToken = req.AccessToken
	return order, true, nil
}

func (s *OrderService) RecoverCustomerStoreOrder(requestKey, accessToken string) (*model.Order, error) {
	if err := validateCustomerOrderRequestCredentials(requestKey, accessToken); err != nil {
		return nil, errors.New("order not found")
	}
	order, err := s.orders.FindByOrderNo(customerOrderNoForRequestKey(requestKey))
	if err != nil || order.OrderType != "store_order" || !customerOrderAccessTokenMatches(order, accessToken) {
		return nil, errors.New("order not found")
	}
	order.CustomerAccessToken = accessToken
	return order, nil
}

func customerOrderMatchesCreateRequest(order *model.Order, req dto.CreateCustomerOrderRequest) bool {
	if order == nil || order.OrderType != "store_order" || order.StoreID == nil || *order.StoreID != req.StoreID {
		return false
	}
	if order.CustomerPhone != normalizePhone(req.CustomerPhone) ||
		order.CustomerNote != strings.TrimSpace(req.CustomerNote) ||
		order.ShareCode != strings.TrimSpace(req.ShareCode) ||
		order.CouponNo != strings.TrimSpace(req.CouponNo) {
		return false
	}
	expectedChannel := req.PaymentChannel
	if expectedChannel == "" {
		expectedChannel = "alipay"
	}
	if order.PaymentChannel != expectedChannel {
		return false
	}

	expectedItems, err := aggregateCustomerOrderItems(req.Items)
	if err != nil {
		return false
	}
	var snapshots []StoreOrderItemSnapshot
	if err := json.Unmarshal([]byte(order.Items), &snapshots); err != nil || len(snapshots) != len(expectedItems) {
		return false
	}
	expectedQuantities := make(map[uint]int, len(expectedItems))
	for _, item := range expectedItems {
		expectedQuantities[item.ProductID] = item.Quantity
	}
	for _, snapshot := range snapshots {
		quantity, ok := expectedQuantities[snapshot.ProductID]
		if !ok || quantity != snapshot.Quantity {
			return false
		}
		delete(expectedQuantities, snapshot.ProductID)
	}
	return len(expectedQuantities) == 0
}
