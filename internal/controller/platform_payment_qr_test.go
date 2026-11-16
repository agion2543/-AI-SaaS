package controller

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
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

func TestPlatformQRCodeStagingBindsOwnerChannelAndExpiry(t *testing.T) {
	root := t.TempDir()
	store := newPlatformPaymentQRStore(filepath.Join(root, "private"), filepath.Join(root, "public"))
	base := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return base }

	id, err := store.Stage(41, "alipay", ".png", bytes.NewReader(syntheticPlatformQRCode(t, "png")))
	if err != nil {
		t.Fatalf("stage synthetic QR: %v", err)
	}
	stagedPath := store.uploads[id].StagePath
	info, err := os.Stat(stagedPath)
	if err != nil {
		t.Fatalf("stat private staged QR: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("staged QR permissions=%#o, want 0600", info.Mode().Perm())
	}
	if _, err := store.ReserveUploads(42, map[string]string{"alipay": id}); err == nil {
		t.Fatal("another administrator reserved the staged QR")
	}
	if _, err := store.ReserveUploads(41, map[string]string{"wechat": id}); err == nil {
		t.Fatal("staged Alipay QR was accepted for the WeChat channel")
	}
	refs, err := store.ReserveUploads(41, map[string]string{"alipay": id})
	if err != nil || len(refs) != 1 {
		t.Fatalf("owner could not reserve matching channel upload: refs=%v err=%v", refs, err)
	}
	store.Abort(refs, nil)
	if _, err := os.Stat(stagedPath); !os.IsNotExist(err) {
		t.Fatalf("aborted private staged file still exists: %v", err)
	}

	expiredID, err := store.Stage(41, "wechat", ".jpg", bytes.NewReader(syntheticPlatformQRCode(t, "jpeg")))
	if err != nil {
		t.Fatalf("stage expiring synthetic QR: %v", err)
	}
	expiredPath := store.uploads[expiredID].StagePath
	store.now = func() time.Time { return base.Add(store.ttl + time.Second) }
	if _, err := store.ReserveUploads(41, map[string]string{"wechat": expiredID}); err == nil {
		t.Fatal("expired QR upload was accepted")
	}
	if _, err := os.Stat(expiredPath); !os.IsNotExist(err) {
		t.Fatalf("expired private staged file was not cleaned: %v", err)
	}
}

func TestPlatformQRCodeRejectsMismatchedAndInvalidImageContent(t *testing.T) {
	ctl, _, store := newPlatformQRTestController(t)
	cases := []struct {
		name     string
		filename string
		content  []byte
	}{
		{name: "extension mismatch", filename: "synthetic-qr.jpg", content: syntheticPlatformQRCode(t, "png")},
		{name: "non-image bytes", filename: "synthetic-qr.png", content: []byte("not an image")},
		{name: "truncated image structure", filename: "synthetic-qr.png", content: []byte{137, 'P', 'N', 'G', 13, 10, 26, 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, uploadID := postPlatformQRCodeUploadContent(t, ctl, 81, "alipay", tc.filename, tc.content)
			if response.Code != http.StatusBadRequest || uploadID != "" {
				t.Fatalf("invalid synthetic upload status=%d uploadID=%q response=%s", response.Code, uploadID, response.Body.String())
			}
			assertPlatformQRNoNewFiles(t, store)
		})
	}
}

func TestPlatformQRCodeAcceptsOnlyFullyDecodedSupportedFormats(t *testing.T) {
	root := t.TempDir()
	store := newPlatformPaymentQRStore(filepath.Join(root, "private"), filepath.Join(root, "public"))
	for _, tc := range []struct{ format, extension string }{
		{format: "jpeg", extension: ".jpg"},
		{format: "png", extension: ".png"},
		{format: "webp", extension: ".webp"},
	} {
		t.Run(tc.format, func(t *testing.T) {
			id, err := store.Stage(82, "wechat", tc.extension, bytes.NewReader(syntheticPlatformQRCode(t, tc.format)))
			if err != nil {
				t.Fatalf("stage valid synthetic %s QR: %v", tc.format, err)
			}
			store.Abort([]platformQRCodeUpload{store.uploads[id]}, nil)
		})
	}
}

