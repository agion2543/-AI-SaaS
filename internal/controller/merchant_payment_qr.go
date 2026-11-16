package controller

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

const (
	merchantQRCodeMaxSize      = 3 * 1024 * 1024
	merchantQRCodeMaxDimension = 8192
	merchantQRCodeMaxPixels    = 16 * 1024 * 1024
	merchantQRCodeTTL          = 30 * time.Minute
)

var validMerchantQRCodeExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// Merchant QR uploads are private until a successful explicit payment-config save.
type merchantPaymentQRStore struct {
	mu        sync.Mutex
	stageDir  string
	publicDir string
	ttl       time.Duration
	now       func() time.Time
	uploads   map[string]merchantQRCodeUpload
	mkdirAll  func(string, os.FileMode) error
	createTmp func(string, string) (*os.File, error)
	rename    func(string, string) error
	remove    func(string) error
}

type merchantQRCodeUpload struct {
	ID        string
	OwnerID   uint
	Channel   string
	Extension string
	StagePath string
	ExpiresAt time.Time
	Reserved  bool
}

type finalizedMerchantQRCode struct {
	UploadID string
	Path     string
	URLPath  string
}

func newDefaultMerchantPaymentQRStore() *merchantPaymentQRStore {
	return newMerchantPaymentQRStore(filepath.Join("var", "merchant-payment-staging"), filepath.Join("uploads", "merchant-payment"))
}

func newMerchantPaymentQRStore(stageDir, publicDir string) *merchantPaymentQRStore {
	store := &merchantPaymentQRStore{
		stageDir:  stageDir,
		publicDir: publicDir,
		ttl:       merchantQRCodeTTL,
		now:       time.Now,
		uploads:   make(map[string]merchantQRCodeUpload),
		mkdirAll:  os.MkdirAll,
		createTmp: os.CreateTemp,
		rename:    os.Rename,
		remove:    os.Remove,
	}
	_ = store.cleanupAbandonedStageFiles(store.now())
	return store
}

