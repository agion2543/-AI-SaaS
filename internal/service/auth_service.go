package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
	"go-web-gin-health/internal/utils"
)

const mainlandPhoneLength = 11
const devFixedSMSCode = "123456"

type AuthService struct {
	cfg   *config.Config
	db    *gorm.DB
	users *repository.UserRepository
	admin *repository.AdminRepository
	codes *repository.VerificationCodeRepository
}

func NewAuthService(cfg *config.Config, db *gorm.DB) *AuthService {
	return &AuthService{
		cfg:   cfg,
		db:    db,
		users: repository.NewUserRepository(db),
		admin: repository.NewAdminRepository(db),
		codes: repository.NewVerificationCodeRepository(db),
	}
}

func (s *AuthService) Register(req dto.RegisterRequest) (*model.User, string, error) {
	req.Phone = normalizePhone(req.Phone)
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if !isValidMainlandPhone(req.Phone) {
		return nil, "", errors.New("手机号必须为 11 位数字")
	}
	if req.DisplayName == "" {
		return nil, "", errors.New("显示名不能为空")
	}
	if _, err := s.users.FindByPhone(req.Phone); err == nil {
		return nil, "", errors.New("手机号已被注册")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", err
	}
	if _, err := s.users.FindByDisplayName(req.DisplayName); err == nil {
		return nil, "", errors.New("显示名已存在，请更换后重试")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", err
	}
	if err := s.consumeVerificationCode(req.Phone, "register", req.SMSCode); err != nil {
		return nil, "", err
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, "", err
	}

	user := &model.User{
		UUID:         uuid.NewString(),
		Email:        newPendingEmail(),
		Phone:        req.Phone,
		MerchantID:   nil,
		Role:         "customer",
		PasswordHash: hash,
		DisplayName:  req.DisplayName,
		Status:       "active",
		MemberLevel:  "free",
	}

	if err := s.users.Create(user); err != nil {
		return nil, "", err
	}

	token, err := utils.GenerateToken(s.cfg.JWTSecret, user.ID, user.Phone, user.Role, user.MerchantID, 72)
	return user, token, err
}

func (s *AuthService) Login(req dto.LoginRequest) (*model.User, string, error) {
	user, err := s.users.FindByIdentifier(strings.TrimSpace(req.Account))
	if err != nil {
		return nil, "", errors.New("账号或密码错误")
	}
	if user.Status != "active" {
		return nil, "", errors.New("账号已被禁用")
	}
	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, "", errors.New("账号或密码错误")
	}

	s.ensureUserTenantDefaults(user)

	token, err := utils.GenerateToken(s.cfg.JWTSecret, user.ID, user.Phone, user.Role, user.MerchantID, 72)
	return user, token, err
}

func (s *AuthService) LoginByPhone(req dto.PhoneLoginRequest) (*model.User, string, error) {
	phone := normalizePhone(req.Phone)
	if !isValidMainlandPhone(phone) {
		return nil, "", errors.New("手机号必须为 11 位数字")
	}

	user, err := s.users.FindByPhone(phone)
	if err != nil {
		return nil, "", errors.New("手机号未注册")
	}
	if user.Status != "active" {
		return nil, "", errors.New("账号已被禁用")
	}
	if err := s.consumeVerificationCode(phone, "login", req.Code); err != nil {
		return nil, "", err
	}

	s.ensureUserTenantDefaults(user)

	token, err := utils.GenerateToken(s.cfg.JWTSecret, user.ID, user.Phone, user.Role, user.MerchantID, 72)
	return user, token, err
}

func (s *AuthService) ForgotPassword(req dto.ForgotPasswordRequest) error {
	user, err := s.users.FindByEmail(strings.TrimSpace(strings.ToLower(req.Email)))
	if err != nil {
		return errors.New("用户不存在")
	}

	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	user.ResetToken = fmt.Sprintf("reset_%d", time.Now().Unix())
	return s.users.Save(user)
}