func TestPlatformQRCodeStartupCleanupIsTTLBoundedAndPrefixScoped(t *testing.T) {
	root := t.TempDir()
	stageDir := filepath.Join(root, "private")
	if err := os.MkdirAll(stageDir, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stalePath := filepath.Join(stageDir, "platform-qr-stage-stale")
	freshPath := filepath.Join(stageDir, "platform-qr-stage-fresh")
	unrelatedPath := filepath.Join(stageDir, "unrelated-file")
	for _, path := range []string{stalePath, freshPath, unrelatedPath} {
		if err := os.WriteFile(path, []byte("synthetic stage artifact"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	staleTime := now.Add(-platformQRCodeTTL - time.Second)
	freshTime := now.Add(-platformQRCodeTTL / 2)
	if err := os.Chtimes(stalePath, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(freshPath, freshTime, freshTime); err != nil {
		t.Fatal(err)
	}
	store := newPlatformPaymentQRStore(stageDir, filepath.Join(root, "public"))
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("startup did not clean expired orphan stage: %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("startup removed a potentially active fresh stage: %v", err)
	}
	if _, err := os.Stat(unrelatedPath); err != nil {
		t.Fatalf("startup removed unrelated private file: %v", err)
	}
	store.now = func() time.Time { return now.Add(platformQRCodeTTL) }
	if _, err := store.Stage(83, "alipay", ".png", bytes.NewReader(syntheticPlatformQRCode(t, "png"))); err != nil {
		t.Fatalf("stage synthetic QR after stale file aged out: %v", err)
	}
	if _, err := os.Stat(freshPath); !os.IsNotExist(err) {
		t.Fatalf("runtime cleanup did not remove expired orphan stage: %v", err)
	}
	if _, err := os.Stat(unrelatedPath); err != nil {
		t.Fatalf("runtime cleanup removed unrelated file: %v", err)
	}
}

func TestPlatformQRCodeUploadRemainsPrivateUntilExplicitConfigSave(t *testing.T) {
	ctl, db, store := newPlatformQRTestController(t)
	oldURL := "http://synthetic.invalid/uploads/platform-payment/old_qr.png"
	oldFile := filepath.Join(store.publicDir, "old_qr.png")
	if err := os.MkdirAll(store.publicDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldFile, []byte("old-configured-file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SystemConfig{ConfigKey: "platform_alipay_qr_code", ConfigValue: oldURL}).Error; err != nil {
		t.Fatalf("create synthetic prior QR config: %v", err)
	}

	uploadResponse, uploadID := postPlatformQRCodeUpload(t, ctl, 71, "alipay")
	if uploadResponse.Code != http.StatusOK || uploadID == "" {
		t.Fatalf("synthetic upload failed: status=%d response=%s", uploadResponse.Code, uploadResponse.Body.String())
	}
	if strings.Contains(uploadResponse.Body.String(), `"url"`) {
		t.Fatalf("upload exposed a public URL before config save: %s", uploadResponse.Body.String())
	}
	if files, _ := filepath.Glob(filepath.Join(store.publicDir, "platform_qr_*")); len(files) != 0 {
		t.Fatalf("upload created publicly served files before config save: %v", files)
	}
	stagedFiles, err := filepath.Glob(filepath.Join(store.stageDir, "platform-qr-stage-*"))
	if err != nil || len(stagedFiles) != 1 {
		t.Fatalf("expected one private staged file, files=%v err=%v", stagedFiles, err)
	}

	saveBody, err := json.Marshal(saveSystemConfigWithQRUploads{
		SaveSystemConfigRequest:  dtoForPlatformQRTest(oldURL),
		PlatformAlipayQRUploadID: uploadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	saveResponse := invokePlatformQRHandlerWithRequest(t, ctl.SaveConfigs, http.MethodPost, "/api/v1/admin/configs", saveBody, "application/json", 71, "attacker.invalid", "http")
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("synthetic config save failed: status=%d response=%s", saveResponse.Code, saveResponse.Body.String())
	}
	var stored model.SystemConfig
	if err := db.Where("config_key = ?", "platform_alipay_qr_code").First(&stored).Error; err != nil {
		t.Fatalf("load finalized synthetic QR config: %v", err)
	}
	const publicPrefix = "https://platform.example.invalid/uploads/platform-payment/"
	if !strings.HasPrefix(stored.ConfigValue, publicPrefix) || stored.ConfigValue == oldURL {
		t.Fatalf("saved QR config did not reference finalized file: %q", stored.ConfigValue)
	}
	newFile := filepath.Join(store.publicDir, filepath.Base(stored.ConfigValue))
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("finalized public QR file is missing: %v", err)
	}
	if _, err := os.Stat(oldFile); err != nil {
		t.Fatalf("successful replacement deleted the prior configured file: %v", err)
	}
	stagedFiles, err = filepath.Glob(filepath.Join(store.stageDir, "platform-qr-stage-*"))
	if err != nil || len(stagedFiles) != 0 {
		t.Fatalf("successful save retained private staging files: files=%v err=%v", stagedFiles, err)
	}
	var audits int64
	if err := db.Model(&model.AuditLog{}).Count(&audits).Error; err != nil || audits != 1 {
		t.Fatalf("successful save audit count=%d err=%v, want 1", audits, err)
	}
	if _, err := store.ReserveUploads(71, map[string]string{"alipay": uploadID}); err == nil {
		t.Fatal("successfully consumed upload ID was reusable")
	}
}

func TestPlatformQRCodeFileFailurePreservesOldConfigAndCleansStage(t *testing.T) {
	ctl, db, store := newPlatformQRTestController(t)
	oldURL := "http://synthetic.invalid/uploads/platform-payment/old_file_failure.png"
	if err := db.Create(&model.SystemConfig{ConfigKey: "platform_alipay_qr_code", ConfigValue: oldURL}).Error; err != nil {
		t.Fatal(err)
	}
	_, uploadID := postPlatformQRCodeUpload(t, ctl, 72, "alipay")
	store.rename = func(string, string) error { return errors.New("synthetic finalize failure") }
	body, _ := json.Marshal(saveSystemConfigWithQRUploads{
		SaveSystemConfigRequest:  dtoForPlatformQRTest(oldURL),
		PlatformAlipayQRUploadID: uploadID,
	})
	response := invokePlatformQRHandler(t, ctl.SaveConfigs, http.MethodPost, "/api/v1/admin/configs", body, "application/json", 72)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("file finalization failure status=%d, want 500; response=%s", response.Code, response.Body.String())
	}
	assertPlatformQRFailurePreservedConfig(t, db, oldURL)
	assertPlatformQRNoNewFiles(t, store)
}

func TestPlatformQRCodeConfigAuditFailurePreservesOldConfigAndCleansFinalFile(t *testing.T) {
	ctl, db, store := newPlatformQRTestController(t)
	oldURL := "http://synthetic.invalid/uploads/platform-payment/old_audit_failure.png"
	oldFile := filepath.Join(store.publicDir, "old_audit_failure.png")
	if err := os.MkdirAll(store.publicDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldFile, []byte("old-file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SystemConfig{ConfigKey: "platform_alipay_qr_code", ConfigValue: oldURL}).Error; err != nil {
		t.Fatal(err)
	}
	_, uploadID := postPlatformQRCodeUpload(t, ctl, 73, "alipay")
	if err := db.Exec("CREATE TRIGGER synthetic_platform_qr_audit_failure BEFORE INSERT ON audit_logs BEGIN SELECT RAISE(ABORT, 'synthetic audit storage failure'); END").Error; err != nil {
		t.Fatalf("install synthetic audit failure: %v", err)
	}
	body, _ := json.Marshal(saveSystemConfigWithQRUploads{
		SaveSystemConfigRequest:  dtoForPlatformQRTest(oldURL),
		PlatformAlipayQRUploadID: uploadID,
	})
	response := invokePlatformQRHandler(t, ctl.SaveConfigs, http.MethodPost, "/api/v1/admin/configs", body, "application/json", 73)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("audit persistence failure status=%d, want 400; response=%s", response.Code, response.Body.String())
	}
	assertPlatformQRFailurePreservedConfig(t, db, oldURL)
	if _, err := os.Stat(oldFile); err != nil {
		t.Fatalf("audit failure deleted the old configured file: %v", err)
	}
	assertPlatformQRNoNewFiles(t, store)
}

func newPlatformQRTestController(t *testing.T) (*AdminController, *gorm.DB, *platformPaymentQRStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic platform QR database: %v", err)
	}
	if err := db.AutoMigrate(&model.SystemConfig{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic platform QR schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic platform QR database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	root := t.TempDir()
	store := newPlatformPaymentQRStore(filepath.Join(root, "private-stage"), filepath.Join(root, "public-uploads"))
	adminConfig := &config.Config{AESSecret: strings.Repeat("s", 32), AppURL: "https://platform.example.invalid"}
	admin := service.NewAdminService(adminConfig, db)
	return &AdminController{admin: admin, platformQR: store, appURL: adminConfig.AppURL}, db, store
}

func postPlatformQRCodeUpload(t *testing.T, ctl *AdminController, ownerID uint, channel string) (*httptest.ResponseRecorder, string) {
	return postPlatformQRCodeUploadContent(t, ctl, ownerID, channel, "synthetic-qr.png", syntheticPlatformQRCode(t, "png"))
}

func postPlatformQRCodeUploadContent(t *testing.T, ctl *AdminController, ownerID uint, channel, filename string, content []byte) (*httptest.ResponseRecorder, string) {
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
	response := invokePlatformQRHandler(t, ctl.UploadPaymentQRCode, http.MethodPost, "/api/v1/admin/payment-qrcode/upload", body.Bytes(), writer.FormDataContentType(), ownerID)
	var payload struct {
		Data struct {
			UploadID string `json:"upload_id"`
			Channel  string `json:"channel"`
		} `json:"data"`
	}
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode synthetic upload response: %v (%s)", err, response.Body.String())
		}
	}
	if payload.Data.Channel != "" && payload.Data.Channel != channel {
		t.Fatalf("upload channel=%q, want %q", payload.Data.Channel, channel)
	}
	return response, payload.Data.UploadID
}

func invokePlatformQRHandler(t *testing.T, handler gin.HandlerFunc, method, path string, body []byte, contentType string, ownerID uint) *httptest.ResponseRecorder {
	return invokePlatformQRHandlerWithRequest(t, handler, method, path, body, contentType, ownerID, "synthetic.local", "")
}

func invokePlatformQRHandlerWithRequest(t *testing.T, handler gin.HandlerFunc, method, path string, body []byte, contentType string, ownerID uint, host, forwardedProto string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	ctx.Request.Host = host
	ctx.Request.RemoteAddr = "127.0.0.1:12345"
	if forwardedProto != "" {
		ctx.Request.Header.Set("X-Forwarded-Proto", forwardedProto)
	}
	if contentType != "" {
		ctx.Request.Header.Set("Content-Type", contentType)
	}
	ctx.Set("user_id", ownerID)
	ctx.Set("role", "admin")
	handler(ctx)
	return response
}

func syntheticPlatformQRCode(t *testing.T, format string) []byte {
	t.Helper()
	if format == "webp" {
		data, err := base64.StdEncoding.DecodeString("UklGRh4AAABXRUJQVlA4TBEAAAAvAUAAAAfQ//73v/+BiOh/AAA=")
		if err != nil {
			t.Fatalf("decode synthetic WebP fixture: %v", err)
		}
		return data
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var out bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&out, img)
	case "jpeg":
		err = jpeg.Encode(&out, img, nil)
	default:
		t.Fatalf("unsupported synthetic image format %q", format)
	}
	if err != nil {
		t.Fatalf("encode synthetic %s image: %v", format, err)
	}
	return out.Bytes()
}

func dtoForPlatformQRTest(alipayURL string) dto.SaveSystemConfigRequest {
	return dto.SaveSystemConfigRequest{
		SiteName:                 "synthetic platform",
		PaymentGateway:           "synthetic-gateway",
		PlatformAlipayQRCode:     alipayURL,
		PlatformWechatQRCode:     "https://synthetic.invalid/wechat-existing.png",
		PlatformSubscriptionNote: "synthetic note",
		FilingInfo:               "synthetic filing",
		Notice:                   "synthetic notice",
	}
}

func assertPlatformQRFailurePreservedConfig(t *testing.T, db *gorm.DB, oldURL string) {
	t.Helper()
	var stored model.SystemConfig
	if err := db.Where("config_key = ?", "platform_alipay_qr_code").First(&stored).Error; err != nil {
		t.Fatalf("reload prior QR configuration after failure: %v", err)
	}
	var configs, audits int64
	if err := db.Model(&model.SystemConfig{}).Count(&configs).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AuditLog{}).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ConfigValue != oldURL || configs != 1 || audits != 0 {
		t.Fatalf("failed save changed config or audit state: url=%q configs=%d audits=%d", stored.ConfigValue, configs, audits)
	}
}

func assertPlatformQRNoNewFiles(t *testing.T, store *platformPaymentQRStore) {
	t.Helper()
	publicFiles, err := filepath.Glob(filepath.Join(store.publicDir, "platform_qr_*"))
	if err != nil {
		t.Fatal(err)
	}
	stagedFiles, err := filepath.Glob(filepath.Join(store.stageDir, "platform-qr-stage-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(publicFiles) != 0 || len(stagedFiles) != 0 {
		t.Fatalf("failed save did not clean new files: public=%v staged=%v", publicFiles, stagedFiles)
	}
}
