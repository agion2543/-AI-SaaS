package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const RefundReservationStaleAfter = 15 * time.Minute

type RefundReconciliationItem struct {
	RefundID          uint      `json:"refund_id"`
	RefundNo          string    `json:"refund_no"`
	OrderID           uint      `json:"order_id"`
	OrderNo           string    `json:"order_no"`
	Amount            int64     `json:"amount"`
	IdempotencyKey    string    `json:"idempotency_key"`
	Reason            string    `json:"reason"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	IsStale           bool      `json:"is_stale"`
	StaleAfterMinutes int       `json:"stale_after_minutes"`
	UnknownReason     string    `json:"unknown_reason"`
	ReservationStatus string    `json:"reservation_status"`
	OrderRefundStatus string    `json:"order_refund_status"`
	MerchantName      string    `json:"merchant_name"`
	StoreName         string    `json:"store_name"`
}

type RefundReconciliationDetail struct {
	RefundReconciliationItem
	Audit []model.RefundReconciliationAudit `json:"audit"`
}

func (s *OrderService) UnknownRefundReservations() ([]RefundReconciliationItem, error) {
	var refunds []model.RefundRecord
	if err := s.db.Preload("Order.Merchant").Preload("Order.Store").
		Where("status IN ?", []string{"unknown", "processing"}).Order("updated_at ASC, id ASC").Find(&refunds).Error; err != nil {
		return nil, err
	}
	items := make([]RefundReconciliationItem, 0, len(refunds))
	cutoff := time.Now().UTC().Add(-RefundReservationStaleAfter)
	for _, refund := range refunds {
		item := refundReconciliationItem(refund)
		if refund.Status == "processing" {
			item.IsStale = !item.UpdatedAt.IsZero() && !item.UpdatedAt.After(cutoff)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *OrderService) RefundReconciliationDetail(refundID uint) (*RefundReconciliationDetail, error) {
	var refund model.RefundRecord
	if err := s.db.Preload("Order.Merchant").Preload("Order.Store").First(&refund, refundID).Error; err != nil {
		return nil, err
	}
	var audits []model.RefundReconciliationAudit
	if err := s.db.Where("refund_id = ?", refundID).Order("created_at ASC, id ASC").Find(&audits).Error; err != nil {
		return nil, err
	}
	return &RefundReconciliationDetail{RefundReconciliationItem: refundReconciliationItem(refund), Audit: audits}, nil
}

func refundReconciliationItem(refund model.RefundRecord) RefundReconciliationItem {
	updatedAt := refund.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = refund.CreatedAt
	}
	item := RefundReconciliationItem{
		RefundID:          refund.ID,
		RefundNo:          refund.RefundNo,
		OrderID:           refund.OrderID,
		Amount:            refund.Amount,
		Reason:            refund.Reason,
		CreatedAt:         refund.CreatedAt,
		UpdatedAt:         updatedAt,
		IsStale:           refund.Status == "processing" && !updatedAt.IsZero() && !updatedAt.After(time.Now().UTC().Add(-RefundReservationStaleAfter)),
		StaleAfterMinutes: int(RefundReservationStaleAfter / time.Minute),
		UnknownReason:     strings.TrimSpace(refund.RawPayload),
		ReservationStatus: refund.Status,
	}
	if refund.IdempotencyKey != nil {
		item.IdempotencyKey = *refund.IdempotencyKey
	}
	if refund.Order.ID != 0 {
		item.OrderNo = refund.Order.OrderNo
		item.OrderRefundStatus = refund.Order.RefundStatus
		if refund.Order.Merchant != nil {
			item.MerchantName = refund.Order.Merchant.Name
		}
		if refund.Order.Store != nil {
			item.StoreName = refund.Order.Store.Name
		}
	}
	return item
}

// MarkStaleProcessingRefundUnknown moves an overdue reservation to the existing
// manual reconciliation state. It deliberately preserves the reserved amount and
// never retries or queries the payment provider.
func (s *OrderService) MarkStaleProcessingRefundUnknown(refundID uint, reason string, actorID uint, actorRole string) (*RefundReconciliationDetail, error) {
	if actorRole != "admin" && actorRole != "super_admin" {
		return nil, errors.New("only platform administrators may move stale refunds to manual review")
	}
	if actorID == 0 {
		return nil, errors.New("administrator identity is required for reconciliation audit")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("manual review note/reason is required")
	}
	if len([]rune(reason)) > 1000 {
		return nil, errors.New("manual review note cannot exceed 1000 characters")
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Keep the same order-then-refund lock order as refund finalization and
		// outcome reconciliation. A processing reservation cannot be force-moved
		// until its persisted updated_at has aged past the documented threshold.
		var seed model.RefundRecord
		if err := tx.First(&seed, refundID).Error; err != nil {
			return err
		}
		var order model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, seed.OrderID).Error; err != nil {
			return err
		}
		var refund model.RefundRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&refund, refundID).Error; err != nil {
			return err
		}

		if refund.Status != "processing" {
			if refund.Status == "unknown" || refund.Status == "success" || refund.Status == "failed" {
				var prior model.RefundReconciliationAudit
				err := tx.Where("refund_id = ? AND old_status = ? AND new_status = ? AND actor_id = ? AND actor_role = ? AND reason = ?",
					refund.ID, "processing", "unknown", actorID, actorRole, reason).First(&prior).Error
				if err == nil {
					return nil // exact retry after a lost response; no additional audit or balance change
				}
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
			return fmt.Errorf("refund reservation %s is %q and cannot be moved to manual review", refund.RefundNo, refund.Status)
		}

		cutoff := time.Now().UTC().Add(-RefundReservationStaleAfter)
		updatedAt := refund.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = refund.CreatedAt
		}
		if updatedAt.IsZero() || updatedAt.After(cutoff) {
			return fmt.Errorf("refund reservation is not overdue; processing must last at least %d minutes before manual review", int(RefundReservationStaleAfter/time.Minute))
		}

		transition := tx.Model(&model.RefundRecord{}).Where("id = ? AND status = ?", refund.ID, "processing")
		if refund.UpdatedAt.IsZero() {
			transition = transition.Where("(updated_at IS NULL OR updated_at = ?)", refund.UpdatedAt)
		} else {
			transition = transition.Where("updated_at = ?", refund.UpdatedAt)
		}
		transition = transition.Update("status", "unknown")
		if transition.Error != nil {
			return transition.Error
		}
		if transition.RowsAffected != 1 {
			return errors.New("refund reservation changed concurrently; reload reconciliation detail")
		}

		// Do not touch the order's refunded amount or refund status: this row still
		// reserves the same amount, and unknown remains part of the reservation sum.
		audit := model.RefundReconciliationAudit{
			RefundID:  refund.ID,
			RefundNo:  refund.RefundNo,
			OrderID:   order.ID,
			ActorID:   actorID,
			ActorRole: actorRole,
			OldStatus: "processing",
			NewStatus: "unknown",
			Amount:    refund.Amount,
			Reason:    reason,
			CreatedAt: time.Now().UTC(),
		}
		return tx.Create(&audit).Error
	})
	if err != nil {
		return nil, err
	}
	return s.RefundReconciliationDetail(refundID)
}

// ResolveUnknownRefund records a human decision backed by separate provider-side
// evidence. It does not call or claim to verify a provider API.
func (s *OrderService) ResolveUnknownRefund(refundID uint, req dto.RefundReconciliationRequest, actorID uint, actorRole string) (*RefundReconciliationDetail, error) {
	if actorRole != "admin" && actorRole != "super_admin" {
		return nil, errors.New("only platform administrators may reconcile refunds")
	}
	if actorID == 0 {
		return nil, errors.New("administrator identity is required for reconciliation audit")
	}
	if req.Outcome != "success" && req.Outcome != "failed" {
		return nil, errors.New("outcome must be success or failed")
	}
	reference := strings.TrimSpace(req.ProviderReference)
	reason := strings.TrimSpace(req.Reason)
	if reference == "" {
		return nil, errors.New("provider-side refund/reference number is required")
	}
	if len([]rune(reference)) > 120 {
		return nil, errors.New("provider reference cannot exceed 120 characters")
	}
	if reason == "" {
		return nil, errors.New("reconciliation note/reason is required")
	}
	if len([]rune(reason)) > 1000 {
		return nil, errors.New("reconciliation note cannot exceed 1000 characters")
	}
	req.ProviderReference = reference
	req.Reason = reason

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Read the foreign key first, then lock order followed by refund, matching
		// the existing refund finalizer's lock order to avoid cross-path deadlocks.
		var seed model.RefundRecord
		if err := tx.First(&seed, refundID).Error; err != nil {
			return err
		}
		var order model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, seed.OrderID).Error; err != nil {
			return err
		}
		var refund model.RefundRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&refund, refundID).Error; err != nil {
			return err
		}

		if refund.Status != "unknown" {
			if refund.Status == req.Outcome {
				var prior model.RefundReconciliationAudit
				err := tx.Where(
					"refund_id = ? AND old_status = ? AND new_status = ? AND actor_id = ? AND actor_role = ? AND provider_reference = ? AND reason = ?",
					refund.ID, "unknown", req.Outcome, actorID, actorRole, reference, reason,
				).First(&prior).Error
				if err == nil {
					return nil // exact retry after a lost response; no second balance change or audit row
				}
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
			return fmt.Errorf("refund reservation %s is %q and cannot be resolved as %q", refund.RefundNo, refund.Status, req.Outcome)
		}

		committed := order.RefundedAmount
		var successfulAmount int64
		if err := tx.Model(&model.RefundRecord{}).Where("order_id = ? AND status = ?", order.ID, "success").
			Select("COALESCE(SUM(amount), 0)").Scan(&successfulAmount).Error; err != nil {
			return err
		}
		if successfulAmount > committed {
			committed = successfulAmount
		}
		if req.Outcome == "success" && (refund.Amount <= 0 || committed > order.TotalAmount || refund.Amount > order.TotalAmount-committed) {
			return errors.New("recorded refund would exceed the remaining paid order amount; reservation remains unknown")
		}

		transition := tx.Model(&model.RefundRecord{}).
			Where("id = ? AND status = ?", refund.ID, "unknown").
			Update("status", req.Outcome)
		if transition.Error != nil {
			return transition.Error
		}
		if transition.RowsAffected != 1 {
			return errors.New("refund reservation changed concurrently; reload reconciliation detail")
		}

		if req.Outcome == "success" {
			order.RefundedAmount = committed + refund.Amount
		}
		if err := tx.Model(&model.RefundRecord{}).
			Where("order_id = ? AND status = ?", order.ID, "success").
			Select("COALESCE(SUM(amount), 0)").Scan(&successfulAmount).Error; err != nil {
			return err
		}
		if successfulAmount > order.RefundedAmount {
			order.RefundedAmount = successfulAmount
		}

		var reservedCount int64
		if err := tx.Model(&model.RefundRecord{}).
			Where("order_id = ? AND status IN ?", order.ID, []string{"processing", "unknown"}).
			Count(&reservedCount).Error; err != nil {
			return err
		}
		if reservedCount > 0 {
			order.RefundStatus = "processing"
		} else if order.RefundedAmount >= order.TotalAmount {
			order.RefundStatus = "full"
		} else if order.RefundedAmount > 0 {
			order.RefundStatus = "partial"
		} else {
			order.RefundStatus = "none"
		}

		if req.Outcome == "success" {
			order.OperationLogs = s.appendOrderLog(order.OperationLogs, "refund_reconciled_success", "平台管理员基于外部凭证记录退款成功；凭证详情仅保存在平台对账审计中")
		} else {
			order.OperationLogs = s.appendOrderLog(order.OperationLogs, "refund_reconciled_failed", "平台管理员基于外部凭证记录退款确定失败；该退款预留额度已释放")
		}
		updatedOrder := tx.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
			"refunded_amount": order.RefundedAmount,
			"refund_status":   order.RefundStatus,
			"operation_logs":  order.OperationLogs,
		})
		if updatedOrder.Error != nil {
			return updatedOrder.Error
		}
		if updatedOrder.RowsAffected != 1 {
			return errors.New("order was not updated during refund reconciliation")
		}

		audit := model.RefundReconciliationAudit{
			RefundID:          refund.ID,
			RefundNo:          refund.RefundNo,
			OrderID:           order.ID,
			ActorID:           actorID,
			ActorRole:         actorRole,
			OldStatus:         "unknown",
			NewStatus:         req.Outcome,
			Amount:            refund.Amount,
			ProviderReference: reference,
			Reason:            reason,
			CreatedAt:         time.Now().UTC(),
		}
		return tx.Create(&audit).Error
	})
	if err != nil {
		return nil, err
	}
	return s.RefundReconciliationDetail(refundID)
}
