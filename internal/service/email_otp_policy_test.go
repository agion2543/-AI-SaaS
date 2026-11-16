package service

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestDevelopmentEmailOTPUsesHMACWithoutLoggingPlaintext(t *testing.T) {
	db := newEmailOTPTestDB(t)
	user := seedEmailOTPTestUser(t, db)
	auth := NewAuthService(&config.Config{AppEnv: "development", JWTSecret: testOTPSecret}, db)

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	email := "Test.User@example.test"
	if err := auth.SendBindEmailCode(user.ID, dto.SendEmailCodeRequest{Email: email}); err != nil {
		t.Fatalf("development mock should issue an email OTP: %v", err)
	}

	var record model.EmailVerificationCode
	if err := db.Where("email = ? AND scene = ?", strings.ToLower(email), "bind_email").First(&record).Error; err != nil {
		t.Fatalf("read synthetic email OTP record: %v", err)
	}
	wantHash, err := hashEmailCode(testOTPSecret, strings.ToLower(email), "bind_email", devFixedEmailCode)
	if err != nil {
		t.Fatal(err)
	}
	if record.Code != legacyOTPRedacted || record.CodeHash != wantHash || len(record.CodeHash) != 64 || record.CodeHash == devFixedEmailCode {
		t.Fatalf("email OTP must persist only a context-bound HMAC and redaction marker: code=%q hash=%q", record.Code, record.CodeHash)
	}
	if record.FailedAttempts != 0 || time.Until(record.ExpiresAt) <= 0 || time.Until(record.ExpiresAt) > otpLifetime {
		t.Fatalf("unexpected OTP limits: attempts=%d expires_at=%s", record.FailedAttempts, record.ExpiresAt)
	}
	if strings.Contains(logs.String(), devFixedEmailCode) || strings.Contains(logs.String(), email) || strings.Contains(logs.String(), "mock-email") {
		t.Fatalf("mock email address/code must not appear in logs: %q", logs.String())
	}
}

func TestEmailOTPHashBindsEmailSceneAndCode(t *testing.T) {
	first, err := hashEmailCode(testOTPSecret, " Person@Example.Test ", "bind_email", "123456")
	if err != nil {
		t.Fatal(err)
	}
	otherEmail, _ := hashEmailCode(testOTPSecret, "other@example.test", "bind_email", "123456")
	otherScene, _ := hashEmailCode(testOTPSecret, "person@example.test", "reset_password", "123456")
	otherCode, _ := hashEmailCode(testOTPSecret, "person@example.test", "bind_email", "654321")
	if len(first) != 64 || first == otherEmail || first == otherScene || first == otherCode {
		t.Fatal("email OTP HMAC must be bound to normalized email, scene, and code")
	}
	if _, err := hashEmailCode("short", "person@example.test", "bind_email", "123456"); !errors.Is(err, errOTPHashKeyTooShort) {
		t.Fatalf("short HMAC key must fail closed; got %v", err)
	}
}

