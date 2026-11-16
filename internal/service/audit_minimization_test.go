package service

import (
	"strings"
	"testing"

	"go-web-gin-health/internal/model"
)

func TestAppendAuditTxStoresOnlyActorObjectActionTimeAndPurpose(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic audit table: %v", err)
	}
	merchantID := uint(27)
	event := AuditEvent{
		Actor: AuditActor{ID: 42, Type: "merchant_admin", IP: "192.0.2.44", UserAgent: "synthetic-agent"},
		Input: AuditRecordInput{
			Action:     "merchant_order_refund",
			TargetType: "order",
			TargetID:   81,
			TargetName: "synthetic customer 13800138000",
			MerchantID: &merchantID,
			Detail:     map[string]interface{}{"reason": "synthetic customer note", "phone": "13800138000"},
		},
	}
	if err := AppendAuditTx(db, event); err != nil {
		t.Fatalf("append synthetic audit event: %v", err)
	}
	var log model.AuditLog
	if err := db.First(&log).Error; err != nil {
		t.Fatalf("load synthetic audit event: %v", err)
	}
	if log.ActorID != 42 || log.ActorType != "merchant_admin" || log.Action != "merchant_order_refund" || log.TargetType != "order" || log.TargetID != 81 || log.MerchantID == nil || *log.MerchantID != merchantID || log.CreatedAt.IsZero() {
		t.Fatalf("audit identifiers/time not preserved: %+v", log)
	}
	if log.ActorName != "" || log.TargetName != "" || log.IP != "" || log.UserAgent != "" {
		t.Errorf("audit must not store names, IP, or user-agent: %+v", log)
	}
	if log.Detail != `{"purpose":"商家退款处理"}` || strings.Contains(log.Detail, "13800138000") || strings.Contains(log.Detail, "customer note") || strings.Contains(log.Detail, "reason") {
		t.Errorf("audit must store only a purpose, got %s", log.Detail)
	}
}
