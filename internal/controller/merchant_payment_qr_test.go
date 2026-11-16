package controller

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMerchantQRCodeStaysPrivateUntilExplicitSaveAndAuditIsPIISafe(t *testing.T) {
	ctl, db, store, merchantID := newMerchantQRTestController(t)
	oldURL := "https://merchant.example.invalid/uploads/merchant-payment/old_qr.png"
	oldPath := filepath.Join(store.publicDir, "old_qr.png")
	if err := os.MkdirAll(store.publicDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("synthetic-old-configured-file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.MerchantPaymentConfig{
		MerchantID: merchantID, Channel: "alipay", Mode: "direct", AccountName: "old synthetic account",
		AccountNo: "synthetic-old-account-number", AlipayQRCode: oldURL, Status: "enabled", AuditStatus: "approved",
	}).Error; err != nil {
		t.Fatalf("create synthetic prior payment config: %v", err)
	}

	uploadResponse, uploadID := postMerchantQRCodeUpload(t, ctl, merchantID, "alipay", "synthetic-qr.png", syntheticMerchantQRCode(t, 16, 16))
	if uploadResponse.Code != http.StatusOK || uploadID == "" {
		t.Fatalf("synthetic upload failed: status=%d body=%s", uploadResponse.Code, uploadResponse.Body.String())
	}
	if strings.Contains(uploadResponse.Body.String(), `"url"`) {
		t.Fatalf("staging response exposed a public URL: %s", uploadResponse.Body.String())
	}
	if files, _ := filepath.Glob(filepath.Join(store.publicDir, "merchant_qr_*")); len(files) != 0 {
		t.Fatalf("staging created public QR files before explicit save: %v", files)
	}
	stagedFiles, err := filepath.Glob(filepath.Join(store.stageDir, "merchant-qr-stage-*"))
	if err != nil || len(stagedFiles) != 1 {
		t.Fatalf("expected one private staged QR file, files=%v err=%v", stagedFiles, err)
	}
	stagedInfo, err := os.Stat(stagedFiles[0])
	if err != nil || stagedInfo.Mode().Perm() != 0600 {
		t.Fatalf("staged QR permissions=%v err=%v, want 0600", stagedInfo, err)
	}
	stageDirInfo, err := os.Stat(store.stageDir)
	if err != nil || stageDirInfo.Mode().Perm() != 0700 {
		t.Fatalf("private staging directory permissions=%v err=%v, want 0700", stageDirInfo, err)
	}

	response := saveMerchantQRConfig(t, ctl, merchantID, saveMerchantQRRequest(oldURL, uploadID, ""))
	if response.Code != http.StatusOK {
		t.Fatalf("synthetic config save failed: status=%d body=%s", response.Code, response.Body.String())
	}
	var saved model.MerchantPaymentConfig
	if err := db.Where("merchant_id = ?", merchantID).First(&saved).Error; err != nil {
		t.Fatalf("load saved synthetic payment config: %v", err)
	}
	const expectedPrefix = "https://merchant.example.invalid/uploads/merchant-payment/"
	if !strings.HasPrefix(saved.AlipayQRCode, expectedPrefix) || saved.AlipayQRCode == oldURL {
		t.Fatalf("saved QR URL did not use configured APP_URL and finalized file: %q", saved.AlipayQRCode)
	}
	if saved.Status != "disabled" || saved.AuditStatus != "pending" || saved.Mode != "direct" {
		t.Fatalf("save changed existing manual-review/direct-receipt semantics: %+v", saved)
	}
	newFile := filepath.Join(store.publicDir, filepath.Base(saved.AlipayQRCode))
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("new finalized public QR file is missing: %v", err)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("successful replacement deleted the prior configured file: %v", err)
	}
	stagedFiles, err = filepath.Glob(filepath.Join(store.stageDir, "merchant-qr-stage-*"))
	if err != nil || len(stagedFiles) != 0 {
		t.Fatalf("successful save retained staged files: files=%v err=%v", stagedFiles, err)
	}

	var audits []model.AuditLog
	if err := db.Where("action = ?", "merchant_payment_config_save").Find(&audits).Error; err != nil || len(audits) != 1 {
		t.Fatalf("audit rows=%d err=%v, want exactly one", len(audits), err)
	}
	audit := audits[0]
	if audit.TargetType != "merchant_payment_config" || audit.TargetID != saved.ID || audit.MerchantID == nil || *audit.MerchantID != merchantID {
		t.Fatalf("audit target binding is incorrect: %+v", audit)
	}
	if audit.TargetName != "" || audit.IP != "" || audit.UserAgent != "" {
		t.Fatalf("audit contains unnecessary identifying fields: %+v", audit)
	}
	var detail map[string]string
	if err := json.Unmarshal([]byte(audit.Detail), &detail); err != nil {
		t.Fatalf("decode minimal audit detail %q: %v", audit.Detail, err)
	}
	if len(detail) != 1 || detail["purpose"] != "保存商家收款配置" {
		t.Fatalf("audit detail is not purpose-only: %v", detail)
	}
	encoded, _ := json.Marshal(audit)
	for _, privateValue := range []string{"synthetic-old-account-number", "synthetic-account-name", "synthetic-account-number", "15500002222", "synthetic-private-remark"} {
		if strings.Contains(string(encoded), privateValue) {
			t.Errorf("audit leaked payment PII %q: %s", privateValue, encoded)
		}
	}
	if _, err := store.ReserveUploads(merchantID, map[string]string{"alipay": uploadID}); err == nil {
		t.Fatal("successfully consumed staged upload ID was reusable")
	}
}

