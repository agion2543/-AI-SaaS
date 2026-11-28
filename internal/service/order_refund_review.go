package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/model"
)

// PendingReviewRefunds lists above-threshold refunds waiting for a
// super-admin decision, oldest first.
func (s *OrderService) PendingReviewRefunds() ([]model.RefundRecord, error) {
	var refunds []model.RefundRecord
	if err := s.db.Preload("Order").Preload("Order.Merchant").Preload("Order.Store").
		Where("status = ?", "pending_review").Order("created_at ASC, id ASC").Find(&refunds).Error; err != nil {
		return nil, err
	}
	return refunds, nil
}

func isRefundReviewAdminRole(role string) bool {
	return role == "admin" || role == "super_admin"
}

// ReviewRefund decides a pending-review refund. Approving dispatches it to
// the provider; rejecting releases the reservation with a reason. The
// reviewer must be a super-admin (enforced at the route) and must not be
// the applicant: when the applicant was an admin, the reviewer has to be
// a different admin account. Admin-side roles ("admin" and "super_admin")
// compare as one family, so a super-admin cannot approve a refund another
// admin applied under the plain "admin" label either. Apply, review, and
// execute each carry an audit row.
func (s *OrderService) ReviewRefund(refundID uint, approve bool, reason string, reviewerID uint, reviewerRole string, audit ...AuditEvent) (*model.Order, error) {
	reason = strings.TrimSpace(reason)
	if !approve {
		if reason == "" {
			return nil, errors.New("review rejection requires a reason")
		}
		if len([]rune(reason)) > 255 {
			return nil, errors.New("review reason cannot exceed 255 characters")
		}
	}
	var current model.Order
	var reservation model.RefundRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&reservation, refundID).Error; err != nil {
			return errors.New("refund not found")
		}
		if reservation.Status != "pending_review" {
			return fmt.Errorf("refund request %s is %q, not pending review", reservation.RefundNo, reservation.Status)
		}
		if reviewerID == reservation.OperatorID && (reviewerRole == reservation.OperatorRole || (isRefundReviewAdminRole(reviewerRole) && isRefundReviewAdminRole(reservation.OperatorRole))) {
			return errors.New("the reviewer cannot be the refund applicant")
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Store").Preload("Merchant").First(&current, reservation.OrderID).Error; err != nil {
			return errors.New("order not found")
		}
		if approve && (s.alipay == nil || !s.alipay.Enabled()) {
			return errors.New("payment provider is unavailable; the reviewed refund cannot be dispatched")
		}
		now := time.Now()
		reservation.ReviewerID = reviewerID
		reservation.ReviewerRole = reviewerRole
		reservation.ReviewReason = reason
		reservation.ReviewedAt = &now
		if approve {
			reservation.Status = "processing"
			current.OperationLogs = s.appendOrderLog(current.OperationLogs, "refund_review_approved", fmt.Sprintf("超管复核通过，等待支付渠道执行退款：%s 元，退款请求号：%s", formatFen(reservation.Amount), reservation.RefundNo))
		} else {
			reservation.Status = "review_rejected"
			current.OperationLogs = s.appendOrderLog(current.OperationLogs, "refund_review_rejected", fmt.Sprintf("超管复核拒绝退款：%s 元，原因：%s，退款请求号：%s", formatFen(reservation.Amount), reason, reservation.RefundNo))
			// Release the reservation: recompute the order refund status
			// without it, mirroring finishRefundReservation.
			var successfulAmount int64
			if err := tx.Model(&model.RefundRecord{}).Where("order_id = ? AND status = ?", current.ID, "success").Select("COALESCE(SUM(amount), 0)").Scan(&successfulAmount).Error; err != nil {
				return err
			}
			if successfulAmount > current.RefundedAmount {
				current.RefundedAmount = successfulAmount
			}
			var reservedCount int64
			// Exclude this reservation: it is being rejected right now
			// and no longer reserves anything once the transaction
			// commits.
			if err := tx.Model(&model.RefundRecord{}).Where("order_id = ? AND id != ? AND status IN ?", current.ID, reservation.ID, []string{"processing", "unknown", "pending_review"}).Count(&reservedCount).Error; err != nil {
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
		}
		if err := tx.Save(&reservation).Error; err != nil {
			return err
		}
		if err := tx.Save(&current).Error; err != nil {
			return err
		}
		if len(audit) > 0 {
			event := audit[0]
			event.Input.TargetType = "order"
			event.Input.TargetID = current.ID
			event.Input.TargetName = current.OrderNo
			event.Input.MerchantID = current.MerchantID
			if event.Input.Detail == nil {
				event.Input.Detail = map[string]interface{}{}
			}
			event.Input.Detail["amount"] = reservation.Amount
			event.Input.Detail["refund_no"] = reservation.RefundNo
			event.Input.Detail["review_reason"] = reason
			event.Input.Detail["reviewer_id"] = reviewerID
			event.Input.Detail["reviewer_role"] = reviewerRole
			if approve {
				event.Input.Detail["phase"] = "review_approved"
				event.Input.Detail["status"] = "processing"
			} else {
				event.Input.Detail["phase"] = "review_rejected"
				event.Input.Detail["status"] = "review_rejected"
			}
			if err := appendOptionalAuditTx(tx, []AuditEvent{event}); err != nil {
				return fmt.Errorf("persist refund review audit: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !approve {
		return &current, nil
	}
	updated, _, dispatchErr := s.dispatchReservedRefund(&current, reservation)
	return updated, dispatchErr
}
