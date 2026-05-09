package repository

import (
	"go-web-gin-health/internal/model"

	"gorm.io/gorm"
)

type MerchantPlanRepository struct {
	db *gorm.DB
}

func NewMerchantPlanRepository(db *gorm.DB) *MerchantPlanRepository {
	return &MerchantPlanRepository{db: db}
}

func (r *MerchantPlanRepository) List() ([]model.MerchantPlan, error) {
	var plans []model.MerchantPlan
	err := r.db.Order("sort asc, id asc").Find(&plans).Error
	return plans, err
}

func (r *MerchantPlanRepository) FindByID(id uint) (*model.MerchantPlan, error) {
	var plan model.MerchantPlan
	err := r.db.First(&plan, id).Error
	return &plan, err
}

func (r *MerchantPlanRepository) Save(plan *model.MerchantPlan) error {
	return r.db.Save(plan).Error
}

func (r *MerchantPlanRepository) FindByName(name string) (*model.MerchantPlan, error) {
	var plan model.MerchantPlan
	err := r.db.Where("name = ?", name).First(&plan).Error
	return &plan, err
}