func TestMerchantQRCodeConfigFailureRollsBackAndCompensatesFile(t *testing.T) {
	ctl, db, store, merchantID := newMerchantQRTestController(t)
	oldURL := "https://merchant.example.invalid/uploads/merchant-payment/config-failure-old.png"
	if err := db.Create(&model.MerchantPaymentConfig{MerchantID: merchantID, Channel: "alipay", Mode: "direct", AccountName: "synthetic", AccountNo: "old", AlipayQRCode: oldURL}).Error; err != nil {
		t.Fatal(err)
	}
	_, uploadID := postMerchantQRCodeUpload(t, ctl, merchantID, "alipay", "synthetic-qr.png", syntheticMerchantQRCode(t, 12, 12))
	if err := db.Exec("CREATE TRIGGER synthetic_merchant_qr_config_failure BEFORE UPDATE ON merchant_payment_configs BEGIN SELECT RAISE(ABORT, 'synthetic config failure'); END").Error; err != nil {
		t.Fatalf("install synthetic config failure: %v", err)
	}
	response := saveMerchantQRConfig(t, ctl, merchantID, saveMerchantQRRequest(oldURL, uploadID, ""))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("config failure status=%d body=%s", response.Code, response.Body.String())
	}
	assertMerchantQRFailurePreserved(t, db, store, merchantID, oldURL)
	var audits int64
	if err := db.Model(&model.AuditLog{}).Count(&audits).Error; err != nil || audits != 0 {
		t.Fatalf("config failure left audit rows=%d err=%v", audits, err)
	}
}

