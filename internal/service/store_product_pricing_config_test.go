package service

import (
	"strings"
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestSaveStoreProductNormalizesTiersAndPreservesOmittedLegacyFields(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant := model.Merchant{Name: "合成商品配置商户", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatal(err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "合成商品配置门店", Status: "active", IsOpen: true}
	if err := db.Create(&store).Error; err != nil {
		t.Fatal(err)
	}
	unit, specification := "袋", "500g/袋"
	minimum := 2
	tiers := []dto.StoreProductPriceTierRequest{{MinQuantity: 5, Price: 800}, {MinQuantity: 3, Price: 900}}
	service := NewMerchantService(nil, db)
	product, err := service.SaveStoreProduct(merchant.ID, 0, dto.SaveStoreProductRequest{
		StoreID: store.ID, Name: "合成大米", Price: 1000, SalesUnit: &unit,
		Specification: &specification, MinimumOrderQuantity: &minimum, PriceTiers: &tiers,
	})
	if err != nil {
		t.Fatalf("create synthetic configured product: %v", err)
	}
	if product.SalesUnit != unit || product.Specification != specification || product.MinimumOrderQuantity != minimum || len(product.PriceTiers) != 2 || product.PriceTiers[0].MinQuantity != 3 || product.PriceTiers[1].MinQuantity != 5 {
		t.Fatalf("product pricing configuration was not normalized: %+v", product)
	}

	updated, err := service.SaveStoreProduct(merchant.ID, product.ID, dto.SaveStoreProductRequest{
		StoreID: store.ID, Name: product.Name, Price: 1100,
	})
	if err != nil {
		t.Fatalf("legacy update with omitted optional fields should remain compatible: %v", err)
	}
	if updated.SalesUnit != unit || updated.Specification != specification || updated.MinimumOrderQuantity != minimum || len(updated.PriceTiers) != 2 || updated.PriceTiers[0].Price != 900 {
		t.Fatalf("legacy update unexpectedly cleared tier configuration: %+v", updated)
	}

	_, err = service.SaveStoreProduct(merchant.ID, product.ID, dto.SaveStoreProductRequest{StoreID: store.ID, Name: product.Name, Price: 500})
	if err == nil || !strings.Contains(err.Error(), "档位单价") {
		t.Fatalf("lowering base price below a preserved tier must be rejected, got %v", err)
	}
}

func TestSaveStoreProductRejectsInvalidMerchantTierRules(t *testing.T) {
	for _, check := range []struct {
		name  string
		tiers []dto.StoreProductPriceTierRequest
		want  string
	}{
		{name: "duplicate threshold", tiers: []dto.StoreProductPriceTierRequest{{MinQuantity: 3, Price: 900}, {MinQuantity: 3, Price: 800}}, want: "不能重复"},
		{name: "ascending price", tiers: []dto.StoreProductPriceTierRequest{{MinQuantity: 3, Price: 800}, {MinQuantity: 5, Price: 900}}, want: "不能上升"},
		{name: "above base", tiers: []dto.StoreProductPriceTierRequest{{MinQuantity: 3, Price: 1100}}, want: "基础单价"},
	} {
		t.Run(check.name, func(t *testing.T) {
			db := newOrderBoundaryTestDB(t)
			merchant := model.Merchant{Name: "合成档位校验商户-" + check.name, Status: "active"}
			if err := db.Create(&merchant).Error; err != nil {
				t.Fatal(err)
			}
			store := model.Store{MerchantID: merchant.ID, Name: "合成档位校验门店-" + check.name, Status: "active", IsOpen: true}
			if err := db.Create(&store).Error; err != nil {
				t.Fatal(err)
			}
			tiers := check.tiers
			_, err := NewMerchantService(nil, db).SaveStoreProduct(merchant.ID, 0, dto.SaveStoreProductRequest{StoreID: store.ID, Name: "合成商品", Price: 1000, PriceTiers: &tiers})
			if err == nil || !strings.Contains(err.Error(), check.want) {
				t.Fatalf("invalid tier should be rejected with %q, got %v", check.want, err)
			}
			var count int64
			if err := db.Model(&model.StoreProduct{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("invalid pricing configuration created %d product rows", count)
			}
		})
	}
}
