package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

type refundReservationTotals struct {
	SuccessfulAmount int64 `gorm:"column:successful_amount"`
	ReservedAmount   int64 `gorm:"column:reserved_amount"`
}

type refundFingerprintInput struct {
	Amount       int64  `json:"amount"`
	Full         bool   `json:"full"`
	Reason       string `json:"reason"`
	OperatorRole string `json:"operator_role"`
	OperatorID   uint   `json:"operator_id"`
}

func (s *OrderService) processReservedRefund(order *model.Order, req dto.RefundOrderRequest, operatorID uint, operatorRole string, audit ...AuditEvent) (*model.Order, error) {
	if order == nil || order.OrderType != "store_order" {
		return nil, errors.New("only store orders can be refunded here")
	}
	if req.Amount < 0 {
		return nil, errors.New("refund amount must not be negative")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "售后退款"
	}
	if len([]rune(reason)) > 255 {
		return nil, errors.New("refund reason cannot exceed 255 characters")
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if len(idempotencyKey) > 120 {
		return nil, errors.New("refund idempotency key cannot exceed 120 characters")
	}
	fingerprint, err := refundRequestFingerprint(req, reason, operatorID, operatorRole)
	if err != nil {
		return nil, err
	}

	var current model.Order
	var reservation model.RefundRecord
	replayed := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var locked model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Store").Preload("Merchant").First(&locked, order.ID).Error; err != nil {
			return err
		}
		if locked.OrderType != "store_order" {
			return errors.New("only store orders can be refunded here")
		}

		if idempotencyKey != "" {
			var existing model.RefundRecord
			lookupErr := tx.Where("order_id = ? AND idempotency_key = ?", locked.ID, idempotencyKey).First(&existing).Error
			if lookupErr == nil {
				if existing.RequestFingerprint != fingerprint {
					return errors.New("refund idempotency key was already used with a different request")
				}
				current = locked
				switch existing.Status {
				case "success":
					replayed = true
					return nil
				case "processing", "unknown":
					return fmt.Errorf("refund request %s is %s and must be reconciled; it will not be dispatched again", existing.RefundNo, existing.Status)
				case "failed":
					return fmt.Errorf("refund request %s was definitively rejected; use a new idempotency key to retry", existing.RefundNo)
				default:
					return fmt.Errorf("refund request %s has unsupported status %q", existing.RefundNo, existing.Status)
				}
			}
			if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				return lookupErr
			}
		}

		var unknownCount int64
		if err := tx.Model(&model.RefundRecord{}).Where("order_id = ? AND status = ?", locked.ID, "unknown").Count(&unknownCount).Error; err != nil {
			return err
		}
		if unknownCount > 0 {
			return errors.New("unknown refund reservation must be reconciled before any further refund")
		}

		if !orderHasConfirmedPaidAmount(locked) {
			return errors.New("only confirmed paid orders can be refunded")
		}
		if locked.Status != "received" && locked.Status != "accepted" && locked.Status != "completed" && locked.Status != "closed" {
			return errors.New("order status does not allow refund")
		}
		var totals refundReservationTotals
		if err := tx.Model(&model.RefundRecord{}).
			Where("order_id = ?", locked.ID).
			Select("COALESCE(SUM(CASE WHEN status = 'success' THEN amount ELSE 0 END), 0) AS successful_amount, COALESCE(SUM(CASE WHEN status IN ('processing', 'unknown') THEN amount ELSE 0 END), 0) AS reserved_amount").
			Scan(&totals).Error; err != nil {
			return err
		}
		committed := locked.RefundedAmount
		if totals.SuccessfulAmount > committed {
			committed = totals.SuccessfulAmount
		}
		remaining := locked.TotalAmount - committed - totals.ReservedAmount
		amount, err := resolveRefundAmountAgainstRemaining(locked, req, remaining)
		if err != nil {
			return err
		}
		if locked.PaymentChannel != "alipay" || locked.TransactionNo == "" || s.alipay == nil || !s.alipay.Enabled() {
			return errors.New("manual QR refunds cannot be recorded as successful without a verified refund channel")
		}
		refundNo, err := newStableRefundNo()
		if err != nil {
			return err
		}

		reservation = model.RefundRecord{
			OrderID:            locked.ID,
			MerchantID:         locked.MerchantID,
			StoreID:            locked.StoreID,
			RefundNo:           refundNo,
			TransactionNo:      locked.TransactionNo,
			Amount:             amount,
			Reason:             reason,
			OperatorRole:       operatorRole,
			OperatorID:         operatorID,
			Status:             "processing",
			RequestFingerprint: fingerprint,
		}
		if idempotencyKey != "" {
			reservation.IdempotencyKey = &idempotencyKey
		}
		if err := tx.Create(&reservation).Error; err != nil {
			return err
		}
		locked.RefundStatus = "processing"
		locked.OperationLogs = s.appendOrderLog(locked.OperationLogs, "refund_processing", fmt.Sprintf("已预占退款额度，等待支付渠道确认：%s 元，退款请求号：%s", formatFen(amount), refundNo))
		if err := tx.Save(&locked).Error; err != nil {
			return err
		}
		if len(audit) > 0 {
			event := audit[0]
			event.Input.TargetType = "order"
			event.Input.TargetID = locked.ID
			event.Input.TargetName = locked.OrderNo
			event.Input.MerchantID = locked.MerchantID
			if event.Input.Detail == nil {
				event.Input.Detail = map[string]interface{}{}
			}
			event.Input.Detail["amount"] = amount
			event.Input.Detail["full_refund"] = req.Full
			event.Input.Detail["reason"] = reason
			event.Input.Detail["refund_no"] = refundNo
			event.Input.Detail["status"] = "processing"
			event.Input.Detail["phase"] = "provider_dispatch_intent"
			if err := appendOptionalAuditTx(tx, []AuditEvent{event}); err != nil {
				return fmt.Errorf("persist refund dispatch audit intent: %w", err)
			}
		}
		current = locked
		return nil
	})
	if err != nil {
		return nil, err
	}
	if replayed {
		return &current, nil
	}

	rawPayload, providerErr := s.alipay.Refund(&current, reservation.RefundNo, reservation.Amount, reservation.Reason)
	if providerErr != nil {
		var definitiveFailure *definitiveRefundFailureError
		if errors.As(providerErr, &definitiveFailure) {
			_, persistErr := s.finishRefundReservation(reservation.ID, "failed", providerErr.Error())
			if persistErr != nil {
				return nil, fmt.Errorf("Alipay definitively rejected refund %s, but the local reservation could not be released: %w", reservation.RefundNo, persistErr)
			}
			return nil, fmt.Errorf("refund request %s was definitively rejected; use a new idempotency key to retry: %w", reservation.RefundNo, providerErr)
		}

		_, persistErr := s.finishRefundReservation(reservation.ID, "unknown", providerErr.Error())
		if persistErr != nil {
			return nil, fmt.Errorf("refund outcome is unknown and reservation %s remains held; recording the unknown result also failed: %v (database: %w)", reservation.RefundNo, providerErr, persistErr)
		}
		return nil, fmt.Errorf("refund outcome is unknown; reservation %s remains held for reconciliation and will not be retried automatically: %w", reservation.RefundNo, providerErr)
	}

	updated, err := s.finishRefundReservation(reservation.ID, "success", rawPayload)
	if err != nil {
		return nil, fmt.Errorf("Alipay reported refund %s successful, but local finalization failed; the durable reservation remains for reconciliation: %w", reservation.RefundNo, err)
	}
	return updated, nil
}

