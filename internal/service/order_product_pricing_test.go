package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

func createSyntheticPricingProduct(t *testing.T, db *gorm.DB, price int64, minimum int, tiers []model.ProductPriceTier) (model.Merchant, model.Store, model.StoreProduct) {
	t.Helper()
	merchant := model.Merchant{Name: "合成档位价商户", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic pricing merchant: %v", err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "合成档位价门店", Status: "active", IsOpen: true, OrderMode: "submit_later"}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic pricing store: %v", err)
	}
	stock := 1000
	product := model.StoreProduct{
		StoreID: store.ID, Name: "合成商品", Price: price, Description: "下单快照描述",
		ImageURL: "https://example.invalid/synthetic-product.png", Category: "合成",
		Status: "active", Stock: &stock, SalesUnit: "件", Specification: "合成规格",
		MinimumOrderQuantity: minimum, PriceTiers: tiers,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create synthetic pricing product: %v", err)
	}
	return merchant, store, product
}

func createSyntheticPricingOrder(t *testing.T, db *gorm.DB, store model.Store, items ...dto.CustomerOrderItemRequest) *model.Order {
	t.Helper()
	order, err := NewOrderService(db).CreateCustomerStoreOrder(dto.CreateCustomerOrderRequest{StoreID: store.ID, Items: items})
	if err != nil {
		t.Fatalf("create synthetic pricing order: %v", err)
	}
	return order
}

func decodePricingSnapshots(t *testing.T, raw string) []StoreOrderItemSnapshot {
	t.Helper()
	var items []StoreOrderItemSnapshot
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("decode synthetic order snapshots: %v", err)
	}
	return items
}

func TestStoreOrderPricingKeepsLegacySinglePriceAndIgnoresForgedClientPrice(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, store, product := createSyntheticPricingProduct(t, db, 250, 0, nil)
	order := createSyntheticPricingOrder(t, db, store, dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: 3, Price: 1})
	if order.Amount != 750 || order.TotalAmount != 750 {
		t.Fatalf("legacy no-tier order should use the existing base price: amount=%d total=%d", order.Amount, order.TotalAmount)
	}
	items := decodePricingSnapshots(t, order.Items)
	if len(items) != 1 || items[0].Price != 250 || items[0].BasePrice != 250 || items[0].LineAmount != 750 || items[0].MinimumOrderQuantity != 1 {
		t.Fatalf("legacy snapshot or server price mismatch: %+v", items)
	}
}

func TestStoreOrderTierPricingUsesMerchantThresholdsAtExactBoundaries(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, store, product := createSyntheticPricingProduct(t, db, 100, 1, []model.ProductPriceTier{
		{MinQuantity: 5, Price: 80}, {MinQuantity: 3, Price: 90},
	})
	for _, check := range []struct {
		quantity  int
		unit      int64
		threshold int
	}{
		{quantity: 2, unit: 100},
		{quantity: 3, unit: 90, threshold: 3},
		{quantity: 4, unit: 90, threshold: 3},
		{quantity: 5, unit: 80, threshold: 5},
		{quantity: 6, unit: 80, threshold: 5},
	} {
		t.Run(string(rune('0'+check.quantity)), func(t *testing.T) {
			order := createSyntheticPricingOrder(t, db, store, dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: check.quantity, Price: 1_000_000})
			items := decodePricingSnapshots(t, order.Items)
			if order.Amount != check.unit*int64(check.quantity) || len(items) != 1 || items[0].Price != check.unit || items[0].AppliedTierMinQuantity != check.threshold {
				t.Fatalf("quantity %d pricing mismatch: amount=%d items=%+v", check.quantity, order.Amount, items)
			}
		})
	}
}

