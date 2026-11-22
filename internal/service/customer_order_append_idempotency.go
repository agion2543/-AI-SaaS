package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *OrderService) AppendCustomerStoreOrderItems(orderNo, accessToken string, req dto.AppendCustomerOrderItemsRequest) (*model.Order, error) {
	var requestKeyHash string
	if req.RequestKey != "" {
		if err := validateCustomerAppendRequestKey(req.RequestKey); err != nil {
			return nil, err
		}
		fingerprint, err := customerAppendRequestFingerprint(req)
		if err != nil {
			return nil, err
		}
		req.RequestFingerprint = fingerprint
		requestKeyHash = hashCustomerAppendRequestKey(req.RequestKey)
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var lockedOrder model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", orderNo).First(&lockedOrder).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("order not found")
			}
			return err
		}
		if lockedOrder.OrderType != "store_order" || !customerOrderAccessTokenMatches(&lockedOrder, accessToken) {
			return errors.New("order not found")
		}

		if requestKeyHash != "" {
			replayed, err := findCustomerAppendReplay(lockedOrder.OperationLogs, requestKeyHash, req.RequestFingerprint)
			if err != nil {
				return err
			}
			if replayed {
				return nil
			}
		}

		txService := *s
		txService.db = tx
		txService.orders = repository.NewOrderRepository(tx)
		txService.stores = repository.NewStoreRepository(tx)
		txService.products = repository.NewStoreProductRepository(tx)
		_, err := txService.appendCustomerStoreOrderItemsLocked(orderNo, accessToken, req)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.orders.FindByOrderNo(orderNo)
}

func (s *OrderService) RecoverCustomerStoreOrderAppend(orderNo, accessToken, requestKey string) (*model.Order, error) {
	if validateCustomerAppendRequestKey(requestKey) != nil {
		return nil, errors.New("order not found")
	}
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil || order.OrderType != "store_order" || !customerOrderAccessTokenMatches(order, accessToken) {
		return nil, errors.New("order not found")
	}
	matched, err := findCustomerAppendRequestKey(order.OperationLogs, hashCustomerAppendRequestKey(requestKey))
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, errors.New("order not found")
	}
	return order, nil
}

func validateCustomerAppendRequestKey(requestKey string) error {
	decoded, err := hex.DecodeString(requestKey)
	if err != nil || len(decoded) != 32 {
		return errors.New("customer order append request key must be 32 random bytes encoded as hexadecimal")
	}
	return nil
}

func hashCustomerAppendRequestKey(requestKey string) string {
	sum := sha256.Sum256([]byte(requestKey))
	return hex.EncodeToString(sum[:])
}

func customerAppendRequestFingerprint(req dto.AppendCustomerOrderItemsRequest) (string, error) {
	items, err := aggregateCustomerOrderItems(req.Items)
	if err != nil {
		return "", err
	}
	type itemFingerprint struct {
		ProductID uint `json:"product_id"`
		Quantity  int  `json:"quantity"`
	}
	canonicalItems := make([]itemFingerprint, 0, len(items))
	for _, item := range items {
		canonicalItems = append(canonicalItems, itemFingerprint{ProductID: item.ProductID, Quantity: item.Quantity})
	}
	sort.Slice(canonicalItems, func(i, j int) bool { return canonicalItems[i].ProductID < canonicalItems[j].ProductID })
	payload, err := json.Marshal(struct {
		CustomerNote string            `json:"customer_note"`
		Items        []itemFingerprint `json:"items"`
	}{CustomerNote: strings.TrimSpace(req.CustomerNote), Items: canonicalItems})
	if err != nil {
		return "", fmt.Errorf("encode customer order append request: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func findCustomerAppendReplay(rawLogs, requestKeyHash, fingerprint string) (bool, error) {
	var logs []OrderOperationLog
	if strings.TrimSpace(rawLogs) != "" {
		if err := json.Unmarshal([]byte(rawLogs), &logs); err != nil {
			return false, errors.New("order operation logs are invalid")
		}
	}
	for _, log := range logs {
		if log.Action != "items_appended" || log.AppendRequestKeyHash != requestKeyHash {
			continue
		}
		if log.AppendRequestFingerprint != fingerprint {
			return false, errors.New("this append request key has already been used for a different request")
		}
		return true, nil
	}
	return false, nil
}

func findCustomerAppendRequestKey(rawLogs, requestKeyHash string) (bool, error) {
	var logs []OrderOperationLog
	if strings.TrimSpace(rawLogs) != "" {
		if err := json.Unmarshal([]byte(rawLogs), &logs); err != nil {
			return false, errors.New("order operation logs are invalid")
		}
	}
	for _, log := range logs {
		if log.Action == "items_appended" && log.AppendRequestKeyHash == requestKeyHash {
			return true, nil
		}
	}
	return false, nil
}

func appendCustomerOrderItemsLog(raw, text, requestKey, fingerprint string) (string, error) {
	var logs []OrderOperationLog
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &logs); err != nil {
			return "", errors.New("order operation logs are invalid")
		}
	}
	entry := OrderOperationLog{Action: "items_appended", Text: text, Time: time.Now().Format(time.RFC3339)}
	if requestKey != "" {
		if fingerprint == "" {
			return "", errors.New("customer order append request fingerprint is required")
		}
		entry.AppendRequestKeyHash = hashCustomerAppendRequestKey(requestKey)
		entry.AppendRequestFingerprint = fingerprint
	}
	logs = append(logs, entry)
	encoded, err := json.Marshal(logs)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
