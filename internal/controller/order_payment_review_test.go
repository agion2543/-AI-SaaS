package controller

import (
	"testing"
	"time"

	"go-web-gin-health/internal/model"
)

func TestLatePaymentReviewStateIsExplicitInCustomerAndAdminOrderResponses(t *testing.T) {
	paidAt := time.Now()
	order := model.Order{
		OrderNo:        "SYNTHETIC-LATE-PAYMENT-REVIEW",
		Status:         "closed",
		TotalAmount:    1234,
		PaymentChannel: "alipay",
		PaidAt:         &paidAt,
		TransactionNo:  "synthetic-late-trade",
		OperationLogs:  `[{"action":"late_payment_exception","text":"synthetic late payment"}]`,
	}

	if len(model.PaymentStatusPaidReviewRequired) > 20 {
		t.Fatalf("payment review status %q exceeds existing VARCHAR(20)", model.PaymentStatusPaidReviewRequired)
	}
	if !orderRequiresManualPaymentReview(&order) {
		t.Fatal("late-payment audit action must produce an explicit manual-review flag")
	}

	public := serializePublicCustomerOrder(&order)
	if public["status"] != "closed" || public["payment_review_required"] != true || public["payment_review_status"] != model.PaymentStatusPaidReviewRequired {
		t.Fatalf("customer order response must distinguish paid-review from normal closed/unpaid: status=%v review_required=%v review_status=%v", public["status"], public["payment_review_required"], public["payment_review_status"])
	}

	admin := serializeAdminOrder(&order, false)
	if admin["status"] != "closed" || admin["payment_review_required"] != true || admin["payment_review_status"] != model.PaymentStatusPaidReviewRequired {
		t.Fatalf("admin order response must expose paid-review status alongside closed lifecycle: status=%v review_required=%v review_status=%v", admin["status"], admin["payment_review_required"], admin["payment_review_status"])
	}

	finance := serializeAdminOrderFinanceSummary(order)
	if finance["payment_review_required"] != true || finance["payment_review_status"] != model.PaymentStatusPaidReviewRequired {
		t.Fatalf("admin payment finance summary must expose manual review status: review_required=%v review_status=%v", finance["payment_review_required"], finance["payment_review_status"])
	}
}

func TestNormalClosedOrderIsNotMarkedForPaymentReview(t *testing.T) {
	order := &model.Order{Status: "closed", OperationLogs: `[{"action":"checkout_expired"}]`}
	if orderRequiresManualPaymentReview(order) || paymentReviewStatus(order) != "" {
		t.Fatalf("ordinary expired unpaid order was incorrectly marked as paid-review-required: status=%q", paymentReviewStatus(order))
	}
}

func TestExpiredUnpaidPlatformQRSubscriptionIsExplicitInAdminResponses(t *testing.T) {
	order := &model.Order{
		OrderNo:        "SYNTHETIC-EXPIRED-UNPAID-STATIC-QR",
		OrderType:      "merchant_subscription",
		Status:         "closed",
		PaymentChannel: "platform_qr",
		OperationLogs:  `[{"action":"checkout_expired","text":"synthetic expiry"}]`,
	}
	admin := serializeAdminOrder(order, false)
	if admin["status"] != "closed" || order.PaidAt != nil || admin["expired_unpaid"] != true || admin["payment_review_required"] != false {
		t.Fatalf("admin order must expose a closed but unpaid static QR checkout: status=%v paid_at=%v expired_unpaid=%v payment_review_required=%v", admin["status"], admin["paid_at"], admin["expired_unpaid"], admin["payment_review_required"])
	}

	finance := serializeAdminOrderFinanceSummary(*order)
	if finance["expired_unpaid"] != true || order.PaidAt != nil {
		t.Fatalf("admin finance summary must expose the expired unpaid state: expired_unpaid=%v paid_at=%v", finance["expired_unpaid"], finance["paid_at"])
	}
}
