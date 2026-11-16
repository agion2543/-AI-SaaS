package service

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const maxProductPriceTiers = 20

func normalizeStoreProductPricing(salesUnit, specification string, minimumOrderQuantity int, tiers []model.ProductPriceTier, basePrice int64) (string, string, int, []model.ProductPriceTier, error) {
	salesUnit = strings.TrimSpace(salesUnit)
	specification = strings.TrimSpace(specification)
	if len([]rune(salesUnit)) > 30 {
		return "", "", 0, nil, errors.New("销售单位不能超过 30 个字符")
	}
	if len([]rune(specification)) > 80 {
		return "", "", 0, nil, errors.New("商品规格不能超过 80 个字符")
	}
	if basePrice < 0 {
		return "", "", 0, nil, errors.New("商品价格不能为负数")
	}
	if minimumOrderQuantity == 0 {
		minimumOrderQuantity = 1
	}
	if minimumOrderQuantity < 1 || minimumOrderQuantity > 1_000_000_000 {
		return "", "", 0, nil, errors.New("最低购买量必须在 1 到 10 亿之间")
	}
	if len(tiers) > maxProductPriceTiers {
		return "", "", 0, nil, fmt.Errorf("档位价最多可设置 %d 档", maxProductPriceTiers)
	}

	normalized := append([]model.ProductPriceTier(nil), tiers...)
	seen := make(map[int]struct{}, len(normalized))
	for _, tier := range normalized {
		if tier.MinQuantity < 2 || tier.MinQuantity > 1_000_000_000 {
			return "", "", 0, nil, errors.New("档位数量必须在 2 到 10 亿之间")
		}
		if tier.Price < 0 || tier.Price > basePrice {
			return "", "", 0, nil, errors.New("档位单价必须在 0 到商品基础单价之间")
		}
		if _, exists := seen[tier.MinQuantity]; exists {
			return "", "", 0, nil, errors.New("档位数量不能重复")
		}
		seen[tier.MinQuantity] = struct{}{}
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].MinQuantity < normalized[j].MinQuantity
	})
	for i := 1; i < len(normalized); i++ {
		if normalized[i].Price > normalized[i-1].Price {
			return "", "", 0, nil, errors.New("购买量增加时，档位单价不能上升")
		}
	}
	return salesUnit, specification, minimumOrderQuantity, normalized, nil
}

func resolveStoreProductUnitPrice(product model.StoreProduct, quantity int) (unitPrice int64, appliedTierMinQuantity int, err error) {
	if quantity <= 0 {
		return 0, 0, errors.New("商品数量必须大于 0")
	}
	_, _, minimum, tiers, err := normalizeStoreProductPricing(product.SalesUnit, product.Specification, product.MinimumOrderQuantity, product.PriceTiers, product.Price)
	if err != nil {
		return 0, 0, fmt.Errorf("%s 的价格配置无效：%w", product.Name, err)
	}
	if quantity < minimum {
		return 0, 0, fmt.Errorf("%s 最低购买量为 %d", product.Name, minimum)
	}
	unitPrice = product.Price
	for _, tier := range tiers {
		if quantity >= tier.MinQuantity {
			unitPrice = tier.Price
			appliedTierMinQuantity = tier.MinQuantity
		}
	}
	return unitPrice, appliedTierMinQuantity, nil
}

func calculateStoreOrderLineAmount(product model.StoreProduct, quantity int) (unitPrice int64, appliedTierMinQuantity int, lineAmount int64, err error) {
	unitPrice, appliedTierMinQuantity, err = resolveStoreProductUnitPrice(product, quantity)
	if err != nil {
		return 0, 0, 0, err
	}
	if unitPrice != 0 && int64(quantity) > math.MaxInt64/unitPrice {
		return 0, 0, 0, fmt.Errorf("%s 订单金额超出允许范围", product.Name)
	}
	return unitPrice, appliedTierMinQuantity, unitPrice * int64(quantity), nil
}

func addStoreOrderAmount(total, amount int64) (int64, error) {
	if amount < 0 || total < 0 || total > math.MaxInt64-amount {
		return 0, errors.New("订单金额超出允许范围")
	}
	return total + amount, nil
}

func aggregateCustomerOrderItems(items []dto.CustomerOrderItemRequest) ([]dto.CustomerOrderItemRequest, error) {
	if len(items) == 0 {
		return nil, errors.New("please select at least one product")
	}
	aggregated := make([]dto.CustomerOrderItemRequest, 0, len(items))
	indexByProduct := make(map[uint]int, len(items))
	for _, item := range items {
		if item.ProductID == 0 {
			return nil, errors.New("some products are unavailable")
		}
		if item.Quantity <= 0 {
			return nil, errors.New("product quantity must be greater than 0")
		}
		if index, exists := indexByProduct[item.ProductID]; exists {
			if item.Quantity > int(^uint(0)>>1)-aggregated[index].Quantity {
				return nil, errors.New("product quantity exceeds the allowed range")
			}
			aggregated[index].Quantity += item.Quantity
			continue
		}
		indexByProduct[item.ProductID] = len(aggregated)
		aggregated = append(aggregated, item)
	}
	return aggregated, nil
}

func buildStoreOrderItemSnapshot(product model.StoreProduct, quantity int) (StoreOrderItemSnapshot, int64, error) {
	unitPrice, appliedTierMinQuantity, lineAmount, err := calculateStoreOrderLineAmount(product, quantity)
	if err != nil {
		return StoreOrderItemSnapshot{}, 0, err
	}
	minimumOrderQuantity := product.MinimumOrderQuantity
	if minimumOrderQuantity == 0 {
		minimumOrderQuantity = 1
	}
	return StoreOrderItemSnapshot{
		ProductID:              product.ID,
		Name:                   product.Name,
		Price:                  unitPrice,
		BasePrice:              product.Price,
		Quantity:               quantity,
		LineAmount:             lineAmount,
		Description:            product.Description,
		ImageURL:               product.ImageURL,
		SalesUnit:              product.SalesUnit,
		Specification:          product.Specification,
		MinimumOrderQuantity:   minimumOrderQuantity,
		AppliedTierMinQuantity: appliedTierMinQuantity,
		PriceTiers:             append([]model.ProductPriceTier(nil), product.PriceTiers...),
	}, lineAmount, nil
}