func (s *merchantPaymentQRStore) Stage(ownerID uint, channel, extension string, source io.Reader) (string, error) {
	if s == nil || ownerID == 0 || source == nil {
		return "", errors.New("商家收款码暂存信息无效")
	}
	channel = strings.TrimSpace(channel)
	extension = strings.ToLower(strings.TrimSpace(extension))
	if channel != "alipay" && channel != "wechat" {
		return "", errors.New("商家收款渠道无效")
	}
	if !validMerchantQRCodeExtensions[extension] {
		return "", errors.New("仅支持 jpg、png、webp 图片")
	}
	if err := s.mkdirAll(s.stageDir, 0700); err != nil {
		return "", fmt.Errorf("create private merchant QR staging directory: %w", err)
	}
	if err := os.Chmod(s.stageDir, 0700); err != nil {
		return "", fmt.Errorf("secure private merchant QR staging directory: %w", err)
	}
	if err := s.cleanupAbandonedStageFiles(s.now()); err != nil {
		return "", fmt.Errorf("cleanup abandoned merchant QR staging files: %w", err)
	}
	file, err := s.createTmp(s.stageDir, "merchant-qr-stage-*")
	if err != nil {
		return "", fmt.Errorf("create staged merchant QR file: %w", err)
	}
	stagePath := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = s.remove(stagePath)
	}
	if err := file.Chmod(0600); err != nil {
		cleanup()
		return "", fmt.Errorf("secure staged merchant QR file: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(source, merchantQRCodeMaxSize+1))
	if copyErr != nil {
		cleanup()
		return "", fmt.Errorf("write staged merchant QR file: %w", copyErr)
	}
	if written == 0 || written > merchantQRCodeMaxSize {
		cleanup()
		if written > merchantQRCodeMaxSize {
			return "", errors.New("收款码图片不能超过 3MB")
		}
		return "", errors.New("收款码图片为空")
	}
	if err := validateMerchantQRCode(file, extension); err != nil {
		cleanup()
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = s.remove(stagePath)
		return "", fmt.Errorf("close staged merchant QR file: %w", err)
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
	s.uploads[id] = merchantQRCodeUpload{
		ID:        id,
		OwnerID:   ownerID,
		Channel:   channel,
		Extension: extension,
		StagePath: stagePath,
		ExpiresAt: s.now().Add(s.ttl),
	}
	return id, nil
}

func validateMerchantQRCode(file *os.File, extension string) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("读取收款码图片失败: %w", err)
	}
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return errors.New("收款码文件不是有效的 jpg、png 或 webp 图片")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > merchantQRCodeMaxDimension || config.Height > merchantQRCodeMaxDimension || int64(config.Width)*int64(config.Height) > merchantQRCodeMaxPixels {
		return errors.New("收款码图片尺寸无效或过大")
	}
	if !merchantQRCodeExtensionMatchesFormat(extension, format) {
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

func merchantQRCodeExtensionMatchesFormat(extension, format string) bool {
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

func (s *merchantPaymentQRStore) cleanupAbandonedStageFiles(now time.Time) error {
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
		if !strings.HasPrefix(entry.Name(), "merchant-qr-stage-") {
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

func (s *merchantPaymentQRStore) ReserveUploads(ownerID uint, requested map[string]string) ([]merchantQRCodeUpload, error) {
	if s == nil || ownerID == 0 {
		return nil, errors.New("商家会话无效")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(s.now())
	refs := make([]merchantQRCodeUpload, 0, 2)
	seen := make(map[string]bool, 2)
	for _, channel := range []string{"alipay", "wechat"} {
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
			return nil, errors.New("收款码暂存凭证无效、已过期或不属于当前商家/渠道，请重新上传")
		}
		refs = append(refs, ref)
	}
	for _, ref := range refs {
		ref.Reserved = true
		s.uploads[ref.ID] = ref
	}
	return refs, nil
}

func (s *merchantPaymentQRStore) Finalize(ref merchantQRCodeUpload) (finalizedMerchantQRCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.uploads[ref.ID]
	if !ok || current.OwnerID != ref.OwnerID || current.Channel != ref.Channel || !current.Reserved || !s.now().Before(current.ExpiresAt) {
		return finalizedMerchantQRCode{}, errors.New("商家收款码暂存凭证已失效，请重新上传")
	}
	filename := "merchant_qr_" + strings.ReplaceAll(uuid.NewString(), "-", "") + ref.Extension
	finalPath := filepath.Join(s.publicDir, filename)
	if err := s.mkdirAll(s.publicDir, 0755); err != nil {
		return finalizedMerchantQRCode{}, fmt.Errorf("create public merchant QR directory: %w", err)
	}
	if err := s.rename(current.StagePath, finalPath); err != nil {
		return finalizedMerchantQRCode{}, fmt.Errorf("finalize staged merchant QR file: %w", err)
	}
	return finalizedMerchantQRCode{
		UploadID: ref.ID,
		Path:     finalPath,
		URLPath:  "/uploads/merchant-payment/" + filename,
	}, nil
}

func (s *merchantPaymentQRStore) Abort(refs []merchantQRCodeUpload, finalized []finalizedMerchantQRCode) {
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

func (s *merchantPaymentQRStore) Commit(refs []merchantQRCodeUpload) {
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

func (s *merchantPaymentQRStore) cleanupExpiredLocked(now time.Time) {
	for id, upload := range s.uploads {
		if !now.Before(upload.ExpiresAt) {
			_ = s.remove(upload.StagePath)
			delete(s.uploads, id)
		}
	}
}

func merchantQRCodeURL(appURL, publicPath string) string {
	baseURL, err := url.Parse(strings.TrimSpace(appURL))
	if err != nil || baseURL.Host == "" || baseURL.User != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return "http://localhost:8080/" + strings.TrimLeft(publicPath, "/")
	}
	origin := strings.ToLower(baseURL.Scheme) + "://" + baseURL.Host
	return origin + "/" + strings.TrimLeft(publicPath, "/")
}