func refundRequestFingerprint(req dto.RefundOrderRequest, reason string, operatorID uint, operatorRole string) (string, error) {
	payload, err := json.Marshal(refundFingerprintInput{
		Amount:       req.Amount,
		Full:         req.Full,
		Reason:       reason,
		OperatorRole: operatorRole,
		OperatorID:   operatorID,
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func newStableRefundNo() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate refund request number: %w", err)
	}
	return "RF" + hex.EncodeToString(random[:]), nil
}

func (s *OrderService) finishRefundReservation(refundID uint, outcome, rawPayload string) (*model.Order, error) {
	var updated model.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var refund model.RefundRecord
		if err := tx.First(&refund, refundID).Error; err != nil {
			return err
		}
		var current model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, refund.OrderID).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&refund, refundID).Error; err != nil {
			return err
		}
		if refund.Status == "success" && outcome == "success" {
			updated = current
			return nil
		}
		if refund.Status != "processing" {
			return fmt.Errorf("refund reservation %s is %q, not processing", refund.RefundNo, refund.Status)
		}
		if outcome != "success" && outcome != "failed" && outcome != "unknown" {
			return fmt.Errorf("unsupported refund outcome %q", outcome)
		}

		refund.Status = outcome
		refund.RawPayload = rawPayload
		if err := tx.Save(&refund).Error; err != nil {
			return err
		}
		if outcome == "success" {
			current.RefundedAmount += refund.Amount
			current.OperationLogs = s.appendOrderLog(current.OperationLogs, "refund", fmt.Sprintf("%s退款成功：%s 元，原因：%s，退款请求号：%s", refund.OperatorRole, formatFen(refund.Amount), refund.Reason, refund.RefundNo))
		} else if outcome == "failed" {
			current.OperationLogs = s.appendOrderLog(current.OperationLogs, "refund_rejected", fmt.Sprintf("支付渠道明确拒绝退款：%s，退款请求号：%s；额度已释放，使用新幂等键才能重新发起", rawPayload, refund.RefundNo))
		} else {
			current.OperationLogs = s.appendOrderLog(current.OperationLogs, "refund_unknown", fmt.Sprintf("退款结果暂不确定，额度继续预占并待人工/渠道核对：%s，退款请求号：%s", rawPayload, refund.RefundNo))
		}
		var successfulAmount int64
		if err := tx.Model(&model.RefundRecord{}).
			Where("order_id = ? AND status = ?", current.ID, "success").
			Select("COALESCE(SUM(amount), 0)").
			Scan(&successfulAmount).Error; err != nil {
			return err
		}
		if successfulAmount > current.RefundedAmount {
			current.RefundedAmount = successfulAmount
		}

		var reservedCount int64
		if err := tx.Model(&model.RefundRecord{}).
			Where("order_id = ? AND status IN ?", current.ID, []string{"processing", "unknown"}).
			Count(&reservedCount).Error; err != nil {
			return err
		}
		if reservedCount > 0 {
			current.RefundStatus = "processing"
		} else if current.RefundedAmount >= current.TotalAmount {
			current.RefundStatus = "full"
		} else if current.RefundedAmount > 0 {
			current.RefundStatus = "partial"
		} else {
			current.RefundStatus = "none"
		}
		if err := tx.Save(&current).Error; err != nil {
			return err
		}
		updated = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}
