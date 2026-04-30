package repository

import (
	"go-web-gin-health/internal/model"

	"gorm.io/gorm"
)

type CustomerLeadRepository struct {
	db *gorm.DB
}

func NewCustomerLeadRepository(db *gorm.DB) *CustomerLeadRepository {
	return &CustomerLeadRepository{db: db}
}

func (r *CustomerLeadRepository) Create(lead *model.CustomerLead) error {
	return r.db.Create(lead).Error
}

func (r *CustomerLeadRepository) Save(lead *model.CustomerLead) error {
	return r.db.Save(lead).Error
}

func (r *CustomerLeadRepository) FindByMerchantAndID(merchantID, id uint) (*model.CustomerLead, error) {
	var lead model.CustomerLead
	if err := r.db.Where("merchant_id = ? AND id = ?", merchantID, id).First(&lead).Error; err != nil {
		return nil, err
	}
	return &lead, nil
}

func (r *CustomerLeadRepository) ListByMerchant(merchantID uint, offset, limit int) ([]model.CustomerLead, int64, error) {
	var leads []model.CustomerLead
	var total int64

	query := r.db.Model(&model.CustomerLead{}).Where("merchant_id = ?", merchantID)
	query.Count(&total)
	err := query.Preload("Store").Order("id desc").Offset(offset).Limit(limit).Find(&leads).Error
	return leads, total, err
}

func (r *CustomerLeadRepository) CountByStatus(merchantID uint) (map[string]int64, error) {
	type row struct {
		Status string
		Total  int64
	}
	var rows []row
	stats := map[string]int64{
		"new":       0,
		"contacted": 0,
		"converted": 0,
		"invalid":   0,
	}

	if err := r.db.Model(&model.CustomerLead{}).
		Select("status, count(*) as total").
		Where("merchant_id = ?", merchantID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		stats[item.Status] = item.Total
	}
	return stats, nil
}