func TestEmailOTPSendLimitsPerEmail(t *testing.T) {
	db := newEmailOTPTestDB(t)
	user := seedEmailOTPTestUser(t, db)
	auth := NewAuthService(&config.Config{AppEnv: "development", JWTSecret: testOTPSecret}, db)
	request := dto.SendEmailCodeRequest{Email: "limit@example.test"}

	if err := auth.SendBindEmailCode(user.ID, request); err != nil {
		t.Fatalf("first synthetic send should pass: %v", err)
	}
	if err := auth.SendBindEmailCode(user.ID, request); !errors.Is(err, errEmailSendRateLimited) {
		t.Fatalf("send inside cooldown should be rejected; got %v", err)
	}
	for sent := 1; sent < otpMaxSendsPerWindow; sent++ {
		if err := db.Model(&model.EmailVerificationCode{}).Where("email = ?", request.Email).Update("created_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
		if err := auth.SendBindEmailCode(user.ID, request); err != nil {
			t.Fatalf("send %d inside hourly allowance should pass: %v", sent+1, err)
		}
	}
	if err := db.Model(&model.EmailVerificationCode{}).Where("email = ?", request.Email).Update("created_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := auth.SendBindEmailCode(user.ID, request); !errors.Is(err, errEmailSendRateLimited) {
		t.Fatalf("sixth send in rolling hour should be rejected; got %v", err)
	}
	var count int64
	if err := db.Model(&model.EmailVerificationCode{}).Where("email = ?", request.Email).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != otpMaxSendsPerWindow {
		t.Fatalf("rate-limited requests must not create records; got %d", count)
	}
}

func TestEmailOTPFailedAttemptsLockAndSuccessfulCodeIsSingleUse(t *testing.T) {
	db := newEmailOTPTestDB(t)
	user := seedEmailOTPTestUser(t, db)
	auth := NewAuthService(&config.Config{AppEnv: "development", JWTSecret: testOTPSecret}, db)
	email := "verify@example.test"
	if err := auth.SendBindEmailCode(user.ID, dto.SendEmailCodeRequest{Email: email}); err != nil {
		t.Fatal(err)
	}

	for attempt := uint(1); attempt <= otpMaxFailedAttempts; attempt++ {
		if _, err := auth.BindEmail(user.ID, dto.BindEmailRequest{Email: email, Code: "000000"}); !errors.Is(err, errEmailInvalidCode) {
			t.Fatalf("failed attempt %d should be rejected: %v", attempt, err)
		}
	}
	var locked model.EmailVerificationCode
	if err := db.Where("email = ? AND scene = ?", email, "bind_email").First(&locked).Error; err != nil {
		t.Fatal(err)
	}
	if locked.FailedAttempts != otpMaxFailedAttempts || locked.ConsumedAt == nil {
		t.Fatalf("fifth failed attempt should lock the email code: attempts=%d consumed=%v", locked.FailedAttempts, locked.ConsumedAt)
	}
	if _, err := auth.BindEmail(user.ID, dto.BindEmailRequest{Email: email, Code: devFixedEmailCode}); !errors.Is(err, errEmailInvalidCode) {
		t.Fatalf("locked code must reject the correct value: %v", err)
	}
	var unchanged model.User
	if err := db.First(&unchanged, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Email != user.Email {
		t.Fatalf("failed attempts must not bind the email; got %q want %q", unchanged.Email, user.Email)
	}

	// A separate address verifies successful consumption and replay prevention.
	goodEmail := "single-use@example.test"
	if err := auth.SendBindEmailCode(user.ID, dto.SendEmailCodeRequest{Email: goodEmail}); err != nil {
		t.Fatal(err)
	}
	bound, err := auth.BindEmail(user.ID, dto.BindEmailRequest{Email: goodEmail, Code: devFixedEmailCode})
	if err != nil {
		t.Fatalf("first correct code use should bind email: %v", err)
	}
	if bound.Email != goodEmail {
		t.Fatalf("bound email=%q, want %q", bound.Email, goodEmail)
	}
	if _, err := auth.BindEmail(user.ID, dto.BindEmailRequest{Email: goodEmail, Code: devFixedEmailCode}); !errors.Is(err, errEmailInvalidCode) {
		t.Fatalf("consumed code must not be reusable: %v", err)
	}
}

func TestEmailOTPExpiredCodeCannotBind(t *testing.T) {
	db := newEmailOTPTestDB(t)
	user := seedEmailOTPTestUser(t, db)
	auth := NewAuthService(&config.Config{AppEnv: "development", JWTSecret: testOTPSecret}, db)
	email := "expired@example.test"
	if err := auth.SendBindEmailCode(user.ID, dto.SendEmailCodeRequest{Email: email}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.EmailVerificationCode{}).Where("email = ?", email).Update("expires_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := auth.BindEmail(user.ID, dto.BindEmailRequest{Email: email, Code: devFixedEmailCode}); !errors.Is(err, errEmailInvalidCode) {
		t.Fatalf("expired code must be rejected: %v", err)
	}
}

func TestEmailOTPRequestsFailClosedOutsideDevelopment(t *testing.T) {
	for _, appEnv := range []string{"production", "staging"} {
		t.Run(appEnv, func(t *testing.T) {
			db := newEmailOTPTestDB(t)
			user := seedEmailOTPTestUser(t, db)
			auth := NewAuthService(&config.Config{AppEnv: appEnv, JWTSecret: testOTPSecret}, db)
			if err := auth.SendBindEmailCode(user.ID, dto.SendEmailCodeRequest{Email: "blocked@example.test"}); !errors.Is(err, errEmailProviderNotConfigured) {
				t.Fatalf("without a real email provider, %s must reject simulated sends; got %v", appEnv, err)
			}
			var count int64
			if err := db.Model(&model.EmailVerificationCode{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("blocked send must not create OTP records; got %d", count)
			}
		})
	}
}

func newEmailOTPTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.EmailVerificationCode{}); err != nil {
		t.Fatalf("migrate synthetic email OTP schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQLite connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func seedEmailOTPTestUser(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	user := &model.User{
		UUID:         uuid.NewString(),
		Email:        newPendingEmail(),
		Phone:        "13800138091",
		Role:         "customer",
		PasswordHash: "synthetic-test-hash",
		DisplayName:  "synthetic email OTP user",
		Status:       "active",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed synthetic email OTP user: %v", err)
	}
	return user
}
