package repository

import (
	"time"

	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// LockPhone serializes sends for one phone in databases that support row locks.
// The phone-prefix index also lets MySQL protect the empty range on first send.
func (r *VerificationCodeRepository) LockPhone(phone string) error {
	var ids []uint
	return r.db.Model(&model.VerificationCode{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("phone = ?", phone).
		Order("id ASC").
		Pluck("id", &ids).Error
}

func (r *VerificationCodeRepository) CountIssuedSince(phone string, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&model.VerificationCode{}).
		Where("phone = ? AND created_at >= ?", phone, since).
		Count(&count).Error
	return count, err
}

func (r *VerificationCodeRepository) InvalidateActive(phone, scene string) error {
	now := time.Now()
	return r.db.Model(&model.VerificationCode{}).
		Where("phone = ? AND scene = ? AND consumed_at IS NULL AND expires_at > ?", phone, scene, now).
		Update("consumed_at", now).Error
}

func (r *VerificationCodeRepository) FindActiveForUpdate(phone, scene string, userID *uint, now time.Time) (*model.VerificationCode, error) {
	var record model.VerificationCode
	query := r.db.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("phone = ? AND scene = ? AND consumed_at IS NULL AND expires_at > ?", phone, scene, now)
	if userID == nil {
		query = query.Where("user_id IS NULL")
	} else {
		query = query.Where("user_id = ?", *userID)
	}
	err := query.Order("id desc").First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *VerificationCodeRepository) Save(record *model.VerificationCode) error {
	return r.db.Save(record).Error
}

func (r *VerificationCodeRepository) RecordFailedAttempt(record *model.VerificationCode, now time.Time, maxAttempts uint) error {
	updates := map[string]interface{}{"failed_attempts": record.FailedAttempts}
	if record.FailedAttempts >= maxAttempts {
		updates["consumed_at"] = now
	}
	return r.db.Model(&model.VerificationCode{}).Where("id = ?", record.ID).Updates(updates).Error
}
