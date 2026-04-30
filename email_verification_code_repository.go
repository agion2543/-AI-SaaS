package repository

import (
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

type EmailVerificationCodeRepository struct {
	db *gorm.DB
}

func NewEmailVerificationCodeRepository(db *gorm.DB) *EmailVerificationCodeRepository {
	return &EmailVerificationCodeRepository{db: db}
}

func (r *EmailVerificationCodeRepository) Create(record *model.EmailVerificationCode) error {
	return r.db.Create(record).Error
}

func (r *EmailVerificationCodeRepository) Save(record *model.EmailVerificationCode) error {
	return r.db.Save(record).Error
}

func (r *EmailVerificationCodeRepository) InvalidateActive(email, scene string) error {
	now := time.Now()
	return r.db.Model(&model.EmailVerificationCode{}).
		Where("email = ? AND scene = ? AND consumed_at IS NULL AND expires_at > ?", email, scene, now).
		Update("consumed_at", now).Error
}

func (r *EmailVerificationCodeRepository) FindValid(email, scene, code string) (*model.EmailVerificationCode, error) {
	var record model.EmailVerificationCode
	now := time.Now()
	err := r.db.Where(
		"email = ? AND scene = ? AND code = ? AND consumed_at IS NULL AND expires_at > ?",
		email,
		scene,
		code,
		now,
	).Order("id desc").First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}
