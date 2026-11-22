package service

import (
	"bytes"
	"errors"
	"fmt"
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

const testOTPSecret = "synthetic-otp-hmac-secret-32-bytes-minimum"

func TestDevelopmentSMSMockStillIssuesAndAcceptsCodes(t *testing.T) {
	db := newSMSTestDB(t)
	customer, _ := seedSMSTestUsers(t, db)
	cfg := &config.Config{AppEnv: "development", JWTSecret: testOTPSecret}
	auth := NewAuthService(cfg, db)
	merchant := NewMerchantService(cfg, db)

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	if err := auth.SendRegisterCode(dto.SendSMSCodeRequest{Phone: "13800138003"}); err != nil {
		t.Fatalf("development customer registration mock should send: %v", err)
	}
	if err := auth.SendLoginCode(dto.SendLoginCodeRequest{Account: customer.Phone}); err != nil {
		t.Fatalf("development customer login mock should send: %v", err)
	}
	if err := merchant.SendRegisterCode(dto.SendSMSCodeRequest{Phone: "13800138004"}); err != nil {
		t.Fatalf("development merchant registration mock should send: %v", err)
	}

	var records []model.VerificationCode
	if err := db.Order("id ASC").Find(&records).Error; err != nil {
		t.Fatalf("read synthetic verification records: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("development should persist mock codes for all three send flows; got %d records", len(records))
	}
	for _, record := range records {
		want, err := hashSMSCode(testOTPSecret, record.Phone, record.Scene, devFixedSMSCode)
		if err != nil {
			t.Fatal(err)
		}
		if record.CodeHash != want || len(record.CodeHash) != 64 || record.CodeHash == devFixedSMSCode {
			t.Fatal("development flow did not persist a full HMAC digest instead of the plaintext code")
		}
		if record.Code != legacyOTPRedacted {
			t.Fatalf("legacy compatibility column must contain only the redaction marker, got %q", record.Code)
		}
	}
	if err := auth.consumeVerificationCode("13800138003", "register", devFixedSMSCode, nil); err != nil {
		t.Fatalf("development customer registration mock should remain accepted: %v", err)
	}
	if err := merchant.consumeMerchantVerificationCode("13800138004", "merchant_register", devFixedSMSCode, nil); err != nil {
		t.Fatalf("development merchant registration mock should remain accepted: %v", err)
	}
	if err := auth.consumeVerificationCode("13800138003", "register", devFixedSMSCode, nil); !errors.Is(err, errSMSInvalidCode) {
		t.Fatalf("a consumed customer code must not be reusable; got %v", err)
	}
	if strings.Contains(logs.String(), devFixedSMSCode) || strings.Contains(logs.String(), "[mock-sms]") {
		t.Fatalf("SMS code or mock-SMS code log unexpectedly present: %q", logs.String())
	}
}

func TestOTPHashBindsPhoneAndScene(t *testing.T) {
	first, err := hashSMSCode(testOTPSecret, "13800138001", "login", "123456")
	if err != nil {
		t.Fatal(err)
	}
	otherPhone, _ := hashSMSCode(testOTPSecret, "13800138002", "login", "123456")
	otherScene, _ := hashSMSCode(testOTPSecret, "13800138001", "register", "123456")
	if len(first) != 64 || first == otherPhone || first == otherScene {
		t.Fatal("OTP digest must be a full HMAC bound to its phone and scene")
	}
}

func TestSMSSendLimitsPerPhone(t *testing.T) {
	db := newSMSTestDB(t)
	auth := NewAuthService(&config.Config{AppEnv: "development", JWTSecret: testOTPSecret}, db)
	phone := "13800138021"
	request := dto.SendSMSCodeRequest{Phone: phone}
	if err := auth.SendRegisterCode(request); err != nil {
		t.Fatalf("first synthetic send should pass: %v", err)
	}
	if err := auth.SendRegisterCode(request); !errors.Is(err, errSMSSendRateLimited) {
		t.Fatalf("send inside cooldown should be rejected; got %v", err)
	}

	for sent := 1; sent < otpMaxSendsPerWindow; sent++ {
		if err := db.Model(&model.VerificationCode{}).Where("phone = ?", phone).Update("created_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
		if err := auth.SendRegisterCode(request); err != nil {
			t.Fatalf("send %d inside hourly allowance should pass: %v", sent+1, err)
		}
	}
	if err := db.Model(&model.VerificationCode{}).Where("phone = ?", phone).Update("created_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := auth.SendRegisterCode(request); !errors.Is(err, errSMSSendRateLimited) {
		t.Fatalf("sixth send in the rolling hour should be rejected; got %v", err)
	}
	var count int64
	if err := db.Model(&model.VerificationCode{}).Where("phone = ?", phone).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != otpMaxSendsPerWindow {
		t.Fatalf("rate-limited requests must not create records; got %d", count)
	}
}

func TestSMSFailedAttemptsLockCodeAndSuccessIsSingleUse(t *testing.T) {
	db := newSMSTestDB(t)
	auth := NewAuthService(&config.Config{AppEnv: "development", JWTSecret: testOTPSecret}, db)
	phone := "13800138022"
	if err := auth.SendRegisterCode(dto.SendSMSCodeRequest{Phone: phone}); err != nil {
		t.Fatal(err)
	}
	for attempt := uint(1); attempt <= otpMaxFailedAttempts; attempt++ {
		if err := auth.consumeVerificationCode(phone, "register", "000000", nil); !errors.Is(err, errSMSInvalidCode) {
			t.Fatalf("failed attempt %d should be rejected; got %v", attempt, err)
		}
	}
	var record model.VerificationCode
	if err := db.Where("phone = ? AND scene = ?", phone, "register").First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.FailedAttempts != otpMaxFailedAttempts || record.ConsumedAt == nil {
		t.Fatalf("fifth failed attempt should lock the code; attempts=%d consumed=%v", record.FailedAttempts, record.ConsumedAt)
	}
	if err := auth.consumeVerificationCode(phone, "register", devFixedSMSCode, nil); !errors.Is(err, errSMSInvalidCode) {
		t.Fatalf("locked code must reject the correct value; got %v", err)
	}

	phone = "13800138023"
	if err := auth.SendRegisterCode(dto.SendSMSCodeRequest{Phone: phone}); err != nil {
		t.Fatal(err)
	}
	if err := auth.consumeVerificationCode(phone, "register", devFixedSMSCode, nil); err != nil {
		t.Fatalf("first correct attempt should consume the code: %v", err)
	}
	if err := auth.consumeVerificationCode(phone, "register", devFixedSMSCode, nil); !errors.Is(err, errSMSInvalidCode) {
		t.Fatalf("successfully consumed code must not be replayable; got %v", err)
	}
}

func TestSMSBindingCodeIsScopedToIssuingUser(t *testing.T) {
	db := newSMSTestDB(t)
	customer, merchantAdmin := seedSMSTestUsers(t, db)
	auth := NewAuthService(&config.Config{AppEnv: "development", JWTSecret: testOTPSecret}, db)
	targetPhone := "13800138009"
	if err := auth.SendBindPhoneCode(customer.ID, dto.SendSMSCodeRequest{Phone: targetPhone}); err != nil {
		t.Fatalf("issue synthetic bind code: %v", err)
	}

	if _, err := auth.BindPhone(merchantAdmin.ID, dto.BindPhoneRequest{Phone: targetPhone, Code: devFixedSMSCode}); !errors.Is(err, errSMSInvalidCode) {
		t.Fatalf("another user must not consume the binding code; got %v", err)
	}
	var record model.VerificationCode
	if err := db.Where("phone = ? AND scene = ?", targetPhone, "bind_phone").First(&record).Error; err != nil {
		t.Fatalf("read synthetic bind code: %v", err)
	}
	if record.UserID == nil || *record.UserID != customer.ID || record.ConsumedAt != nil || record.FailedAttempts != 0 {
		t.Fatalf("cross-user attempt changed or mis-scoped OTP record: user_id=%v consumed=%v failed=%d", record.UserID, record.ConsumedAt, record.FailedAttempts)
	}

	bound, err := auth.BindPhone(customer.ID, dto.BindPhoneRequest{Phone: targetPhone, Code: devFixedSMSCode})
	if err != nil {
		t.Fatalf("issuing user should still be able to bind phone: %v", err)
	}
	if bound.Phone != targetPhone {
		t.Fatalf("bound phone=%q, want %q", bound.Phone, targetPhone)
	}
}

func TestSMSExpiredCodeCannotBeConsumed(t *testing.T) {
	db := newSMSTestDB(t)
	auth := NewAuthService(&config.Config{AppEnv: "development", JWTSecret: testOTPSecret}, db)
	phone := "13800138025"
	if err := auth.SendRegisterCode(dto.SendSMSCodeRequest{Phone: phone}); err != nil {
		t.Fatalf("send synthetic registration code: %v", err)
	}
	if err := db.Model(&model.VerificationCode{}).
		Where("phone = ? AND scene = ?", phone, "register").
		Update("expires_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatalf("expire synthetic code: %v", err)
	}
	if err := auth.consumeVerificationCode(phone, "register", devFixedSMSCode, nil); !errors.Is(err, errSMSInvalidCode) {
		t.Fatalf("expired SMS code must be rejected; got %v", err)
	}
	var record model.VerificationCode
	if err := db.Where("phone = ? AND scene = ?", phone, "register").First(&record).Error; err != nil {
		t.Fatalf("read synthetic code record: %v", err)
	}
	if record.ConsumedAt != nil || record.FailedAttempts != 0 {
		t.Fatalf("expired code should not be consumed or counted as a wrong attempt: consumed=%v failed_attempts=%d", record.ConsumedAt, record.FailedAttempts)
	}
}

func TestSMSRequestsFailClosedOutsideDevelopment(t *testing.T) {
	sendCases := []struct {
		name string
		send func(*AuthService, *MerchantService, *model.User, *model.User) error
	}{
		{
			name: "customer registration",
			send: func(auth *AuthService, _ *MerchantService, _, _ *model.User) error {
				return auth.SendRegisterCode(dto.SendSMSCodeRequest{Phone: "13800138003"})
			},
		},
		{
			name: "customer login",
			send: func(auth *AuthService, _ *MerchantService, customer, _ *model.User) error {
				return auth.SendLoginCode(dto.SendLoginCodeRequest{Account: customer.Phone})
			},
		},
		{
			name: "customer password reset",
			send: func(auth *AuthService, _ *MerchantService, customer, _ *model.User) error {
				return auth.SendPasswordResetCode(dto.SendSMSCodeRequest{Phone: customer.Phone})
			},
		},
		{
			name: "customer phone binding",
			send: func(auth *AuthService, _ *MerchantService, customer, _ *model.User) error {
				return auth.SendBindPhoneCode(customer.ID, dto.SendSMSCodeRequest{Phone: "13800138005"})
			},
		},
		{
			name: "merchant registration",
			send: func(_ *AuthService, merchant *MerchantService, _, _ *model.User) error {
				return merchant.SendRegisterCode(dto.SendSMSCodeRequest{Phone: "13800138006"})
			},
		},
		{
			name: "merchant password reset",
			send: func(_ *AuthService, merchant *MerchantService, _, merchantAdmin *model.User) error {
				return merchant.SendPasswordResetCode(dto.MerchantSendPasswordResetCodeRequest{Phone: merchantAdmin.Phone})
			},
		},
	}

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	for _, appEnv := range []string{"production", "staging"} {
		for _, testCase := range sendCases {
			t.Run(appEnv+"/"+testCase.name, func(t *testing.T) {
				db := newSMSTestDB(t)
				customer, merchantAdmin := seedSMSTestUsers(t, db)
				cfg := &config.Config{AppEnv: appEnv, JWTSecret: testOTPSecret}
				auth := NewAuthService(cfg, db)
				merchant := NewMerchantService(cfg, db)

				err := testCase.send(auth, merchant, customer, merchantAdmin)
				if !errors.Is(err, errSMSProviderNotConfigured) {
					t.Fatalf("SMS send should fail with the provider-not-configured error; got %v", err)
				}

				var count int64
				if err := db.Model(&model.VerificationCode{}).Count(&count).Error; err != nil {
					t.Fatalf("count synthetic verification records: %v", err)
				}
				if count != 0 {
					t.Fatalf("blocked send must not create or retain verification records; got %d", count)
				}
			})
		}
	}

	if strings.Contains(logs.String(), "[mock-sms]") {
		t.Fatal("non-development SMS requests must not emit mock-code logs")
	}
}

func TestFixedSMSCodeIsNotAcceptedOutsideDevelopment(t *testing.T) {
	for _, appEnv := range []string{"production", "staging"} {
		t.Run(appEnv, func(t *testing.T) {
			db := newSMSTestDB(t)
			cfg := &config.Config{AppEnv: appEnv, JWTSecret: testOTPSecret}
			auth := NewAuthService(cfg, db)
			merchant := NewMerchantService(cfg, db)

			if err := auth.consumeVerificationCode("13800138001", "login", devFixedSMSCode, nil); err == nil {
				t.Fatal("fixed mock code must not authenticate a customer outside development")
			}
			if err := merchant.consumeMerchantVerificationCode("13800138002", "merchant_reset_password", devFixedSMSCode, nil); err == nil {
				t.Fatal("fixed mock code must not authenticate a merchant outside development")
			}
		})
	}
}

func newSMSTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	if err := db.AutoMigrate(&model.MerchantPlan{}, &model.Merchant{}, &model.User{}, &model.VerificationCode{}); err != nil {
		t.Fatalf("migrate synthetic SQLite schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQLite connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func seedSMSTestUsers(t *testing.T, db *gorm.DB) (*model.User, *model.User) {
	t.Helper()
	customer := &model.User{
		UUID:         uuid.NewString(),
		Email:        uuid.NewString() + "@test.local",
		Phone:        "13800138001",
		Role:         "customer",
		PasswordHash: "synthetic-test-hash",
		Status:       "active",
	}
	merchantAdmin := &model.User{
		UUID:         uuid.NewString(),
		Email:        uuid.NewString() + "@test.local",
		Phone:        "13800138002",
		Role:         "merchant_admin",
		PasswordHash: "synthetic-test-hash",
		Status:       "active",
	}
	if err := db.Create(customer).Error; err != nil {
		t.Fatalf("seed synthetic customer: %v", err)
	}
	if err := db.Create(merchantAdmin).Error; err != nil {
		t.Fatalf("seed synthetic merchant admin: %v", err)
	}
	return customer, merchantAdmin
}
