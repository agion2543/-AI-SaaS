package repository

import (
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type StoreRepository struct {
	db *gorm.DB
}

func NewStoreRepository(db *gorm.DB) *StoreRepository {
	return &StoreRepository{db: db}
}

func (r *StoreRepository) Create(store *model.Store) error {
	return r.db.Create(store).Error
}

func (r *StoreRepository) Save(store *model.Store) error {
	return r.db.Save(store).Error
}

func (r *StoreRepository) FindByID(id uint) (*model.Store, error) {
	var store model.Store
	if err := r.db.First(&store, id).Error; err != nil {
		return nil, err
	}
	return &store, nil
}

func (r *StoreRepository) FindByIDWithMerchant(id uint) (*model.Store, error) {
	var store model.Store
	if err := r.db.Preload("Merchant").First(&store, id).Error; err != nil {
		return nil, err
	}
	return &store, nil
}

func (r *StoreRepository) FindByMerchantAndID(merchantID, id uint) (*model.Store, error) {
	var store model.Store
	if err := r.db.Where("merchant_id = ? AND id = ?", merchantID, id).First(&store).Error; err != nil {
		return nil, err
	}
	return &store, nil
}

func (r *StoreRepository) FindByMerchantAndName(merchantID uint, name string) (*model.Store, error) {
	var store model.Store
	if err := r.db.Where("merchant_id = ? AND name = ?", merchantID, name).First(&store).Error; err != nil {
		return nil, err
	}
	return &store, nil
}

func (r *StoreRepository) ListByMerchant(merchantID uint, offset, limit int) ([]model.Store, int64, error) {
	var stores []model.Store
	var total int64

	query := r.db.Model(&model.Store{}).Where("merchant_id = ?", merchantID)
	query.Count(&total)
	err := query.Order("id desc").Offset(offset).Limit(limit).Find(&stores).Error

	return stores, total, err
}

type StoreProductRepository struct {
	db *gorm.DB
}

func NewStoreProductRepository(db *gorm.DB) *StoreProductRepository {
	return &StoreProductRepository{db: db}
}

func (r *StoreProductRepository) Create(product *model.StoreProduct) error {
	return r.db.Create(product).Error
}

func (r *StoreProductRepository) Save(product *model.StoreProduct) error {
	return r.db.Save(product).Error
}

func (r *StoreProductRepository) FindByID(id uint) (*model.StoreProduct, error) {
	var product model.StoreProduct
	if err := r.db.Preload("Store").First(&product, id).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *StoreProductRepository) ListByStore(storeID uint, offset, limit int) ([]model.StoreProduct, int64, error) {
	var products []model.StoreProduct
	var total int64

	query := r.db.Model(&model.StoreProduct{}).Where("store_id = ?", storeID)
	query.Count(&total)
	err := query.Order("sort asc, id desc").Offset(offset).Limit(limit).Find(&products).Error

	return products, total, err
}

func (r *StoreProductRepository) ListActiveByStore(storeID uint) ([]model.StoreProduct, error) {
	var products []model.StoreProduct
	err := r.db.Where("store_id = ? AND status = ?", storeID, "active").Order("sort asc, id asc").Find(&products).Error
	return products, err
}

func (r *StoreProductRepository) FindActiveByIDs(storeID uint, ids []uint) ([]model.StoreProduct, error) {
	var products []model.StoreProduct
	err := r.db.Where("store_id = ? AND status = ? AND id IN ?", storeID, "active", ids).Find(&products).Error
	return products, err
}
