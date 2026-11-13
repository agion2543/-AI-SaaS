package service

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const manualReviewSubscriptionPlanMarker = "历史复核"

// ManualSubscriptionReviewActor is supplied only from authenticated platform
// administrator context by the controller.
type ManualSubscriptionReviewActor struct {
	ID   uint
	Role string
}

type ManualSubscriptionReviewResult struct {
	Order      *model.Order
	Review     *model.MerchantSubscriptionManualReview
	Audit      *model.MerchantSubscriptionManualReviewAudit
	Idempotent bool
}

// ReviewLegacyMerchantSubscriptionOrder records an administrator's assertion
// that original evidence was checked for a legacy order with no plan snapshot.
// It never calls a provider, creates a PaymentRecord, or sets PaidAt/TransactionNo.
func (s *OrderService) ReviewLegacyMerchantSubscriptionOrder(
	orderID uint,
	req dto.ManualReviewLegacyMerchantSubscriptionRequest,
	actor ManualSubscriptionReviewActor,
) (*ManualSubscriptionReviewResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("order database is unavailable")
	}
	if orderID == 0 {
		return nil, errors.New("subscription order ID is required")
	}
	if actor.ID == 0 || (actor.Role != "admin" && actor.Role != "super_admin") {
		return nil, errors.New("platform administrator is required")
	}
	terms, err := normalizeManualReviewTerms(req)
	if err != nil {
		return nil, err
	}
	fingerprint, err := manualReviewRequestFingerprint(terms)
	if err != nil {
		return nil, err
	}

	result := &ManualSubscriptionReviewResult{}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}

		var existing model.MerchantSubscriptionManualReview
		err := tx.Where("order_id = ?", order.ID).First(&existing).Error
		if err == nil {
			if existing.RequestFingerprint != fingerprint {
				return errors.New("this subscription order already has a different immutable manual review")
			}
			var audit model.MerchantSubscriptionManualReviewAudit
			if err := tx.Where("review_id = ?", existing.ID).First(&audit).Error; err != nil {
				return fmt.Errorf("manual review audit is missing or unreadable: %w", err)
			}
			if order.Status != "manually_verified" || order.PaidAt != nil || strings.TrimSpace(order.TransactionNo) != "" {
				return errors.New("saved manual review conflicts with current order state")
			}
			if err := tx.Preload("Merchant").Where("id = ?", order.ID).First(&order).Error; err != nil {
				return err
			}
			result.Order = &order
			result.Review = &existing
			result.Audit = &audit
			result.Idempotent = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if order.OrderType != "merchant_subscription" {
			return errors.New("only merchant subscription orders can use historical manual review")
		}
		if order.Status != "pending" {
			return errors.New("only pending subscription orders can use historical manual review")
		}
		if order.MerchantID == nil || *order.MerchantID == 0 {
			return errors.New("subscription order has no merchant")
		}
		if order.PaidAt != nil || strings.TrimSpace(order.TransactionNo) != "" {
			return errors.New("subscription order already contains payment markers")
		}
		if order.PlanNameSnapshot != nil || order.PlanPriceCentsSnapshot != nil || order.PlanDurationDaysSnapshot != nil {
			return errors.New("manual review is only available when all plan snapshots are entirely absent")
		}
		if order.Amount <= 0 || order.TotalAmount <= 0 || order.Amount != order.TotalAmount {
			return errors.New("stored order amount and total amount must be positive and equal")
		}
		if terms.AmountCents != order.Amount || terms.AmountCents != order.TotalAmount {
			return errors.New("reviewed amount must exactly match both stored order amounts")
		}
		var paymentCount int64
		if err := tx.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
			return fmt.Errorf("check existing payment records: %w", err)
		}
		if paymentCount != 0 {
			return errors.New("subscription order already has payment records and is not eligible for manual review")
		}

		var merchant model.Merchant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", *order.MerchantID).First(&merchant).Error; err != nil {
			return fmt.Errorf("load and lock subscription merchant: %w", err)
		}
		reviewedAt := time.Now().UTC().Truncate(time.Millisecond)
		beforeExpiry := effectiveMerchantSubscriptionExpiry(&merchant)
		base := reviewedAt
		if beforeExpiry != nil && beforeExpiry.After(base) {
			base = *beforeExpiry
		}
		maxExpiry := time.Date(9999, 12, 30, 23, 59, 59, 0, time.UTC)
		maxDays := int64(maxExpiry.Sub(base) / (24 * time.Hour))
		if terms.DurationDays > maxDays || int64(int(terms.DurationDays)) != terms.DurationDays {
			return errors.New("reviewed duration exceeds the supported subscription expiry range")
		}
		expiry := base.AddDate(0, 0, int(terms.DurationDays))
		if expiry.Year() > 9999 {
			return errors.New("reviewed duration exceeds the supported subscription expiry range")
		}

		beforeOrderStatus := order.Status
		beforeOrderUpdatedAt := order.UpdatedAt
		beforePaidAt := cloneTime(order.PaidAt)
		beforeSubscriptionStatus := merchant.SubscriptionStatus
		beforeSubscriptionPlan := merchant.SubscriptionPlan
		order.Status = "manually_verified"
		order.PaymentChannel = "admin_manual_verified"
		order.SubscriptionEndAt = &expiry
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "subscription_historical_manually_verified", "管理员已核对原始证据并声明历史外部付款已实际发生；系统未调用支付渠道且未生成渠道支付记录")
		order.UpdatedAt = reviewedAt
		updates := map[string]interface{}{
			"status":              order.Status,
			"payment_channel":     order.PaymentChannel,
			"subscription_end_at": expiry,
			"operation_logs":      order.OperationLogs,
			"updated_at":          reviewedAt,
		}
		updateResult := tx.Model(&model.Order{}).Where("id = ? AND status = ?", order.ID, "pending").Updates(updates)
		if updateResult.Error != nil {
			return fmt.Errorf("update manually reviewed subscription order: %w", updateResult.Error)
		}
		if updateResult.RowsAffected != 1 {
			return errors.New("subscription order changed during manual review")
		}

		merchant.SubscriptionPlanID = nil
		merchant.SubscriptionPlan = manualReviewSubscriptionPlanMarker
		merchant.SubscriptionStatus = "active"
		merchant.SubscriptionExpireAt = &expiry
		merchant.SubscriptionExpiredAt = &expiry
		merchant.SubscriptionNote = "管理员声明已核对历史证据并确认外部付款已实际发生；由人工核验流程授予权益，非支付渠道自动确认"
		merchant.UpdatedAt = reviewedAt
		merchantUpdates := map[string]interface{}{
			"subscription_plan_id":    nil,
			"subscription_plan":       merchant.SubscriptionPlan,
			"subscription_status":     merchant.SubscriptionStatus,
			"subscription_expire_at":  expiry,
			"subscription_expired_at": expiry,
			"subscription_note":       merchant.SubscriptionNote,
			"updated_at":              reviewedAt,
		}
		merchantUpdate := tx.Model(&model.Merchant{}).Where("id = ?", merchant.ID).Updates(merchantUpdates)
		if merchantUpdate.Error != nil {
			return fmt.Errorf("activate reviewed historical subscription: %w", merchantUpdate.Error)
		}
		if merchantUpdate.RowsAffected != 1 {
			return errors.New("subscription merchant changed during manual review")
		}

		review := model.MerchantSubscriptionManualReview{
			ReviewKey:               fmt.Sprintf("MSUB-MANUAL-REVIEW-%d", order.ID),
			OrderID:                 order.ID,
			OrderNo:                 order.OrderNo,
			MerchantID:              merchant.ID,
			ActorID:                 actor.ID,
			ActorRole:               actor.Role,
			PlanName:                terms.PlanName,
			AmountCents:             terms.AmountCents,
			DurationDays:            terms.DurationDays,
			PaymentOccurredAttested: terms.PaymentOccurredAttested,
			EvidenceReference:       terms.EvidenceReference,
			Reason:                  terms.Reason,
			RequestFingerprint:      fingerprint,
			ReviewStatus:            "manually_verified",
			CreatedAt:               reviewedAt,
			UpdatedAt:               reviewedAt,
		}
		if err := tx.Create(&review).Error; err != nil {
			return fmt.Errorf("persist immutable subscription manual review record: %w", err)
		}
		audit := model.MerchantSubscriptionManualReviewAudit{
			ReviewID:                   review.ID,
			OrderID:                    order.ID,
			MerchantID:                 merchant.ID,
			ActorID:                    actor.ID,
			ActorRole:                  actor.Role,
			EvidenceReference:          terms.EvidenceReference,
			Reason:                     terms.Reason,
			PlanName:                   terms.PlanName,
			AmountCents:                terms.AmountCents,
			DurationDays:               terms.DurationDays,
			PaymentOccurredAttested:    terms.PaymentOccurredAttested,
			OrderStatusBefore:          beforeOrderStatus,
			OrderStatusAfter:           order.Status,
			OrderPaidAtBefore:          beforePaidAt,
			OrderPaidAtAfter:           nil,
			OrderUpdatedAtBefore:       beforeOrderUpdatedAt,
			OrderUpdatedAtAfter:        reviewedAt,
			SubscriptionStatusBefore:   beforeSubscriptionStatus,
			SubscriptionStatusAfter:    merchant.SubscriptionStatus,
			SubscriptionPlanBefore:     beforeSubscriptionPlan,
			SubscriptionPlanAfter:      merchant.SubscriptionPlan,
			SubscriptionExpireAtBefore: beforeExpiry,
			SubscriptionExpireAtAfter:  expiry,
			ReviewedAt:                 reviewedAt,
			CreatedAt:                  reviewedAt,
		}
		if err := tx.Create(&audit).Error; err != nil {
			return fmt.Errorf("persist append-only subscription manual review audit: %w", err)
		}

		order.Merchant = &merchant
		result.Order = &order
		result.Review = &review
		result.Audit = &audit
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type normalizedManualReviewTerms struct {
	PlanName                string
	AmountCents             int64
	DurationDays            int64
	PaymentOccurredAttested bool
	EvidenceReference       string
	Reason                  string
}

