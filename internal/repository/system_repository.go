package repository

import (
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type SystemRepository struct {
	db *gorm.DB
}

func NewSystemRepository(db *gorm.DB) *SystemRepository {
	return &SystemRepository{db: db}
}

func (r *SystemRepository) Upsert(key, value string, encrypted bool) error {
	var cfg model.SystemConfig
	err := r.db.Where("config_key = ?", key).First(&cfg).Error
	if err == nil {
		cfg.ConfigValue = value
		cfg.IsEncrypted = encrypted
		return r.db.Save(&cfg).Error
	}

	return r.db.Create(&model.SystemConfig{
		ConfigKey:   key,
		ConfigValue: value,
		IsEncrypted: encrypted,
	}).Error
}

func (r *SystemRepository) Get(key string) (*model.SystemConfig, error) {
	var cfg model.SystemConfig
	if err := r.db.Where("config_key = ?", key).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *SystemRepository) List() ([]model.SystemConfig, error) {
	var configs []model.SystemConfig
	err := r.db.Order("id asc").Find(&configs).Error
	return configs, err
}

func (r *SystemRepository) CreateUsage(record *model.UsageRecord) error {
	return r.db.Create(record).Error
}

func (r *SystemRepository) UserUsages(userID uint) ([]model.UsageRecord, error) {
	var records []model.UsageRecord
	err := r.db.Where("user_id = ?", userID).Order("id desc").Find(&records).Error
	return records, err
}
