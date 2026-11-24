package service

import (
	"testing"

	"go-web-gin-health/internal/model"
)

func TestListStoreProductsFilteredSearchesWithinMerchantStoreAndKeepsPagination(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, store, descriptionMatch := createSyntheticPricingProduct(t, db, 100, 0, nil)

	nameMatch := model.StoreProduct{
		StoreID:     store.ID,
		Name:        "快照饮料",
		Price:       200,
		Description: "日常商品",
		Status:      "active",
		Sort:        1,
	}
	if err := db.Create(&nameMatch).Error; err != nil {
		t.Fatalf("create same-store name match: %v", err)
	}
	unrelated := model.StoreProduct{
		StoreID:     store.ID,
		Name:        "手冲咖啡",
		Price:       300,
		Description: "单品咖啡",
		Status:      "active",
		Sort:        2,
	}
	if err := db.Create(&unrelated).Error; err != nil {
		t.Fatalf("create same-store non-match: %v", err)
	}

	foreignMerchant := model.Merchant{Name: "搜索范围外商户", Status: "active"}
	if err := db.Create(&foreignMerchant).Error; err != nil {
		t.Fatalf("create foreign merchant: %v", err)
	}
	foreignStore := model.Store{MerchantID: foreignMerchant.ID, Name: "范围外门店", Status: "active", IsOpen: true}
	if err := db.Create(&foreignStore).Error; err != nil {
		t.Fatalf("create foreign store: %v", err)
	}
	foreignProduct := model.StoreProduct{
		StoreID:     foreignStore.ID,
		Name:        "外部商品",
		Price:       400,
		Description: "快照关键词在其他门店",
		Status:      "active",
	}
	if err := db.Create(&foreignProduct).Error; err != nil {
		t.Fatalf("create foreign-store matching description: %v", err)
	}

	merchantService := NewMerchantService(nil, db)
	firstPage, total, err := merchantService.ListStoreProductsFiltered(merchant.ID, store.ID, 1, 1, " 快照 ")
	if err != nil {
		t.Fatalf("search first page: %v", err)
	}
	if total != 2 || len(firstPage) != 1 || firstPage[0].ID != nameMatch.ID {
		t.Fatalf("first page should include the name match only and count both local matches: total=%d rows=%+v", total, firstPage)
	}

	secondPage, secondTotal, err := merchantService.ListStoreProductsFiltered(merchant.ID, store.ID, 2, 1, "快照")
	if err != nil {
		t.Fatalf("search second page: %v", err)
	}
	if secondTotal != 2 || len(secondPage) != 1 || secondPage[0].ID != descriptionMatch.ID {
		t.Fatalf("second page should include the description match: total=%d rows=%+v", secondTotal, secondPage)
	}

	allProducts, allTotal, err := merchantService.ListStoreProducts(merchant.ID, store.ID, 1, 10)
	if err != nil {
		t.Fatalf("list with legacy empty-keyword method: %v", err)
	}
	if allTotal != 3 || len(allProducts) != 3 {
		t.Fatalf("empty keyword should preserve the unfiltered list: total=%d rows=%d", allTotal, len(allProducts))
	}

	if products, foreignTotal, err := merchantService.ListStoreProductsFiltered(merchant.ID, foreignStore.ID, 1, 10, "快照"); err == nil || foreignTotal != 0 || len(products) != 0 {
		t.Fatalf("merchant must not search another merchant's store: total=%d rows=%+v err=%v", foreignTotal, products, err)
	}
}
