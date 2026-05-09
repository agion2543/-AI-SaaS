package repository

import (
	"time"

	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type VerificationCodeRepository struct {
	db *gorm.DB
}

func NewVerificationCodeRepository(db *gorm.DB) *VerificationCodeRepository {
	return &VerificationCodeRepository{db: db}
}

func (r *VerificationCodeRepository) Create(record *model.VerificationCode) error {
	return r.db.Create(record).Error
}

func (r *VerificationCodeRepository) InvalidateActive(phone, scene string) error {
	now := time.Now()
	return r.db.Model(&model.VerificationCode{}).
		Where("phone = ? AND scene = ? AND consumed_at IS NULL AND expires_at > ?", phone, scene, now).
		Update("consumed_at", now).Error
}

func (r *VerificationCodeRepository) FindValid(phone, scene, code string) (*model.VerificationCode, error) {
	now := time.Now()
	var record model.VerificationCode
	err := r.db.
		Where("phone = ? AND scene = ? AND code = ? AND consumed_at IS NULL AND expires_at > ?", phone, scene, code, now).
		Order("id desc").
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *VerificationCodeRepository) Save(record *model.VerificationCode) error {
	return r.db.Save(record).Error
}