func TestMerchantQRCodeAuditFailureRollsBackConfigAndCompensatesFile(t *testing.T) {
	ctl, db, store, merchantID := newMerchantQRTestController(t)
	oldURL := "https://merchant.example.invalid/uploads/merchant-payment/audit-failure-old.png"
	if err := db.Create(&model.MerchantPaymentConfig{MerchantID: merchantID, Channel: "alipay", Mode: "direct", AccountName: "synthetic", AccountNo: "old", AlipayQRCode: oldURL}).Error; err != nil {
		t.Fatal(err)
	}
	_, uploadID := postMerchantQRCodeUpload(t, ctl, merchantID, "alipay", "synthetic-qr.png", syntheticMerchantQRCode(t, 12, 12))
	if err := db.Exec("CREATE TRIGGER synthetic_merchant_qr_audit_failure BEFORE INSERT ON audit_logs BEGIN SELECT RAISE(ABORT, 'synthetic audit failure'); END").Error; err != nil {
		t.Fatalf("install synthetic audit failure: %v", err)
	}
	response := saveMerchantQRConfig(t, ctl, merchantID, saveMerchantQRRequest(oldURL, uploadID, ""))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("audit failure status=%d body=%s", response.Code, response.Body.String())
	}
	assertMerchantQRFailurePreserved(t, db, store, merchantID, oldURL)
	var audits int64
	if err := db.Model(&model.AuditLog{}).Count(&audits).Error; err != nil || audits != 0 {
		t.Fatalf("audit failure left rows=%d err=%v", audits, err)
	}
}

func TestMerchantQRCodeFileFailurePreservesPriorConfig(t *testing.T) {
	ctl, db, store, merchantID := newMerchantQRTestController(t)
	oldURL := "https://merchant.example.invalid/uploads/merchant-payment/file-failure-old.png"
	if err := db.Create(&model.MerchantPaymentConfig{MerchantID: merchantID, Channel: "alipay", Mode: "direct", AccountName: "synthetic", AccountNo: "old", AlipayQRCode: oldURL}).Error; err != nil {
		t.Fatal(err)
	}
	_, uploadID := postMerchantQRCodeUpload(t, ctl, merchantID, "alipay", "synthetic-qr.png", syntheticMerchantQRCode(t, 12, 12))
	store.rename = func(string, string) error { return os.ErrPermission }
	response := saveMerchantQRConfig(t, ctl, merchantID, saveMerchantQRRequest(oldURL, uploadID, ""))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("file finalization failure status=%d body=%s", response.Code, response.Body.String())
	}
	assertMerchantQRFailurePreserved(t, db, store, merchantID, oldURL)
	var audits int64
	if err := db.Model(&model.AuditLog{}).Count(&audits).Error; err != nil || audits != 0 {
		t.Fatalf("file failure left audit rows=%d err=%v", audits, err)
	}
}

func TestMerchantQRCodeRejectsCrossMerchantChannelAndExpiredUploads(t *testing.T) {
	t.Run("cross merchant", func(t *testing.T) {
		ctl, db, store, merchantID := newMerchantQRTestController(t)
		otherMerchantID := createSyntheticMerchant(t, db, "synthetic merchant two")
		_, uploadID := postMerchantQRCodeUpload(t, ctl, merchantID, "alipay", "synthetic-qr.png", syntheticMerchantQRCode(t, 12, 12))
		response := saveMerchantQRConfig(t, ctl, otherMerchantID, saveMerchantQRRequest("", uploadID, ""))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("cross-merchant staged upload status=%d body=%s", response.Code, response.Body.String())
		}
		assertNoPublicMerchantQR(t, store)
	})

	t.Run("cross channel", func(t *testing.T) {
		ctl, db, store, merchantID := newMerchantQRTestController(t)
		wechatURL := "https://merchant.example.invalid/synthetic-wechat-link"
		if err := db.Create(&model.MerchantPaymentConfig{MerchantID: merchantID, Channel: "wechat", Mode: "direct", AccountName: "synthetic", AccountNo: "synthetic", WechatQRCode: wechatURL}).Error; err != nil {
			t.Fatal(err)
		}
		_, uploadID := postMerchantQRCodeUpload(t, ctl, merchantID, "alipay", "synthetic-qr.png", syntheticMerchantQRCode(t, 12, 12))
		response := saveMerchantQRConfig(t, ctl, merchantID, saveMerchantQRRequest("", "", uploadID))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("cross-channel staged upload status=%d body=%s", response.Code, response.Body.String())
		}
		assertNoPublicMerchantQR(t, store)
	})

	t.Run("expired", func(t *testing.T) {
		ctl, db, store, merchantID := newMerchantQRTestController(t)
		oldURL := "https://merchant.example.invalid/synthetic-expired-old"
		if err := db.Create(&model.MerchantPaymentConfig{MerchantID: merchantID, Channel: "alipay", Mode: "direct", AccountName: "synthetic", AccountNo: "synthetic", AlipayQRCode: oldURL}).Error; err != nil {
			t.Fatal(err)
		}
		base := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
		store.now = func() time.Time { return base }
		_, uploadID := postMerchantQRCodeUpload(t, ctl, merchantID, "alipay", "synthetic-qr.png", syntheticMerchantQRCode(t, 12, 12))
		store.now = func() time.Time { return base.Add(merchantQRCodeTTL + time.Second) }
		response := saveMerchantQRConfig(t, ctl, merchantID, saveMerchantQRRequest(oldURL, uploadID, ""))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("expired staged upload status=%d body=%s", response.Code, response.Body.String())
		}
		assertMerchantQRFailurePreserved(t, db, store, merchantID, oldURL)
	})
}

