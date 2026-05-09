package repository

import (
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type MerchantRepository struct {
	db *gorm.DB
}

func NewMerchantRepository(db *gorm.DB) *MerchantRepository {
	return &MerchantRepository{db: db}
}

func (r *MerchantRepository) Create(merchant *model.Merchant) error {
	return r.db.Create(merchant).Error
}

func (r *MerchantRepository) Save(merchant *model.Merchant) error {
	return r.db.Save(merchant).Error
}

func (r *MerchantRepository) FindByID(id uint) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.First(&merchant, id).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *MerchantRepository) FindByName(name string) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.Where("name = ?", name).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *MerchantRepository) List(offset, limit int) ([]model.Merchant, int64, error) {
	var merchants []model.Merchant
	var total int64
	r.db.Model(&model.Merchant{}).Count(&total)
	err := r.db.Order("id desc").Offset(offset).Limit(limit).Find(&merchants).Error
	return merchants, total, err
}