func normalizeManualReviewTerms(req dto.ManualReviewLegacyMerchantSubscriptionRequest) (normalizedManualReviewTerms, error) {
	terms := normalizedManualReviewTerms{
		PlanName:                strings.TrimSpace(req.PlanName),
		AmountCents:             req.AmountCents,
		DurationDays:            req.DurationDays,
		PaymentOccurredAttested: req.PaymentOccurredAttested,
		EvidenceReference:       strings.TrimSpace(req.EvidenceReference),
		Reason:                  strings.TrimSpace(req.Reason),
	}
	if terms.PlanName == "" || utf8.RuneCountInString(terms.PlanName) > 80 {
		return terms, errors.New("original plan name is required and must be at most 80 characters")
	}
	if terms.AmountCents <= 0 {
		return terms, errors.New("reviewed amount must be greater than zero")
	}
	if terms.DurationDays <= 0 {
		return terms, errors.New("reviewed subscription duration must be a positive number of days")
	}
	if !terms.PaymentOccurredAttested {
		return terms, errors.New("platform administrator must attest that original external payment actually occurred")
	}
	if terms.EvidenceReference == "" || utf8.RuneCountInString(terms.EvidenceReference) > 255 {
		return terms, errors.New("original evidence source/reference is required and must be at most 255 characters")
	}
	if terms.Reason == "" || utf8.RuneCountInString(terms.Reason) > 1000 {
		return terms, errors.New("manual review reason is required and must be at most 1000 characters")
	}
	return terms, nil
}

func manualReviewRequestFingerprint(terms normalizedManualReviewTerms) (string, error) {
	canonical, err := json.Marshal(terms)
	if err != nil {
		return "", fmt.Errorf("encode manual review idempotency fingerprint: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return fmt.Sprintf("%x", sum), nil
}

func effectiveMerchantSubscriptionExpiry(merchant *model.Merchant) *time.Time {
	if merchant == nil {
		return nil
	}
	var expiry *time.Time
	if merchant.SubscriptionExpireAt != nil {
		expiry = merchant.SubscriptionExpireAt
	}
	if merchant.SubscriptionExpiredAt != nil && (expiry == nil || merchant.SubscriptionExpiredAt.After(*expiry)) {
		expiry = merchant.SubscriptionExpiredAt
	}
	return cloneTime(expiry)
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
