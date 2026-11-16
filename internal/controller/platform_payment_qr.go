package controller

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
	_ "golang.org/x/image/webp"
)

const (
	platformQRCodeMaxSize      = 3 * 1024 * 1024
	platformQRCodeMaxDimension = 8192
	platformQRCodeMaxPixels    = 16 * 1024 * 1024
	platformQRCodeTTL          = 30 * time.Minute
)

var validPlatformQRCodeExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// Staged platform QR files live outside ./uploads and are never exposed by the
// public static-file handler. Upload IDs are process-local, opaque and short-lived.
type platformPaymentQRStore struct {
	mu        sync.Mutex
	stageDir  string
	publicDir string
	ttl       time.Duration
	now       func() time.Time
	uploads   map[string]platformQRCodeUpload
	mkdirAll  func(string, os.FileMode) error
	createTmp func(string, string) (*os.File, error)
	rename    func(string, string) error
	remove    func(string) error
}

type platformQRCodeUpload struct {
	ID        string
	OwnerID   uint
	Channel   string
	Extension string
	StagePath string
	ExpiresAt time.Time
	Reserved  bool
}

type finalizedPlatformQRCode struct {
	UploadID   string
	Path       string
	PublicPath string
}

func newDefaultPlatformPaymentQRStore() *platformPaymentQRStore {
	return newPlatformPaymentQRStore(filepath.Join("var", "platform-payment-staging"), filepath.Join("uploads", "platform-payment"))
}

func newPlatformPaymentQRStore(stageDir, publicDir string) *platformPaymentQRStore {
	store := &platformPaymentQRStore{
		stageDir:  stageDir,
		publicDir: publicDir,
		ttl:       platformQRCodeTTL,
		now:       time.Now,
		uploads:   make(map[string]platformQRCodeUpload),
		mkdirAll:  os.MkdirAll,
		createTmp: os.CreateTemp,
		rename:    os.Rename,
		remove:    os.Remove,
	}
	// Upload IDs are intentionally process-local. Clean only old files with the
	// private staging prefix; younger files may belong to another live process.
	_ = store.cleanupAbandonedStageFiles(store.now())
	return store
}

