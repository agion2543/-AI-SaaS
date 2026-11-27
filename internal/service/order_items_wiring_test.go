package service

import (
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

// order_items wiring (P2): the store-order creation paths must persist one
// order_items row per priced snapshot, in the same transaction as the order,
// so the v11 table actually carries data. Historical orders created before
// the wiring have no rows and are not backfilled.
func TestCreateCustomerStoreOrderWritesOrderItems(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.OrderItem{}); err != nil {
		t.Fatalf("migrate order_items test table: %v", err)
	}
	_, store, first := createSyntheticPricingProduct(t, db, 250, 0, nil)
	stock := 1000
	second := model.StoreProduct{
		StoreID: store.ID, Name: "合成商品二", Price: 120, Description: "第二件",
		Status: "active", Stock: &stock, SalesUnit: "件", MinimumOrderQuantity: 1,
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second synthetic product: %v", err)
	}

	order := createSyntheticPricingOrder(t, db, store,
		dto.CustomerOrderItemRequest{ProductID: first.ID, Quantity: 2},
		dto.CustomerOrderItemRequest{ProductID: second.ID, Quantity: 3},
	)
	if order.Amount != 2*250+3*120 {
		t.Fatalf("order amount should be 860 cents, got %d", order.Amount)
	}

	var rows []model.OrderItem
	if err := db.Where("order_id = ?", order.ID).Order("product_id ASC").Find(&rows).Error; err != nil {
		t.Fatalf("read order_items rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 order_items rows, got %d", len(rows))
	}
	if rows[0].ProductID != first.ID || rows[0].Quantity != 2 || rows[0].UnitPrice != 250 || rows[0].Amount != 500 || rows[0].ProductNameSnapshot != "合成商品" {
		t.Fatalf("first order_items row mismatch: %+v", rows[0])
	}
	if rows[1].ProductID != second.ID || rows[1].Quantity != 3 || rows[1].UnitPrice != 120 || rows[1].Amount != 360 || rows[1].ProductNameSnapshot != "合成商品二" {
		t.Fatalf("second order_items row mismatch: %+v", rows[1])
	}
	var total int64
	for _, row := range rows {
		total += row.Amount
	}
	if total != order.Amount {
		t.Fatalf("order_items amounts should sum to order amount %d, got %d", order.Amount, total)
	}
}

func TestAppendCustomerStoreOrderItemsWritesOrderItems(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.OrderItem{}); err != nil {
		t.Fatalf("migrate order_items test table: %v", err)
	}
	_, store, product := createSyntheticPricingProduct(t, db, 250, 0, nil)
	order := createSyntheticPricingOrder(t, db, store,
		dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: 1},
	)

	appended, err := NewOrderService(db).AppendCustomerStoreOrderItems(order.OrderNo, order.CustomerAccessToken,
		dto.AppendCustomerOrderItemsRequest{Items: []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 2}}})
	if err != nil {
		t.Fatalf("append order items: %v", err)
	}
	if appended.Amount != 750 {
		t.Fatalf("appended order amount should be 750 cents, got %d", appended.Amount)
	}

	var rows []model.OrderItem
	if err := db.Where("order_id = ?", order.ID).Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatalf("read order_items rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 order_items rows after append, got %d", len(rows))
	}
	if rows[0].Quantity != 1 || rows[0].Amount != 250 || rows[1].Quantity != 2 || rows[1].Amount != 500 {
		t.Fatalf("appended order_items rows mismatch: %+v", rows)
	}
}

func TestOrderItemsFromSnapshotsMapsPrices(t *testing.T) {
	rows := orderItemsFromSnapshots(7, []StoreOrderItemSnapshot{
		{ProductID: 3, Name: "A", Price: 100, Quantity: 2, LineAmount: 200},
		{ProductID: 4, Name: "B", Price: 50, Quantity: 1, LineAmount: 50},
	})
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].OrderID != 7 || rows[0].UnitPrice != 100 || rows[0].Amount != 200 || rows[0].ProductNameSnapshot != "A" {
		t.Fatalf("first mapped row mismatch: %+v", rows[0])
	}
	if len(orderItemsFromSnapshots(7, nil)) != 0 {
		t.Fatalf("empty snapshots should map to no rows")
	}
}
