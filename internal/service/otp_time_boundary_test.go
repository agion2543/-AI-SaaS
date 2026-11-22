package service

import (
	"errors"
	"testing"
	"time"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
	"gorm.io/gorm"
)

func TestSMSOTPTimeWindowBoundaries(t *testing.T) {
	db := newSMSTestDB(t)
	codes := repository.NewVerificationCodeRepository(db)
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	minuteBoundary := now.Add(-otpSendCooldown)
	if err := db.Create(&model.VerificationCode{
		BaseModel: model.BaseModel{CreatedAt: minuteBoundary, UpdatedAt: minuteBoundary},
		Phone:     "13800138101", Scene: "register", Code: legacyOTPRedacted,
		CodeHash:  "synthetic",
		ExpiresAt: now.Add(otpLifetime),
	}).Error; err != nil {
		t.Fatal(err)
	}
	assertSMSCount(t, codes, "13800138101", minuteBoundary, 1)
	assertSMSCount(t, codes, "13800138101", minuteBoundary.Add(time.Nanosecond), 0)

	hourBoundary := now.Add(-otpSendWindow)
	if err := db.Create(&model.VerificationCode{
		BaseModel: model.BaseModel{CreatedAt: hourBoundary, UpdatedAt: hourBoundary},
		Phone:     "13800138102", Scene: "register", Code: legacyOTPRedacted,
		CodeHash:  "synthetic",
		ExpiresAt: now.Add(otpLifetime),
	}).Error; err != nil {
		t.Fatal(err)
	}
	assertSMSCount(t, codes, "13800138102", hourBoundary, 1)
	assertSMSCount(t, codes, "13800138102", hourBoundary.Add(time.Nanosecond), 0)

	expiresAt := now
	if err := db.Create(&model.VerificationCode{
		BaseModel: model.BaseModel{CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
		Phone:     "13800138103", Scene: "register", Code: legacyOTPRedacted,
		CodeHash:  "synthetic",
		ExpiresAt: expiresAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := codes.FindActiveForUpdate("13800138103", "register", nil, expiresAt.Add(-time.Nanosecond)); err != nil {
		t.Fatalf("SMS code must remain active just before expiry: %v", err)
	}
	if _, err := codes.FindActiveForUpdate("13800138103", "register", nil, expiresAt); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("SMS code must be expired at the exact expiry instant; got %v", err)
	}
}

func TestEmailOTPTimeWindowBoundaries(t *testing.T) {
	db := newEmailOTPTestDB(t)
	codes := repository.NewEmailVerificationCodeRepository(db)
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	minuteBoundary := now.Add(-otpSendCooldown)
	if err := db.Create(&model.EmailVerificationCode{
		BaseModel: model.BaseModel{CreatedAt: minuteBoundary, UpdatedAt: minuteBoundary},
		Email:     "minute-boundary@example.test", Scene: "bind_email", Code: legacyOTPRedacted,
		CodeHash:  "synthetic",
		ExpiresAt: now.Add(otpLifetime),
	}).Error; err != nil {
		t.Fatal(err)
	}
	assertEmailCount(t, codes, "minute-boundary@example.test", minuteBoundary, 1)
	assertEmailCount(t, codes, "minute-boundary@example.test", minuteBoundary.Add(time.Nanosecond), 0)

	hourBoundary := now.Add(-otpSendWindow)
	if err := db.Create(&model.EmailVerificationCode{
		BaseModel: model.BaseModel{CreatedAt: hourBoundary, UpdatedAt: hourBoundary},
		Email:     "hour-boundary@example.test", Scene: "bind_email", Code: legacyOTPRedacted,
		CodeHash:  "synthetic",
		ExpiresAt: now.Add(otpLifetime),
	}).Error; err != nil {
		t.Fatal(err)
	}
	assertEmailCount(t, codes, "hour-boundary@example.test", hourBoundary, 1)
	assertEmailCount(t, codes, "hour-boundary@example.test", hourBoundary.Add(time.Nanosecond), 0)

	expiresAt := now
	if err := db.Create(&model.EmailVerificationCode{
		BaseModel: model.BaseModel{CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
		Email:     "expiry-boundary@example.test", Scene: "bind_email", Code: legacyOTPRedacted,
		CodeHash:  "synthetic",
		ExpiresAt: expiresAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := codes.FindActiveForUpdate("expiry-boundary@example.test", "bind_email", nil, expiresAt.Add(-time.Nanosecond)); err != nil {
		t.Fatalf("email OTP must remain active just before expiry: %v", err)
	}
	if _, err := codes.FindActiveForUpdate("expiry-boundary@example.test", "bind_email", nil, expiresAt); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("email OTP must be expired at the exact expiry instant; got %v", err)
	}
}

func assertSMSCount(t *testing.T, codes *repository.VerificationCodeRepository, phone string, since time.Time, want int64) {
	t.Helper()
	got, err := codes.CountIssuedSince(phone, since)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("SMS count since %s for %s=%d, want %d", since.Format(time.RFC3339Nano), phone, got, want)
	}
}

func assertEmailCount(t *testing.T, codes *repository.EmailVerificationCodeRepository, email string, since time.Time, want int64) {
	t.Helper()
	got, err := codes.CountIssuedSince(email, since)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("email OTP count since %s for %s=%d, want %d", since.Format(time.RFC3339Nano), email, got, want)
	}
}