func (s *platformPaymentQRStore) Stage(ownerID uint, channel, extension string, source io.Reader) (string, error) {
	if s == nil || ownerID == 0 || source == nil {
		return "", errors.New("平台收款码暂存信息无效")
	}
	channel = strings.TrimSpace(channel)
	extension = strings.ToLower(strings.TrimSpace(extension))
	if channel != "alipay" && channel != "wechat" {
		return "", errors.New("平台收款渠道无效")
	}
	if !validPlatformQRCodeExtensions[extension] {
		return "", errors.New("仅支持 jpg、png、webp 图片")
	}
	if err := s.mkdirAll(s.stageDir, 0700); err != nil {
		return "", fmt.Errorf("create private staging directory: %w", err)
	}
	if err := s.cleanupAbandonedStageFiles(s.now()); err != nil {
		return "", fmt.Errorf("cleanup abandoned platform QR staging files: %w", err)
	}
	file, err := s.createTmp(s.stageDir, "platform-qr-stage-*")
	if err != nil {
		return "", fmt.Errorf("create staged platform QR file: %w", err)
	}
	stagePath := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = s.remove(stagePath)
	}
	if err := file.Chmod(0600); err != nil {
		cleanup()
		return "", fmt.Errorf("secure staged platform QR file: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(source, platformQRCodeMaxSize+1))
	if copyErr != nil {
		cleanup()
		return "", fmt.Errorf("write staged platform QR file: %w", copyErr)
	}
	if written == 0 || written > platformQRCodeMaxSize {
		cleanup()
		if written > platformQRCodeMaxSize {
			return "", errors.New("收款码图片不能超过 3MB")
		}
		return "", errors.New("收款码图片为空")
	}
	if err := validatePlatformQRCode(file, extension); err != nil {
		cleanup()
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = s.remove(stagePath)
		return "", fmt.Errorf("close staged platform QR file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(s.now())
	id := uuid.NewString()
	for {
		if _, exists := s.uploads[id]; !exists {
			break
		}
		id = uuid.NewString()
	}
	s.uploads[id] = platformQRCodeUpload{
		ID:        id,
		OwnerID:   ownerID,
		Channel:   channel,
		Extension: extension,
		StagePath: stagePath,
		ExpiresAt: s.now().Add(s.ttl),
	}
	return id, nil
}

func validatePlatformQRCode(file *os.File, extension string) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("读取收款码图片失败: %w", err)
	}
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return errors.New("收款码文件不是有效的 jpg、png 或 webp 图片")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > platformQRCodeMaxDimension || config.Height > platformQRCodeMaxDimension || int64(config.Width)*int64(config.Height) > platformQRCodeMaxPixels {
		return errors.New("收款码图片尺寸无效或过大")
	}
	if !platformQRCodeExtensionMatchesFormat(extension, format) {
		return errors.New("图片内容与文件扩展名不匹配")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("读取收款码图片失败: %w", err)
	}
	if _, decodedFormat, err := image.Decode(file); err != nil || decodedFormat != format {
		return errors.New("收款码文件不是完整有效的图片")
	}
	return nil
}

func platformQRCodeExtensionMatchesFormat(extension, format string) bool {
	switch extension {
	case ".jpg", ".jpeg":
		return format == "jpeg"
	case ".png":
		return format == "png"
	case ".webp":
		return format == "webp"
	default:
		return false
	}
}

func (s *platformPaymentQRStore) cleanupAbandonedStageFiles(now time.Time) error {
	if s == nil {
		return nil
	}
	entries, err := os.ReadDir(s.stageDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	cutoff := now.Add(-s.ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	activePaths := make(map[string]struct{}, len(s.uploads))
	for _, upload := range s.uploads {
		activePaths[filepath.Clean(upload.StagePath)] = struct{}{}
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "platform-qr-stage-") {
			continue
		}
		path := filepath.Join(s.stageDir, entry.Name())
		if _, active := activePaths[filepath.Clean(path)]; active {
			continue
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.ModTime().After(cutoff) {
			continue
		}
		if err := s.remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// ReserveUploads validates all requested IDs before reserving any of them.
// Channel and administrator binding are enforced from server-side upload state.
func (s *platformPaymentQRStore) ReserveUploads(ownerID uint, requested map[string]string) ([]platformQRCodeUpload, error) {
	if s == nil || ownerID == 0 {
		return nil, errors.New("管理员会话无效")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(s.now())
	channels := []string{"alipay", "wechat"}
	refs := make([]platformQRCodeUpload, 0, len(channels))
	seen := make(map[string]bool, len(channels))
	for _, channel := range channels {
		id := strings.TrimSpace(requested[channel])
		if id == "" {
			continue
		}
		if seen[id] {
			return nil, errors.New("同一个暂存二维码不能用于多个收款渠道")
		}
		seen[id] = true
		ref, ok := s.uploads[id]
		if !ok || ref.OwnerID != ownerID || ref.Channel != channel || ref.Reserved || !s.now().Before(ref.ExpiresAt) {
			return nil, errors.New("收款码暂存凭证无效、已过期或不属于当前管理员/渠道，请重新上传")
		}
		refs = append(refs, ref)
	}
	for _, ref := range refs {
		ref.Reserved = true
		s.uploads[ref.ID] = ref
	}
	return refs, nil
}

func (s *platformPaymentQRStore) Finalize(ref platformQRCodeUpload) (finalizedPlatformQRCode, error) {
	filename := "platform_qr_" + strings.ReplaceAll(uuid.NewString(), "-", "") + ref.Extension
	finalPath := filepath.Join(s.publicDir, filename)
	if err := s.mkdirAll(s.publicDir, 0755); err != nil {
		return finalizedPlatformQRCode{}, fmt.Errorf("create public platform QR directory: %w", err)
	}
	if err := s.rename(ref.StagePath, finalPath); err != nil {
		return finalizedPlatformQRCode{}, fmt.Errorf("finalize staged platform QR file: %w", err)
	}
	return finalizedPlatformQRCode{
		UploadID:   ref.ID,
		Path:       finalPath,
		PublicPath: "/uploads/platform-payment/" + filename,
	}, nil
}

func (s *platformPaymentQRStore) Abort(refs []platformQRCodeUpload, finalized []finalizedPlatformQRCode) {
	if s == nil {
		return
	}
	for _, file := range finalized {
		_ = s.remove(file.Path)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ref := range refs {
		if current, ok := s.uploads[ref.ID]; ok && current.OwnerID == ref.OwnerID {
			_ = s.remove(current.StagePath)
			delete(s.uploads, ref.ID)
		}
	}
}

func (s *platformPaymentQRStore) Commit(refs []platformQRCodeUpload) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ref := range refs {
		if current, ok := s.uploads[ref.ID]; ok && current.OwnerID == ref.OwnerID {
			delete(s.uploads, ref.ID)
		}
	}
}

func (s *platformPaymentQRStore) cleanupExpiredLocked(now time.Time) {
	for id, upload := range s.uploads {
		if !now.Before(upload.ExpiresAt) {
			_ = s.remove(upload.StagePath)
			delete(s.uploads, id)
		}
	}
}

type saveSystemConfigWithQRUploads struct {
	dto.SaveSystemConfigRequest
	PlatformAlipayQRUploadID string `json:"platform_alipay_qr_upload_id"`
	PlatformWechatQRUploadID string `json:"platform_wechat_qr_upload_id"`
}

func (ctl *AdminController) SaveConfigs(c *gin.Context) {
	var req saveSystemConfigWithQRUploads
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	refs, err := ctl.platformQR.ReserveUploads(c.GetUint("user_id"), map[string]string{
		"alipay": req.PlatformAlipayQRUploadID,
		"wechat": req.PlatformWechatQRUploadID,
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	finalized := make([]finalizedPlatformQRCode, 0, len(refs))
	for _, ref := range refs {
		file, finalizeErr := ctl.platformQR.Finalize(ref)
		if finalizeErr != nil {
			ctl.platformQR.Abort(refs, finalized)
			utils.Error(c, http.StatusInternalServerError, "收款码定稿失败，配置未保存")
			return
		}
		finalized = append(finalized, file)
		url := platformQRCodeURL(ctl.appURL, file.PublicPath)
		switch ref.Channel {
		case "alipay":
			req.PlatformAlipayQRCode = url
		case "wechat":
			req.PlatformWechatQRCode = url
		}
	}
	if err := ctl.admin.SaveSystemConfig(req.SaveSystemConfigRequest, service.AuditActorFromContext(c)); err != nil {
		ctl.platformQR.Abort(refs, finalized)
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctl.platformQR.Commit(refs)
	utils.Success(c, gin.H{"saved": true})
}

func (ctl *AdminController) UploadPaymentQRCode(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "请选择要上传的收款码图片")
		return
	}
	if file.Size > platformQRCodeMaxSize {
		utils.Error(c, http.StatusBadRequest, "收款码图片不能超过 3MB")
		return
	}
	channel := strings.TrimSpace(c.PostForm("channel"))
	if channel != "alipay" && channel != "wechat" {
		utils.Error(c, http.StatusBadRequest, "请指定支付宝或微信收款渠道")
		return
	}
	extension := strings.ToLower(filepath.Ext(file.Filename))
	if !validPlatformQRCodeExtensions[extension] {
		utils.Error(c, http.StatusBadRequest, "仅支持 jpg、png、webp 图片")
		return
	}
	source, err := file.Open()
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "读取收款码图片失败")
		return
	}
	defer source.Close()
	id, err := ctl.platformQR.Stage(c.GetUint("user_id"), channel, extension, source)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{
		"upload_id":          id,
		"channel":            channel,
		"expires_in_seconds": int64(ctl.platformQR.ttl.Seconds()),
	})
}

func platformQRCodeURL(appURL, publicPath string) string {
	baseURL, err := url.Parse(strings.TrimSpace(appURL))
	if err != nil || baseURL.Host == "" || baseURL.User != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return "http://localhost:8080/" + strings.TrimLeft(publicPath, "/")
	}
	origin := strings.ToLower(baseURL.Scheme) + "://" + baseURL.Host
	return origin + "/" + strings.TrimLeft(publicPath, "/")
}
