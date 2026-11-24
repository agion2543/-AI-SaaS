package service

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/model"
)

const checkoutOrderExpiryDuration = 30 * time.Minute

// CheckoutOrderExpireTime derives the customer-facing payment deadline from
// existing order fields; non-checkout, paid, and zero-amount orders have none.
func CheckoutOrderExpireTime(order *model.Order) *time.Time {
	if order == nil || order.TotalAmount <= 0 || order.PaidAt != nil || order.CreatedAt.IsZero() {
		return nil
	}
	if order.Status != "pending" && order.Status != "failed" {
		return nil
	}
	expiresAt := order.CreatedAt.Add(checkoutOrderExpiryDuration)
	return &expiresAt
}

func isUnpaidCheckoutExpired(order model.Order, now time.Time) bool {
	expiresAt := CheckoutOrderExpireTime(&order)
	return expiresAt != nil && !now.Before(*expiresAt)
}

func (s *OrderService) expireCheckoutOrderIfDue(orderID uint, now time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, orderID).Error; err != nil {
			return err
		}
		if !isUnpaidCheckoutExpired(order, now) {
			return nil
		}
		order.Status = "closed"
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "checkout_expired", "未付款订单超过30分钟有效期，系统已关闭")
		result := tx.Model(&model.Order{}).
			Where("id = ? AND total_amount > ? AND paid_at IS NULL AND status IN ? AND created_at <= ?",
				order.ID, 0, []string{"pending", "failed"}, now.Add(-checkoutOrderExpiryDuration)).
			Updates(map[string]interface{}{"status": order.Status, "operation_logs": order.OperationLogs})
		return result.Error
	})
}

// ExpireUnpaidCheckoutOrders closes unpaid checkout orders whose creation-based
// 30-minute payment window has elapsed. Expiration never records payment or
// grants benefits; static QR receipts remain manual-only.
func (s *OrderService) ExpireUnpaidCheckoutOrders(now time.Time) (int64, error) {
	cutoff := now.Add(-checkoutOrderExpiryDuration)
	var expired int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var orders []model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("total_amount > ? AND paid_at IS NULL AND status IN ? AND created_at <= ?",
				0, []string{"pending", "failed"}, cutoff).
			Order("id ASC").
			Find(&orders).Error; err != nil {
			return err
		}
		for _, order := range orders {
			order.Status = "closed"
			order.OperationLogs = s.appendOrderLog(order.OperationLogs, "checkout_expired", "未付款订单超过30分钟有效期，系统已关闭")
			result := tx.Model(&model.Order{}).
				Where("id = ? AND total_amount > ? AND paid_at IS NULL AND status IN ? AND created_at <= ?",
					order.ID, 0, []string{"pending", "failed"}, cutoff).
				Updates(map[string]interface{}{"status": order.Status, "operation_logs": order.OperationLogs})
			if result.Error != nil {
				return result.Error
			}
			expired += result.RowsAffected
		}
		return nil
	})
	return expired, err
}

// RunCheckoutOrderExpiryWorker performs an initial sweep and then periodically
// closes expired checkouts. Callback handling also checks the deadline inline,
// so correctness does not depend on the sweep interval.
func (s *OrderService) RunCheckoutOrderExpiryWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runSweep := func() {
		count, err := s.ExpireUnpaidCheckoutOrders(time.Now())
		if err != nil {
			log.Printf("checkout order expiry sweep failed: %v", err)
			return
		}
		if count > 0 {
			log.Printf("closed %d expired unpaid checkout orders", count)
		}
	}
	runSweep()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runSweep()
		}
	}
}
