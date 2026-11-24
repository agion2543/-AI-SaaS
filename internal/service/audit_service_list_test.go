package service

import (
	"testing"
	"time"

	"go-web-gin-health/internal/model"
)

func TestAuditServiceListUsesIDAsStableTieBreakerAcrossPages(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic audit table: %v", err)
	}

	createdAt := time.Date(2026, time.November, 22, 10, 0, 0, 0, time.UTC)
	created := make([]model.AuditLog, 0, 5)
	for i := 0; i < cap(created); i++ {
		log := model.AuditLog{
			BaseModel: model.BaseModel{CreatedAt: createdAt, UpdatedAt: createdAt},
			Action:    "synthetic_audit_page_test",
			ActorName: "synthetic actor",
		}
		if err := db.Create(&log).Error; err != nil {
			t.Fatalf("create synthetic audit event %d: %v", i+1, err)
		}
		created = append(created, log)
	}

	wantIDs := make([]uint, len(created))
	for i := range created {
		wantIDs[i] = created[len(created)-1-i].ID
	}

	const pageSize = 2
	pageCount := (len(created) + pageSize - 1) / pageSize
	service := NewAuditService(db)
	gotIDs := make([]uint, 0, len(created))
	for page := 1; page <= pageCount; page++ {
		logs, total, err := service.List(AuditListFilter{}, page, pageSize)
		if err != nil {
			t.Fatalf("list synthetic audit page %d: %v", page, err)
		}
		if total != int64(len(created)) {
			t.Fatalf("page %d total = %d, want %d", page, total, len(created))
		}
		for _, log := range logs {
			gotIDs = append(gotIDs, log.ID)
		}
	}

	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("listed %d audit events, want %d; IDs=%v", len(gotIDs), len(wantIDs), gotIDs)
	}
	for i, wantID := range wantIDs {
		if gotIDs[i] != wantID {
			t.Fatalf("audit ID order at position %d = %d, want %d; got IDs=%v", i, gotIDs[i], wantID, gotIDs)
		}
	}
}