func TestMerchantQRCodeRejectsMismatchedAndInvalidImageBytes(t *testing.T) {
	ctl, _, store, merchantID := newMerchantQRTestController(t)
	cases := []struct {
		name     string
		filename string
		content  []byte
	}{
		{name: "extension mismatch", filename: "synthetic-qr.jpg", content: syntheticMerchantQRCode(t, 10, 10)},
		{name: "non image bytes", filename: "synthetic-qr.png", content: []byte("synthetic non-image bytes")},
		{name: "truncated image", filename: "synthetic-qr.png", content: []byte{137, 'P', 'N', 'G', 13, 10, 26, 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, uploadID := postMerchantQRCodeUpload(t, ctl, merchantID, "alipay", tc.filename, tc.content)
			if response.Code != http.StatusBadRequest || uploadID != "" {
				t.Fatalf("invalid synthetic upload status=%d uploadID=%q body=%s", response.Code, uploadID, response.Body.String())
			}
			assertNoMerchantStageFiles(t, store)
			assertNoPublicMerchantQR(t, store)
		})
	}
}

func TestMerchantQRCodeRejectsOversizedAndExcessiveDimensions(t *testing.T) {
	root := t.TempDir()
	store := newMerchantPaymentQRStore(filepath.Join(root, "private"), filepath.Join(root, "public"))
	if _, err := store.Stage(1, "alipay", ".png", bytes.NewReader(bytes.Repeat([]byte{'x'}, merchantQRCodeMaxSize+1))); err == nil {
		t.Fatal("oversized synthetic QR was accepted")
	}
	if _, err := store.Stage(1, "alipay", ".png", bytes.NewReader(syntheticMerchantQRCode(t, merchantQRCodeMaxDimension+1, 1))); err == nil {
		t.Fatal("synthetic QR exceeding maximum dimension was accepted")
	}
	assertNoMerchantStageFiles(t, store)
}

