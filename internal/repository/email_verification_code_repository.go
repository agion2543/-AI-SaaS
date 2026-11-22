package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// LockEmail serializes sends for one email in databases that support row locks.
// The email-prefix index also lets MySQL protect the empty range on first send.
func (r *EmailVerificationCodeRepository) LockEmail(email string) error {
	var ids []uint
	return r.db.Model(&model.EmailVerificationCode{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("email = ?", email).
		Order("id ASC").
		Pluck("id", &ids).Error
}

func (r *EmailVerificationCodeRepository) CountIssuedSince(email string, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&model.EmailVerificationCode{}).
		Where("email = ? AND created_at >= ?", email, since).
		Count(&count).Error
	return count, err
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

func (r *EmailVerificationCodeRepository) FindActiveForUpdate(email, scene string, userID *uint, now time.Time) (*model.EmailVerificationCode, error) {
	var record model.EmailVerificationCode
	query := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"email = ? AND scene = ? AND consumed_at IS NULL AND expires_at > ?",
		email,
		scene,
		now,
	)
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

func (r *EmailVerificationCodeRepository) RecordFailedAttempt(record *model.EmailVerificationCode, now time.Time, maxAttempts uint) error {
	updates := map[string]interface{}{"failed_attempts": record.FailedAttempts}
	if record.FailedAttempts >= maxAttempts {
		updates["consumed_at"] = now
	}
	return r.db.Model(&model.EmailVerificationCode{}).Where("id = ?", record.ID).Updates(updates).Error
}
