package repository

import (
	"strings"

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

// List 返回一页商家（id DESC）。keyword 非空时按 name 模糊搜（LIKE，
// % _ \ 先转义），用于下拉框远程搜索；total 是筛选后的计数。
// 分页仍走全局 NormalizePage 上限（page_size > 100 会被压成 10），
// 调用方不得靠放大 page_size 拉全量。
func (r *MerchantRepository) List(offset, limit int, keyword string) ([]model.Merchant, int64, error) {
	query := r.db.Model(&model.Merchant{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(keyword)
		query = query.Where("name LIKE ? ESCAPE '\\'", "%"+escaped+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var merchants []model.Merchant
	err := query.Order("id desc").Offset(offset).Limit(limit).Find(&merchants).Error
	return merchants, total, err
}
