package repository

import (
	"time"

	"go-web-gin-health/internal/model"

	"gorm.io/gorm"
)

type PromotionRepository struct {
	db *gorm.DB
}

func NewPromotionRepository(db *gorm.DB) *PromotionRepository {
	return &PromotionRepository{db: db}
}

func (r *PromotionRepository) Create(promotion *model.Promotion) error {
	return r.db.Create(promotion).Error
}

func (r *PromotionRepository) Save(promotion *model.Promotion) error {
	return r.db.Save(promotion).Error
}

func (r *PromotionRepository) Delete(promotion *model.Promotion) error {
	return r.db.Delete(promotion).Error
}

func (r *PromotionRepository) FindByMerchantAndID(merchantID, id uint) (*model.Promotion, error) {
	var promotion model.Promotion
	if err := r.db.Where("merchant_id = ? AND id = ?", merchantID, id).First(&promotion).Error; err != nil {
		return nil, err
	}
	return &promotion, nil
}

func (r *PromotionRepository) ListByMerchant(merchantID uint, offset, limit int) ([]model.Promotion, int64, error) {
	var promotions []model.Promotion
	var total int64
	query := r.db.Model(&model.Promotion{}).Where("merchant_id = ?", merchantID)
	query.Count(&total)
	err := query.Preload("Store").Order("id desc").Offset(offset).Limit(limit).Find(&promotions).Error
	return promotions, total, err
}

func (r *PromotionRepository) ListActiveForStore(merchantID, storeID uint) ([]model.Promotion, error) {
	var promotions []model.Promotion
	now := time.Now()
	err := r.db.Where(
		"merchant_id = ? AND status = ? AND (store_id IS NULL OR store_id = ?) AND (valid_from IS NULL OR valid_from <= ?) AND (valid_to IS NULL OR valid_to >= ?)",
		merchantID,
		"published",
		storeID,
		now,
		now,
	).Order("id desc").Find(&promotions).Error
	return promotions, err
}
