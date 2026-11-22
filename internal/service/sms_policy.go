package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
	"gorm.io/gorm"
)

const (
	otpLifetime          = 10 * time.Minute
	otpSendCooldown      = time.Minute
	otpSendWindow        = time.Hour
	otpMaxSendsPerWindow = 5
	otpMaxFailedAttempts = 5
	minimumOTPKeyLength  = 32
	legacyOTPRedacted    = "REDACTED"
)

var (
	errSMSProviderNotConfigured = errors.New("短信服务尚未配置，暂不可发送验证码")
	errSMSSendRateLimited       = errors.New("验证码发送过于频繁，请稍后再试")
	errSMSInvalidCode           = errors.New("验证码错误或已过期")
	errOTPHashKeyTooShort       = errors.New("JWT_SECRET must contain at least 32 bytes for safe verification code storage")
)

func isDevelopmentEnvironment(appEnv string) bool {
	return strings.EqualFold(strings.TrimSpace(appEnv), "development")
}

func generateMockSMSCode(appEnv string) (string, error) {
	if !isDevelopmentEnvironment(appEnv) {
		return "", errSMSProviderNotConfigured
	}
	return devFixedSMSCode, nil
}

// hashSMSCode binds the digest to its phone and purpose so a code cannot be
// replayed against another account or verification flow.
func hashSMSCode(secret, phone, scene, code string) (string, error) {
	if len(secret) < minimumOTPKeyLength {
		return "", errOTPHashKeyTooShort
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("sms-otp-v1\x00"))
	_, _ = mac.Write([]byte(phone))
	_, _ = mac.Write([]byte{'\x00'})
	_, _ = mac.Write([]byte(scene))
	_, _ = mac.Write([]byte{'\x00'})
	_, _ = mac.Write([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func issueSMSVerificationCode(db *gorm.DB, appEnv, secret, phone, scene string, userID *uint) error {
	code, err := generateMockSMSCode(appEnv)
	if err != nil {
		return err
	}
	digest, err := hashSMSCode(secret, phone, scene, code)
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		codes := repository.NewVerificationCodeRepository(tx)
		if err := codes.LockPhone(phone); err != nil {
			return err
		}

		now := time.Now()
		hourCount, err := codes.CountIssuedSince(phone, now.Add(-otpSendWindow))
		if err != nil {
			return err
		}
		if hourCount >= otpMaxSendsPerWindow {
			return errSMSSendRateLimited
		}
		minuteCount, err := codes.CountIssuedSince(phone, now.Add(-otpSendCooldown))
		if err != nil {
			return err
		}
		if minuteCount > 0 {
			return errSMSSendRateLimited
		}
		if err := codes.InvalidateActive(phone, scene); err != nil {
			return err
		}

		record := &model.VerificationCode{
			Phone:          phone,
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

func consumeSMSVerificationCode(db *gorm.DB, secret, phone, scene, code string, userID *uint) error {
	valid := false
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		valid, err = consumeSMSVerificationCodeInTransaction(tx, secret, phone, scene, code, userID)
		return err
	})
	if err != nil {
		return err
	}
	if !valid {
		return errSMSInvalidCode
	}
	return nil
}

// consumeSMSVerificationCodeInTransaction commits failed-attempt updates when
// the caller returns nil for an invalid code; successful consumption can share
// the caller's transaction with the protected account change.
func consumeSMSVerificationCodeInTransaction(tx *gorm.DB, secret, phone, scene, code string, userID *uint) (bool, error) {
	digest, err := hashSMSCode(secret, phone, scene, code)
	if err != nil {
		return false, err
	}
	codes := repository.NewVerificationCodeRepository(tx)
	now := time.Now()
	record, err := codes.FindActiveForUpdate(phone, scene, userID, now)
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