func newMerchantQRTestController(t *testing.T) (*MerchantController, *gorm.DB, *merchantPaymentQRStore, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic merchant QR database: %v", err)
	}
	if err := db.AutoMigrate(&model.MerchantPlan{}, &model.Merchant{}, &model.MerchantPaymentConfig{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic merchant QR schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic merchant QR database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	merchantID := createSyntheticMerchant(t, db, "synthetic merchant")
	root := t.TempDir()
	store := newMerchantPaymentQRStore(filepath.Join(root, "private-stage"), filepath.Join(root, "public-uploads"))
	merchantService := service.NewMerchantService(&config.Config{}, db)
	ctl := &MerchantController{merchant: merchantService, paymentQR: store, appURL: "https://merchant.example.invalid"}
	return ctl, db, store, merchantID
}

func createSyntheticMerchant(t *testing.T, db *gorm.DB, name string) uint {
	t.Helper()
	merchant := model.Merchant{Name: name, ContactPhone: "15500001111", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	return merchant.ID
}

func postMerchantQRCodeUpload(t *testing.T, ctl *MerchantController, merchantID uint, channel, filename string, content []byte) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("channel", channel); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	response := invokeMerchantQRHandler(t, ctl.UploadPaymentQRCode, http.MethodPost, "/api/v1/merchant/payment-qrcode/upload", body.Bytes(), writer.FormDataContentType(), merchantID)
	var payload struct {
		Data struct {
			UploadID string `json:"upload_id"`
			Channel  string `json:"channel"`
		} `json:"data"`
	}
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode synthetic merchant upload response: %v (%s)", err, response.Body.String())
		}
		if payload.Data.Channel != channel {
			t.Fatalf("upload channel=%q want %q", payload.Data.Channel, channel)
		}
	}
	return response, payload.Data.UploadID
}

func saveMerchantQRRequest(alipayURL, alipayUploadID, wechatUploadID string) saveMerchantPaymentConfigWithQRUploads {
	return saveMerchantPaymentConfigWithQRUploads{
		SaveMerchantPaymentConfigRequest: dto.SaveMerchantPaymentConfigRequest{
			Channel: "alipay", Mode: "direct", AccountName: "synthetic-account-name", AccountNo: "synthetic-account-number",
			AlipayQRCode: alipayURL, WechatQRCode: "", ContactPhone: "15500002222", Remark: "synthetic-private-remark",
		},
		AlipayQRUploadID: alipayUploadID,
		WechatQRUploadID: wechatUploadID,
	}
}

func saveMerchantQRConfig(t *testing.T, ctl *MerchantController, merchantID uint, body saveMerchantPaymentConfigWithQRUploads) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal synthetic merchant config: %v", err)
	}
	return invokeMerchantQRHandler(t, ctl.SavePaymentConfig, http.MethodPut, "/api/v1/merchant/payment-config", encoded, "application/json", merchantID)
}

func invokeMerchantQRHandler(t *testing.T, handler gin.HandlerFunc, method, path string, body []byte, contentType string, merchantID uint) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	ctx.Request.Host = "attacker.invalid"
	ctx.Request.RemoteAddr = "127.0.0.1:12345"
	if contentType != "" {
		ctx.Request.Header.Set("Content-Type", contentType)
	}
	ctx.Request.Header.Set("User-Agent", "synthetic-test-agent")
	ctx.Set("merchant_id", merchantID)
	ctx.Set("user_id", uint(901))
	ctx.Set("role", "merchant")
	handler(ctx)
	return response
}

func syntheticMerchantQRCode(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.White)
			}
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode synthetic merchant QR: %v", err)
	}
	return buffer.Bytes()
}

func assertMerchantQRFailurePreserved(t *testing.T, db *gorm.DB, store *merchantPaymentQRStore, merchantID uint, oldURL string) {
	t.Helper()
	var saved model.MerchantPaymentConfig
	if err := db.Where("merchant_id = ?", merchantID).First(&saved).Error; err != nil {
		t.Fatalf("load prior config after failure: %v", err)
	}
	if saved.AlipayQRCode != oldURL {
		t.Fatalf("failed save replaced prior QR: got %q want %q", saved.AlipayQRCode, oldURL)
	}
	assertNoPublicMerchantQR(t, store)
	assertNoMerchantStageFiles(t, store)
}

func assertNoPublicMerchantQR(t *testing.T, store *merchantPaymentQRStore) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(store.publicDir, "merchant_qr_*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("unexpected newly finalized public QR files: files=%v err=%v", files, err)
	}
}

func assertNoMerchantStageFiles(t *testing.T, store *merchantPaymentQRStore) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(store.stageDir, "merchant-qr-stage-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("unexpected private staging files: files=%v err=%v", files, err)
	}
}