func TestStoreOrderPricingEnforcesMOQAfterMergingDuplicateProductLines(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, store, product := createSyntheticPricingProduct(t, db, 100, 3, []model.ProductPriceTier{{MinQuantity: 3, Price: 90}})
	service := NewOrderService(db)
	_, err := service.CreateCustomerStoreOrder(dto.CreateCustomerOrderRequest{StoreID: store.ID, Items: []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 2}}})
	if err == nil || !strings.Contains(err.Error(), "最低购买量为 3") {
		t.Fatalf("below-MOQ order should be rejected, got %v", err)
	}
	var before int64
	if err := db.Model(&model.Order{}).Count(&before).Error; err != nil {
		t.Fatal(err)
	}
	order := createSyntheticPricingOrder(t, db, store,
		dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: 1, Price: 1},
		dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: 2, Price: 1},
	)
	items := decodePricingSnapshots(t, order.Items)
	if order.Amount != 270 || len(items) != 1 || items[0].Quantity != 3 || items[0].Price != 90 {
		t.Fatalf("duplicate request lines must aggregate before MOQ/tier pricing: amount=%d items=%+v", order.Amount, items)
	}
	var after int64
	if err := db.Model(&model.Order{}).Count(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after != before+1 {
		t.Fatalf("failed MOQ request created an order: before=%d after=%d", before, after)
	}
}

func TestStoreOrderTierPricingAndPromotionDiscountInteractOnServerAmount(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, store, product := createSyntheticPricingProduct(t, db, 100, 1, []model.ProductPriceTier{{MinQuantity: 3, Price: 80}})
	promotion := model.Promotion{MerchantID: merchant.ID, StoreID: &store.ID, Title: "合成满额优惠", Type: "amount", Threshold: 400, Discount: 50, Status: "published"}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create synthetic promotion: %v", err)
	}
	order := createSyntheticPricingOrder(t, db, store, dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: 5, Price: 1})
	if order.Amount != 400 || order.DiscountAmount != 50 || order.TotalAmount != 350 {
		t.Fatalf("promotion should apply after server-side tier pricing: amount=%d discount=%d total=%d", order.Amount, order.DiscountAmount, order.TotalAmount)
	}
	items := decodePricingSnapshots(t, order.Items)
	if len(items) != 1 || items[0].Price != 80 || items[0].LineAmount != 400 {
		t.Fatalf("promotion changed purchase-time unit-price snapshot: %+v", items)
	}
}

func TestAppendingStoreOrderItemsKeepsOldSnapshotAndUsesCurrentPriceForNewLine(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, store, product := createSyntheticPricingProduct(t, db, 100, 1, []model.ProductPriceTier{{MinQuantity: 3, Price: 90}})
	service := NewOrderService(db)
	order := createSyntheticPricingOrder(t, db, store, dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: 3, Price: 1})
	if order.Amount != 270 {
		t.Fatalf("initial tier amount=%d, want 270", order.Amount)
	}
	product.Name = "编辑后的商品名"
	product.Price = 120
	product.SalesUnit = "箱"
	product.Specification = "新规格"
	product.PriceTiers = []model.ProductPriceTier{{MinQuantity: 2, Price: 70}}
	if err := db.Save(&product).Error; err != nil {
		t.Fatalf("edit synthetic product after order: %v", err)
	}
	updated, err := service.AppendCustomerStoreOrderItems(order.OrderNo, order.CustomerAccessToken, dto.AppendCustomerOrderItemsRequest{
		Items: []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 2, Price: 1}},
	})
	if err != nil {
		t.Fatalf("append synthetic order line: %v", err)
	}
	items := decodePricingSnapshots(t, updated.Items)
	if updated.Amount != 410 || updated.TotalAmount != 410 || len(items) != 2 {
		t.Fatalf("append should add the new server-priced line only: order=%+v snapshots=%+v", updated, items)
	}
	if items[0].Name != "合成商品" || items[0].Price != 90 || items[0].BasePrice != 100 || items[0].SalesUnit != "件" || items[0].Specification != "合成规格" || items[0].Quantity != 3 {
		t.Fatalf("later product edits changed the original immutable snapshot: %+v", items[0])
	}
	if items[1].Name != "编辑后的商品名" || items[1].Price != 70 || items[1].BasePrice != 120 || items[1].SalesUnit != "箱" || items[1].Specification != "新规格" || items[1].Quantity != 2 {
		t.Fatalf("new appended line did not snapshot current product settings: %+v", items[1])
	}
}
