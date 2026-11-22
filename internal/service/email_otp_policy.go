package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
)

const devFixedEmailCode = "123456"

var (
	errEmailProviderNotConfigured = errors.New("邮件服务尚未配置，暂不可发送验证码")
	errEmailSendRateLimited       = errors.New("验证码发送过于频繁，请稍后再试")
	errEmailInvalidCode           = errors.New("验证码错误或已过期")
)

// generateMockEmailCode deliberately supports simulation only in development.
// Production has no configured mail provider and must not claim a mock send.
func generateMockEmailCode(appEnv string) (string, error) {
	if !isDevelopmentEnvironment(appEnv) {
		return "", errEmailProviderNotConfigured
	}
	return devFixedEmailCode, nil
}

// hashEmailCode binds an OTP to its normalized email and purpose. HMAC prevents
// offline guessing of the six-digit code if the database alone is exposed.
func hashEmailCode(secret, email, scene, code string) (string, error) {
	if len(secret) < minimumOTPKeyLength {
		return "", errOTPHashKeyTooShort
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("email-otp-v1\x00"))
	_, _ = mac.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	_, _ = mac.Write([]byte{'\x00'})
	_, _ = mac.Write([]byte(scene))
	_, _ = mac.Write([]byte{'\x00'})
	_, _ = mac.Write([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func issueEmailVerificationCode(db *gorm.DB, appEnv, secret, email, scene string, userID *uint) error {
	code, err := generateMockEmailCode(appEnv)
	if err != nil {
		return err
	}
	digest, err := hashEmailCode(secret, email, scene, code)
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		codes := repository.NewEmailVerificationCodeRepository(tx)
		if err := codes.LockEmail(email); err != nil {
			return err
		}

		now := time.Now()
		hourCount, err := codes.CountIssuedSince(email, now.Add(-otpSendWindow))
		if err != nil {
			return err
		}
		if hourCount >= otpMaxSendsPerWindow {
			return errEmailSendRateLimited
		}
		minuteCount, err := codes.CountIssuedSince(email, now.Add(-otpSendCooldown))
		if err != nil {
			return err
		}
		if minuteCount > 0 {
			return errEmailSendRateLimited
		}
		if err := codes.InvalidateActive(email, scene); err != nil {
			return err
		}

		record := &model.EmailVerificationCode{
			Email:          strings.ToLower(strings.TrimSpace(email)),
			Scene:          scene,
			Code:           legacyOTPRedacted,
			CodeHash:       digest,
			FailedAttempts: 0,
			UserID:         userID,
			ExpiresAt:      now.Add(otpLifetime),
		}
		return codes.Create(record)
	})
}

func consumeEmailVerificationCode(db *gorm.DB, secret, email, scene, code string, userID *uint) error {
	valid := false
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		valid, err = consumeEmailVerificationCodeInTransaction(tx, secret, email, scene, code, userID)
		return err
	})
	if err != nil {
		return err
	}
	if !valid {
		return errEmailInvalidCode
	}
	return nil
}

// consumeEmailVerificationCodeInTransaction persists failed attempts when the
// caller commits an invalid result, and allows successful code consumption to
// be atomic with the protected account change.
func consumeEmailVerificationCodeInTransaction(tx *gorm.DB, secret, email, scene, code string, userID *uint) (bool, error) {
	digest, err := hashEmailCode(secret, email, scene, code)
	if err != nil {
		return false, err
	}
	codes := repository.NewEmailVerificationCodeRepository(tx)
	now := time.Now()
	record, err := codes.FindActiveForUpdate(email, scene, userID, now)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if record.FailedAttempts >= otpMaxFailedAttempts {
		return false, nil
	}
	if subtle.ConstantTimeCompare([]byte(record.CodeHash), []byte(digest)) != 1 {
		record.FailedAttempts++
		return false, codes.RecordFailedAttempt(record, now, otpMaxFailedAttempts)
	}

	record.ConsumedAt = &now
	if err := codes.Save(record); err != nil {
		return false, err
	}
	return true, nil
}