func (s *AuthService) AdminLogin(req dto.AdminLoginRequest) (*model.AdminUser, string, error) {
	admin, err := s.admin.FindByUsername(strings.TrimSpace(req.Username))
	if err != nil {
		return nil, "", errors.New("账号或密码错误")
	}
	if admin.Status != "active" {
		return nil, "", errors.New("账号已被禁用")
	}
	if !utils.CheckPassword(admin.PasswordHash, req.Password) {
		return nil, "", errors.New("账号或密码错误")
	}

	tokenRole := "admin"
	if admin.Role.Code == "super_admin" {
		tokenRole = "super_admin"
	}
	credentialVersion := utils.AdminCredentialVersion(s.cfg.JWTSecret, admin.AuthVersion, admin.PasswordHash, admin.Status, admin.RoleID, admin.Role.Code, admin.Role.Permissions)
	token, err := utils.GenerateAdminToken(s.cfg.JWTSecret, admin.ID, admin.Username, tokenRole, credentialVersion, 24)
	return admin, token, err
}

func (s *AuthService) SendRegisterCode(req dto.SendSMSCodeRequest) error {
	phone := normalizePhone(req.Phone)
	if !isValidMainlandPhone(phone) {
		return errors.New("手机号必须为 11 位数字")
	}
	if _, err := s.users.FindByPhone(phone); err == nil {
		return errors.New("手机号已被注册")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.issueVerificationCode(phone, "register", nil)
}

func (s *AuthService) SendLoginCode(req dto.SendLoginCodeRequest) error {
	phone := normalizePhone(req.Account)
	if !isValidMainlandPhone(phone) {
		return errors.New("手机号必须为 11 位数字")
	}
	user, err := s.users.FindByPhone(phone)
	if err != nil {
		return errors.New("手机号未注册")
	}
	return s.issueVerificationCode(user.Phone, "login", &user.ID)
}

func (s *AuthService) SendPasswordResetCode(req dto.SendSMSCodeRequest) error {
	phone := normalizePhone(req.Phone)
	if !isValidMainlandPhone(phone) {
		return errors.New("手机号必须为 11 位数字")
	}
	user, err := s.users.FindByPhone(phone)
	if err != nil {
		return errors.New("手机号未注册")
	}
	return s.issueVerificationCode(phone, "reset_password", &user.ID)
}

func (s *AuthService) SendBindPhoneCode(userID uint, req dto.SendSMSCodeRequest) error {
	phone := normalizePhone(req.Phone)
	if !isValidMainlandPhone(phone) {
		return errors.New("手机号必须为 11 位数字")
	}

	user, err := s.users.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	if user.Phone != phone {
		if existing, err := s.users.FindByPhone(phone); err == nil && existing.ID != userID {
			return errors.New("手机号已被其他账号使用")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	return s.issueVerificationCode(phone, "bind_phone", &userID)
}

func (s *AuthService) BindPhone(userID uint, req dto.BindPhoneRequest) (*model.User, error) {
	phone := normalizePhone(req.Phone)
	if !isValidMainlandPhone(phone) {
		return nil, errors.New("手机号必须为 11 位数字")
	}

	var updated *model.User
	invalidCode := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		users := repository.NewUserRepository(tx)

		user, err := users.FindByID(userID)
		if err != nil {
			return errors.New("用户不存在")
		}

		if user.Phone != phone {
			if existing, err := users.FindByPhone(phone); err == nil && existing.ID != userID {
				return errors.New("手机号已被其他账号使用")
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		valid, err := consumeSMSVerificationCodeInTransaction(tx, s.cfg.JWTSecret, phone, "bind_phone", req.Code)
		if err != nil {
			return err
		}
		if !valid {
			invalidCode = true
			return nil
		}

		user.Phone = phone

		if err := users.Save(user); err != nil {
			return err
		}
		updated = user
		return nil
	})
	if err != nil {
		return nil, err
	}
	if invalidCode {
		return nil, errSMSInvalidCode
	}
	return updated, nil
}

func (s *AuthService) ResetPasswordByPhone(req dto.ResetPasswordByPhoneRequest) error {
	phone := normalizePhone(req.Phone)
	if !isValidMainlandPhone(phone) {
		return errors.New("手机号必须为 11 位数字")
	}

	invalidCode := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		users := repository.NewUserRepository(tx)

		user, err := users.FindByPhone(phone)
		if err != nil {
			return errors.New("手机号未注册")
		}

		valid, err := consumeSMSVerificationCodeInTransaction(tx, s.cfg.JWTSecret, phone, "reset_password", req.Code)
		if err != nil {
			return err
		}
		if !valid {
			invalidCode = true
			return nil
		}

		hash, err := utils.HashPassword(req.NewPassword)
		if err != nil {
			return err
		}

		now := time.Now()
		user.PasswordHash = hash
		user.ResetToken = fmt.Sprintf("reset_%d", now.Unix())

		return users.Save(user)
	})
	if err != nil {
		return err
	}
	if invalidCode {
		return errSMSInvalidCode
	}
	return nil
}

func (s *AuthService) SendBindEmailCode(userID uint, req dto.SendEmailCodeRequest) error {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if !isSafeBindableEmail(email) {
		return errors.New("邮箱格式不正确")
	}

	user, err := s.users.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	if !isPendingEmail(user.Email) && user.Email != email {
		if existing, err := s.users.FindByEmail(email); err == nil && existing.ID != userID {
			return errors.New("邮箱已被其他账号绑定")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	return s.issueEmailVerificationCode(email, "bind_email", &userID)
}

func (s *AuthService) BindEmail(userID uint, req dto.BindEmailRequest) (*model.User, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if !isSafeBindableEmail(email) {
		return nil, errors.New("邮箱格式不正确")
	}

	var updated *model.User
	invalidCode := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		users := repository.NewUserRepository(tx)

		user, err := users.FindByID(userID)
		if err != nil {
			return errors.New("用户不存在")
		}

		if !isPendingEmail(user.Email) && user.Email != email {
			if existing, err := users.FindByEmail(email); err == nil && existing.ID != userID {
				return errors.New("邮箱已被其他账号绑定")
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		valid, err := consumeEmailVerificationCodeInTransaction(tx, s.cfg.JWTSecret, email, "bind_email", req.Code)
		if err != nil {
			return err
		}
		if !valid {
			invalidCode = true
			return nil
		}

		user.Email = email

		if err := users.Save(user); err != nil {
			return err
		}
		updated = user
		return nil
	})

	if err != nil {
		return nil, err
	}
	if invalidCode {
		return nil, errEmailInvalidCode
	}
	return updated, nil
}

func (s *AuthService) DevResetPassword(req dto.DevResetPasswordRequest) error {
	if strings.EqualFold(s.cfg.AppEnv, "production") {
		return errors.New("生产环境不允许使用开发重置密码功能")
	}

	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	switch req.AccountType {
	case "admin":
		admin, err := s.admin.FindByUsername(strings.TrimSpace(req.Account))
		if err != nil {
			return errors.New("管理员不存在")
		}
		return s.db.Model(&model.AdminUser{}).Where("id = ?", admin.ID).Updates(map[string]interface{}{
			"password_hash": hash,
			"auth_version":  gorm.Expr("auth_version + ?", 1),
		}).Error
	case "user":
		user, err := s.users.FindByIdentifier(strings.TrimSpace(req.Account))
		if err != nil {
			return errors.New("用户不存在")
		}
		user.PasswordHash = hash
		user.ResetToken = fmt.Sprintf("dev_reset_%d", time.Now().Unix())
		return s.users.Save(user)
	default:
		return errors.New("不支持的账号类型")
	}
}

func (s *AuthService) issueVerificationCode(phone, scene string, userID *uint) error {
	return issueSMSVerificationCode(s.db, s.cfg.AppEnv, s.cfg.JWTSecret, phone, scene, userID)
}

func (s *AuthService) issueEmailVerificationCode(email, scene string, userID *uint) error {
	return issueEmailVerificationCode(s.db, s.cfg.AppEnv, s.cfg.JWTSecret, email, scene, userID)
}

func (s *AuthService) consumeVerificationCode(phone, scene, code string) error {
	return consumeSMSVerificationCode(s.db, s.cfg.JWTSecret, phone, scene, code)
}

func (s *AuthService) ensureUserTenantDefaults(user *model.User) {
	if user.Role == "" {
		user.Role = "customer"
	}
}

func (s *AuthService) generateSMSCode() (string, error) {
	return generateMockSMSCode(s.cfg.AppEnv)
}

func normalizePhone(phone string) string {
	return strings.TrimSpace(phone)
}

func isValidMainlandPhone(phone string) bool {
	if len(phone) != mainlandPhoneLength {
		return false
	}
	if !strings.HasPrefix(phone, "1") {
		return false
	}
	for _, char := range phone {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func newPendingEmail() string {
	return fmt.Sprintf("%s@pending.local", strings.ReplaceAll(uuid.NewString(), "-", ""))
}

func isPendingEmail(email string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(email)), "@pending.local")
}

func isSafeBindableEmail(email string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || isPendingEmail(email) {
		return false
	}
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}
