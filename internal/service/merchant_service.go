package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

type MerchantService struct {
	cfg        *config.Config
	db         *gorm.DB
	merchants  *repository.MerchantRepository
	users      *repository.UserRepository
	stores     *repository.StoreRepository
	products   *repository.StoreProductRepository
	leads      *repository.CustomerLeadRepository
	promotions *repository.PromotionRepository
	plans      *repository.MerchantPlanRepository
	orders     *repository.OrderRepository
	packages   *repository.PackageRepository
	cards      *repository.CardRepository
	codes      *repository.VerificationCodeRepository
	systems    *repository.SystemRepository
	ai         *AIService
}

func NewMerchantService(cfg *config.Config, db *gorm.DB) *MerchantService {
	return &MerchantService{
		cfg:        cfg,
		db:         db,
		merchants:  repository.NewMerchantRepository(db),
		users:      repository.NewUserRepository(db),
		stores:     repository.NewStoreRepository(db),
		products:   repository.NewStoreProductRepository(db),
		leads:      repository.NewCustomerLeadRepository(db),
		promotions: repository.NewPromotionRepository(db),
		plans:      repository.NewMerchantPlanRepository(db),
		orders:     repository.NewOrderRepository(db),
		packages:   repository.NewPackageRepository(db),
		cards:      repository.NewCardRepository(db),
		codes:      repository.NewVerificationCodeRepository(db),
		systems:    repository.NewSystemRepository(db),
		ai:         NewAIService(cfg),
	}
}

func (s *MerchantService) SendRegisterCode(req dto.SendSMSCodeRequest) error {
	phone := normalizePhone(req.Phone)
	if !isValidMainlandPhone(phone) {
		return errors.New("\u624b\u673a\u53f7\u5fc5\u987b\u4e3a 11 \u4f4d\u6570\u5b57")
	}
	if _, err := s.users.FindByPhone(phone); err == nil {
		return errors.New("\u8054\u7cfb\u4eba\u624b\u673a\u53f7\u5df2\u88ab\u6ce8\u518c")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.issueMerchantVerificationCode(phone, "merchant_register", nil)
}

func (s *MerchantService) Register(req dto.MerchantRegisterRequest) (*model.Merchant, *model.User, string, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.ContactPhone = normalizePhone(req.ContactPhone)
	req.SMSCode = strings.TrimSpace(req.SMSCode)

	if req.Name == "" {
		return nil, nil, "", errors.New("\u5546\u5bb6\u540d\u79f0\u4e0d\u80fd\u4e3a\u7a7a")
	}
	if !isValidMainlandPhone(req.ContactPhone) {
		return nil, nil, "", errors.New("\u8054\u7cfb\u4eba\u624b\u673a\u53f7\u5fc5\u987b\u4e3a 11 \u4f4d\u6570\u5b57")
	}
	if len(req.Password) < 6 {
		return nil, nil, "", errors.New("\u5bc6\u7801\u81f3\u5c11 6 \u4f4d")
	}
	if req.Password != req.ConfirmPassword {
		return nil, nil, "", errors.New("\u4e24\u6b21\u8f93\u5165\u7684\u5bc6\u7801\u4e0d\u4e00\u81f4")
	}
	if err := s.consumeMerchantVerificationCode(req.ContactPhone, "merchant_register", req.SMSCode); err != nil {
		return nil, nil, "", err
	}

	if _, err := s.merchants.FindByName(req.Name); err == nil {
		return nil, nil, "", errors.New("\u5546\u5bb6\u540d\u79f0\u5df2\u5b58\u5728\uff0c\u8bf7\u66f4\u6362\u540e\u91cd\u8bd5")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, "", err
	}

	if _, err := s.users.FindByPhone(req.ContactPhone); err == nil {
		return nil, nil, "", errors.New("\u8054\u7cfb\u4eba\u624b\u673a\u53f7\u5df2\u88ab\u6ce8\u518c")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, "", err
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, nil, "", err
	}

	var merchant *model.Merchant
	var user *model.User

	err = s.db.Transaction(func(tx *gorm.DB) error {
		merchantRepo := repository.NewMerchantRepository(tx)
		userRepo := repository.NewUserRepository(tx)

		merchant = &model.Merchant{
			Name:         req.Name,
			ContactPhone: req.ContactPhone,
			Status:       "active",
		}
		if err := merchantRepo.Create(merchant); err != nil {
			return err
		}

		user = &model.User{
			UUID:         uuid.NewString(),
			Email:        newPendingEmail(),
			Phone:        req.ContactPhone,
			MerchantID:   ptrUint(uint(merchant.ID)),
			Role:         "merchant_admin",
			PasswordHash: hash,
			DisplayName:  req.Name,
			Status:       "active",
			MemberLevel:  "free",
		}
		if err := userRepo.Create(user); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, nil, "", err
	}

	token, err := utils.GenerateToken(s.cfg.JWTSecret, user.ID, user.Phone, user.Role, user.MerchantID, 72)
	return merchant, user, token, err
}

func (s *MerchantService) Login(req dto.MerchantLoginRequest) (*model.Merchant, *model.User, string, error) {
	phone := normalizePhone(req.Phone)
	if !isValidMainlandPhone(phone) {
		return nil, nil, "", errors.New("\u624b\u673a\u53f7\u5fc5\u987b\u4e3a 11 \u4f4d\u6570\u5b57")
	}

	user, err := s.users.FindByPhone(phone)
	if err != nil {
		return nil, nil, "", errors.New("\u8d26\u53f7\u6216\u5bc6\u7801\u9519\u8bef")
	}
	if user.Role != "merchant_admin" {
		return nil, nil, "", errors.New("\u8be5\u8d26\u53f7\u4e0d\u662f\u5546\u5bb6\u7ba1\u7406\u5458\u8d26\u53f7")
	}
	if user.Status != "active" {
		return nil, nil, "", errors.New("\u8d26\u53f7\u5df2\u88ab\u7981\u7528")
	}
	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, nil, "", errors.New("\u8d26\u53f7\u6216\u5bc6\u7801\u9519\u8bef")
	}
	if user.MerchantID == nil {
		return nil, nil, "", errors.New("\u5546\u5bb6\u4fe1\u606f\u4e0d\u5b58\u5728")
	}

	merchant, err := s.merchants.FindByID(*user.MerchantID)
	if err != nil {
		return nil, nil, "", errors.New("\u5546\u5bb6\u4fe1\u606f\u4e0d\u5b58\u5728")
	}
	if merchant.Status == "suspended" {
		return nil, nil, "", errors.New("\u5546\u5bb6\u5df2\u88ab\u7981\u7528")
	}

	token, err := utils.GenerateToken(s.cfg.JWTSecret, user.ID, user.Phone, user.Role, user.MerchantID, 72)
	return merchant, user, token, err
}

func (s *MerchantService) SendPasswordResetCode(req dto.MerchantSendPasswordResetCodeRequest) error {
	phone := normalizePhone(req.Phone)
	if !isValidMainlandPhone(phone) {
		return errors.New("手机号必须为 11 位数字")
	}
	user, err := s.users.FindByPhone(phone)
	if err != nil || user.Role != "merchant_admin" {
		return errors.New("商家账号不存在")
	}
	return s.issueMerchantVerificationCode(phone, "merchant_reset_password", &user.ID)
}

func (s *MerchantService) ResetPassword(req dto.MerchantResetPasswordRequest) error {
	phone := normalizePhone(req.Phone)
	req.SMSCode = strings.TrimSpace(req.SMSCode)
	if !isValidMainlandPhone(phone) {
		return errors.New("手机号必须为 11 位数字")
	}
	if len(req.NewPassword) < 6 {
		return errors.New("新密码至少 6 位")
	}
	if req.NewPassword != req.ConfirmPassword {
		return errors.New("两次输入的新密码不一致")
	}
	if err := s.consumeMerchantVerificationCode(phone, "merchant_reset_password", req.SMSCode); err != nil {
		return err
	}
	user, err := s.users.FindByPhone(phone)
	if err != nil || user.Role != "merchant_admin" {
		return errors.New("商家账号不存在")
	}
	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	return s.users.Save(user)
}

func (s *MerchantService) ChangePassword(userID uint, req dto.MerchantChangePasswordRequest) error {
	if len(req.NewPassword) < 6 {
		return errors.New("新密码至少 6 位")
	}
	if req.NewPassword != req.ConfirmPassword {
		return errors.New("两次输入的新密码不一致")
	}
	user, err := s.users.FindByID(userID)
	if err != nil || user.Role != "merchant_admin" {
		return errors.New("商家账号不存在")
	}
	if !utils.CheckPassword(user.PasswordHash, req.OldPassword) {
		return errors.New("原密码错误")
	}
	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	return s.users.Save(user)
}

func (s *MerchantService) RedeemSubscriptionCard(userID, merchantID uint, req dto.MerchantRedeemCardRequest) (*model.Merchant, *model.CardCode, error) {
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, nil, errors.New("请输入卡密")
	}
	card, err := s.cards.FindByCode(code)
	if err != nil {
		return nil, nil, errors.New("卡密不存在")
	}
	if card.Status != "unused" {
		return nil, nil, errors.New("卡密已被使用或已失效")
	}
	if card.ExpiredAt != nil && card.ExpiredAt.Before(time.Now()) {
		return nil, nil, errors.New("卡密已过期")
	}
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, nil, errors.New("商家信息不存在")
	}
	durationDays := card.Package.DurationDays
	if durationDays <= 0 && card.Package.IsLifetime {
		durationDays = 36500
	}
	if durationDays <= 0 {
		durationDays = 30
	}

	now := time.Now()
	base := now
	if merchant.SubscriptionExpireAt != nil && merchant.SubscriptionExpireAt.After(now) {
		base = *merchant.SubscriptionExpireAt
	}
	expire := base.AddDate(0, 0, durationDays)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		merchantRepo := repository.NewMerchantRepository(tx)
		cardRepo := repository.NewCardRepository(tx)

		merchant.SubscriptionStatus = "active"
		merchant.SubscriptionPlan = card.Package.Code
		merchant.SubscriptionExpireAt = &expire
		merchant.SubscriptionExpiredAt = &expire
		merchant.SubscriptionNote = fmt.Sprintf("卡密兑换：%s，延长 %d 天", card.Code, durationDays)
		if err := merchantRepo.Save(merchant); err != nil {
			return err
		}

		card.Status = "used"
		card.RedeemedBy = &userID
		card.RedeemedAt = &now
		return cardRepo.Save(card)
	})
	if err != nil {
		return nil, nil, err
	}
	return merchant, card, nil
}

func (s *MerchantService) GetInfo(merchantID uint) (*model.Merchant, error) {
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, errors.New("\u5546\u5bb6\u4fe1\u606f\u4e0d\u5b58\u5728")
	}
	return merchant, nil
}

func (s *MerchantService) SubscriptionInfo(merchantID uint) (map[string]interface{}, error) {
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, errors.New("\u5546\u5bb6\u4fe1\u606f\u4e0d\u5b58\u5728")
	}
	plans, err := s.plans.List()
	if err != nil {
		return nil, err
	}
	if syncMerchantSubscriptionFields(merchant, plans) {
		_ = s.merchants.Save(merchant)
	}
	active := merchant.SubscriptionExpireAt != nil && merchant.SubscriptionExpireAt.After(time.Now())
	return map[string]interface{}{
		"merchant":         merchant,
		"plans":            plans,
		"active":           active,
		"platform_payment": s.platformSubscriptionPaymentConfig(),
	}, nil
}

func (s *MerchantService) platformSubscriptionPaymentConfig() map[string]string {
	configs, err := s.systems.List()
	if err != nil {
		return map[string]string{}
	}
	result := map[string]string{}
	for _, item := range configs {
		switch item.ConfigKey {
		case "platform_alipay_qr_code", "platform_wechat_qr_code", "platform_subscription_note":
			result[item.ConfigKey] = item.ConfigValue
		}
	}
	return result
}

func (s *MerchantService) CheckSubscription(merchantID uint) (bool, error) {
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return false, errors.New("\u5546\u5bb6\u4fe1\u606f\u4e0d\u5b58\u5728")
	}
	expireAt := merchant.SubscriptionExpireAt
	if expireAt == nil {
		expireAt = merchant.SubscriptionExpiredAt
	}
	return expireAt != nil && expireAt.After(time.Now()), nil
}

func (s *MerchantService) SubscriptionOrderStatus(merchantID uint, orderNo string) (map[string]interface{}, error) {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, errors.New("\u8ba2\u5355\u4e0d\u5b58\u5728")
	}
	if order.OrderType != "merchant_subscription" || order.MerchantID == nil || *order.MerchantID != merchantID {
		return nil, errors.New("\u8ba2\u5355\u4e0d\u5b58\u5728")
	}
	valid, _ := s.CheckSubscription(merchantID)
	return map[string]interface{}{
		"order_no":    order.OrderNo,
		"status":      order.Status,
		"paid":        order.Status == "paid",
		"valid":       valid,
		"marked_paid": orderHasAction(order.OperationLogs, "merchant_subscription_paid_marked"),
	}, nil
}

func (s *MerchantService) MarkSubscriptionOrderPaid(merchantID uint, orderNo string) (*model.Order, error) {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, errors.New("订阅订单不存在")
	}
	if order.OrderType != "merchant_subscription" || order.MerchantID == nil || *order.MerchantID != merchantID {
		return nil, errors.New("订阅订单不存在")
	}
	if order.Status == "paid" {
		return order, nil
	}
	if order.Status != "pending" {
		return nil, errors.New("当前订阅订单不能标记付款")
	}
	if !orderHasAction(order.OperationLogs, "merchant_subscription_paid_marked") {
		order.OperationLogs = appendMerchantOrderLog(order.OperationLogs, "merchant_subscription_paid_marked", "商家已提交付款提醒，等待平台核对到账")
	}
	if strings.TrimSpace(order.PaymentChannel) == "" {
		order.PaymentChannel = "platform_qr"
	}
	if err := s.orders.Save(order); err != nil {
		return nil, err
	}
	return order, nil
}

func appendMerchantOrderLog(raw, action, text string) string {
	var logs []OrderOperationLog
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &logs)
	}
	logs = append(logs, OrderOperationLog{
		Action: action,
		Text:   text,
		Time:   time.Now().Format(time.RFC3339),
	})
	bytes, err := json.Marshal(logs)
	if err != nil {
		return raw
	}
	return string(bytes)
}

func syncMerchantSubscriptionFields(merchant *model.Merchant, plans []model.MerchantPlan) bool {
	if merchant == nil {
		return false
	}
	changed := false
	if merchant.SubscriptionExpireAt == nil && merchant.SubscriptionExpiredAt != nil {
		merchant.SubscriptionExpireAt = merchant.SubscriptionExpiredAt
		changed = true
	}
	if merchant.SubscriptionExpiredAt == nil && merchant.SubscriptionExpireAt != nil {
		merchant.SubscriptionExpiredAt = merchant.SubscriptionExpireAt
		changed = true
	}
	if merchant.SubscriptionPlanID == nil && merchant.SubscriptionPlan != "" {
		targetName := map[string]string{"month": "月付", "year": "年付"}[merchant.SubscriptionPlan]
		for _, plan := range plans {
			if plan.Name == targetName {
				merchant.SubscriptionPlanID = &plan.ID
				changed = true
				break
			}
		}
	}
	if merchant.SubscriptionStatus != "active" && merchant.SubscriptionExpireAt != nil && merchant.SubscriptionExpireAt.After(time.Now()) {
		merchant.SubscriptionStatus = "active"
		changed = true
	}
	return changed
}

func (s *MerchantService) CreateSubscriptionOrder(userID, merchantID uint, req dto.CreateMerchantSubscriptionOrderRequest) (*model.Order, error) {
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, errors.New("\u5546\u5bb6\u4fe1\u606f\u4e0d\u5b58\u5728")
	}
	if merchant.Status == "suspended" {
		return nil, errors.New("\u5546\u5bb6\u5df2\u88ab\u7981\u7528")
	}
	plan, err := s.plans.FindByID(req.PlanID)
	if err != nil {
		return nil, errors.New("\u8ba2\u9605\u5957\u9910\u4e0d\u5b58\u5728")
	}
	var fallbackPackage model.MembershipPackage
	if err := s.db.Order("id asc").First(&fallbackPackage).Error; err != nil {
		return nil, errors.New("\u7cfb\u7edf\u4f1a\u5458\u5957\u9910\u672a\u521d\u59cb\u5316")
	}
	channel := strings.TrimSpace(req.PaymentChannel)
	if channel == "" {
		channel = "alipay"
	}
	order := &model.Order{
		OrderNo:        fmt.Sprintf("MSUB%d", time.Now().UnixNano()),
		OrderType:      "merchant_subscription",
		UserID:         ptrUint(userID),
		MerchantID:     &merchantID,
		PackageID:      &fallbackPackage.ID,
		MerchantPlanID: &plan.ID,
		Amount:         plan.PriceCents,
		TotalAmount:    plan.PriceCents,
		Status:         "pending",
		PaymentChannel: channel,
	}
	if err := s.orders.Create(order); err != nil {
		return nil, err
	}
	order.Merchant = merchant
	order.MerchantPlan = plan
	order.Package = &fallbackPackage
	return order, nil
}

func (s *MerchantService) UpdateInfo(merchantID uint, req dto.UpdateMerchantInfoRequest) (*model.Merchant, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.ContactPhone = normalizePhone(req.ContactPhone)
	req.ContactEmail = strings.TrimSpace(strings.ToLower(req.ContactEmail))

	if req.Name == "" {
		return nil, errors.New("\u5546\u5bb6\u540d\u79f0\u4e0d\u80fd\u4e3a\u7a7a")
	}
	if !isValidMainlandPhone(req.ContactPhone) {
		return nil, errors.New("\u8054\u7cfb\u4eba\u624b\u673a\u53f7\u5fc5\u987b\u4e3a 11 \u4f4d\u6570\u5b57")
	}

	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, errors.New("\u5546\u5bb6\u4fe1\u606f\u4e0d\u5b58\u5728")
	}

	if merchant.Name != req.Name {
		if existing, err := s.merchants.FindByName(req.Name); err == nil && existing.ID != merchantID {
			return nil, errors.New("\u5546\u5bb6\u540d\u79f0\u5df2\u5b58\u5728\uff0c\u8bf7\u66f4\u6362\u540e\u91cd\u8bd5")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	merchant.Name = req.Name
	merchant.ContactPhone = req.ContactPhone
	merchant.ContactEmail = req.ContactEmail

	if err := s.merchants.Save(merchant); err != nil {
		return nil, err
	}

	return merchant, nil
}

func (s *MerchantService) PaymentConfig(merchantID uint) (*model.MerchantPaymentConfig, error) {
	var config model.MerchantPaymentConfig
	err := s.db.Preload("Merchant").Where("merchant_id = ?", merchantID).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		merchant, merchantErr := s.merchants.FindByID(merchantID)
		if merchantErr != nil {
			return nil, errors.New("商家信息不存在")
		}
		return &model.MerchantPaymentConfig{
			MerchantID:   merchantID,
			Merchant:     *merchant,
			Channel:      "alipay",
			Mode:         "direct",
			Status:       "disabled",
			AuditStatus:  "pending",
			ContactPhone: merchant.ContactPhone,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *MerchantService) SavePaymentConfig(merchantID uint, req dto.SaveMerchantPaymentConfigRequest) (*model.MerchantPaymentConfig, error) {
	channel := strings.TrimSpace(req.Channel)
	mode := strings.TrimSpace(req.Mode)
	accountName := strings.TrimSpace(req.AccountName)
	accountNo := strings.TrimSpace(req.AccountNo)
	alipayQRCode := strings.TrimSpace(req.AlipayQRCode)
	wechatQRCode := strings.TrimSpace(req.WechatQRCode)
	appID := strings.TrimSpace(req.AppID)
	contactPhone := normalizePhone(req.ContactPhone)
	remark := strings.TrimSpace(req.Remark)
	if mode == "" {
		mode = "direct"
	}
	if channel != "alipay" && channel != "wechat" && channel != "bank" {
		return nil, errors.New("收款渠道无效")
	}
	if mode != "direct" && mode != "platform" && mode != "service_provider" {
		return nil, errors.New("收款模式无效")
	}
	if accountName == "" || accountNo == "" {
		return nil, errors.New("请填写收款账户名称和账号")
	}
	if contactPhone != "" && !isValidMainlandPhone(contactPhone) {
		return nil, errors.New("联系电话必须为 11 位数字")
	}
	if len([]rune(remark)) > 255 {
		return nil, errors.New("备注不能超过 255 字")
	}
	if len([]rune(alipayQRCode)) > 1000 || len([]rune(wechatQRCode)) > 1000 {
		return nil, errors.New("收款码链接不能超过 1000 字")
	}
	if mode == "direct" && alipayQRCode == "" && wechatQRCode == "" {
		return nil, errors.New("商家收款码模式至少需要填写一个收款码")
	}
	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, errors.New("商家信息不存在")
	}

	var config model.MerchantPaymentConfig
	err := s.db.Where("merchant_id = ?", merchantID).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		config = model.MerchantPaymentConfig{MerchantID: merchantID}
	} else if err != nil {
		return nil, err
	}

	config.Channel = channel
	config.Mode = mode
	config.AccountName = accountName
	config.AccountNo = accountNo
	config.AlipayQRCode = alipayQRCode
	config.WechatQRCode = wechatQRCode
	config.AppID = appID
	config.ContactPhone = contactPhone
	config.Remark = remark
	config.Status = "disabled"
	config.AuditStatus = "pending"
	config.AuditRemark = "商家提交或修改收款信息，等待平台审核"

	if err := s.db.Save(&config).Error; err != nil {
		return nil, err
	}
	return s.PaymentConfig(merchantID)
}

func (s *MerchantService) ReviewPaymentConfig(merchantID uint, req dto.ReviewMerchantPaymentConfigRequest) (*model.MerchantPaymentConfig, error) {
	var config model.MerchantPaymentConfig
	if err := s.db.Where("merchant_id = ?", merchantID).First(&config).Error; err != nil {
		return nil, errors.New("商家收款配置不存在")
	}
	status := strings.TrimSpace(req.Status)
	auditStatus := strings.TrimSpace(req.AuditStatus)
	remark := strings.TrimSpace(req.AuditRemark)
	if auditStatus != "pending" && auditStatus != "approved" && auditStatus != "rejected" {
		return nil, errors.New("审核状态无效")
	}
	if status == "" {
		if auditStatus == "approved" {
			status = "enabled"
		} else {
			status = "disabled"
		}
	}
	if status != "enabled" && status != "disabled" {
		return nil, errors.New("收款状态无效")
	}
	config.Status = status
	config.AuditStatus = auditStatus
	config.AuditRemark = remark
	if err := s.db.Save(&config).Error; err != nil {
		return nil, err
	}
	return s.PaymentConfig(merchantID)
}

func (s *MerchantService) ListStores(merchantID uint, page, pageSize int) ([]model.Store, int64, error) {
	return s.stores.ListByMerchant(merchantID, (page-1)*pageSize, pageSize)
}

func (s *MerchantService) CreateStore(merchantID uint, req dto.CreateStoreRequest) (*model.Store, error) {
	name := strings.TrimSpace(req.Name)
	address := strings.TrimSpace(req.Address)
	phone := normalizePhone(req.ContactPhone)
	businessHours := strings.TrimSpace(req.BusinessHours)
	pauseReason := strings.TrimSpace(req.PauseReason)
	orderMode := normalizeStoreOrderMode(req.OrderMode)
	isOpen := true
	if req.IsOpen != nil {
		isOpen = *req.IsOpen
	}
	autoAccept := orderMode == "submit_later"
	if req.AutoAccept != nil {
		autoAccept = *req.AutoAccept
	}

	if name == "" {
		return nil, errors.New("\u95e8\u5e97\u540d\u79f0\u4e0d\u80fd\u4e3a\u7a7a")
	}
	if !isValidMainlandPhone(phone) {
		return nil, errors.New("\u8054\u7cfb\u7535\u8bdd\u5fc5\u987b\u4e3a 11 \u4f4d\u6570\u5b57")
	}

	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, errors.New("\u5546\u5bb6\u4fe1\u606f\u4e0d\u5b58\u5728")
	}
	if _, err := s.stores.FindByMerchantAndName(merchantID, name); err == nil {
		return nil, errors.New("\u540c\u4e00\u5546\u5bb6\u4e0b\u95e8\u5e97\u540d\u79f0\u4e0d\u80fd\u91cd\u590d")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	store := &model.Store{
		MerchantID:    merchantID,
		Name:          name,
		Address:       address,
		ContactPhone:  phone,
		Status:        "active",
		IsOpen:        isOpen,
		BusinessHours: businessHours,
		PauseReason:   pauseReason,
		OrderMode:     orderMode,
		AutoAccept:    autoAccept,
	}
	if err := s.stores.Create(store); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *MerchantService) UpdateStore(merchantID, storeID uint, req dto.UpdateStoreRequest) (*model.Store, error) {
	name := strings.TrimSpace(req.Name)
	address := strings.TrimSpace(req.Address)
	phone := normalizePhone(req.ContactPhone)
	status := strings.TrimSpace(req.Status)
	businessHours := strings.TrimSpace(req.BusinessHours)
	pauseReason := strings.TrimSpace(req.PauseReason)
	orderMode := normalizeStoreOrderMode(req.OrderMode)

	if name == "" {
		return nil, errors.New("\u95e8\u5e97\u540d\u79f0\u4e0d\u80fd\u4e3a\u7a7a")
	}
	if !isValidMainlandPhone(phone) {
		return nil, errors.New("\u8054\u7cfb\u7535\u8bdd\u5fc5\u987b\u4e3a 11 \u4f4d\u6570\u5b57")
	}
	if status != "" && status != "active" && status != "inactive" {
		return nil, errors.New("\u95e8\u5e97\u72b6\u6001\u65e0\u6548")
	}

	store, err := s.stores.FindByMerchantAndID(merchantID, storeID)
	if err != nil {
		return nil, errors.New("\u95e8\u5e97\u4e0d\u5b58\u5728")
	}

	if store.Name != name {
		if existing, err := s.stores.FindByMerchantAndName(merchantID, name); err == nil && existing.ID != storeID {
			return nil, errors.New("\u540c\u4e00\u5546\u5bb6\u4e0b\u95e8\u5e97\u540d\u79f0\u4e0d\u80fd\u91cd\u590d")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	store.Name = name
	store.Address = address
	store.ContactPhone = phone
	store.BusinessHours = businessHours
	store.PauseReason = pauseReason
	store.OrderMode = orderMode
	if req.IsOpen != nil {
		store.IsOpen = *req.IsOpen
	}
	if req.AutoAccept != nil {
		store.AutoAccept = *req.AutoAccept
	}
	if status != "" {
		store.Status = status
	}

	if err := s.stores.Save(store); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *MerchantService) DisableStore(merchantID, storeID uint) error {
	store, err := s.stores.FindByMerchantAndID(merchantID, storeID)
	if err != nil {
		return errors.New("\u95e8\u5e97\u4e0d\u5b58\u5728")
	}
	store.Status = "inactive"
	return s.stores.Save(store)
}

func normalizeStoreOrderMode(value string) string {
	value = strings.TrimSpace(value)
	if value == "submit_later" {
		return "submit_later"
	}
	return "pay_first"
}

func (s *MerchantService) ListStoreProducts(merchantID, storeID uint, page, pageSize int) ([]model.StoreProduct, int64, error) {
	if _, err := s.stores.FindByMerchantAndID(merchantID, storeID); err != nil {
		return nil, 0, errors.New("\u95e8\u5e97\u4e0d\u5b58\u5728")
	}
	return s.products.ListByStore(storeID, (page-1)*pageSize, pageSize)
}

func (s *MerchantService) SaveStoreProduct(merchantID, productID uint, req dto.SaveStoreProductRequest) (*model.StoreProduct, error) {
	name := strings.TrimSpace(req.Name)
	description := strings.TrimSpace(req.Description)
	imageURL := strings.TrimSpace(req.ImageURL)
	category := strings.TrimSpace(req.Category)
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}
	if category == "" {
		category = "默认分类"
	}
	if name == "" {
		return nil, errors.New("\u5546\u54c1\u540d\u79f0\u4e0d\u80fd\u4e3a\u7a7a")
	}
	if req.Price < 0 {
		return nil, errors.New("\u5546\u54c1\u4ef7\u683c\u4e0d\u80fd\u4e3a\u8d1f\u6570")
	}
	if status != "active" && status != "inactive" {
		return nil, errors.New("\u5546\u54c1\u72b6\u6001\u65e0\u6548")
	}
	if _, err := s.stores.FindByMerchantAndID(merchantID, req.StoreID); err != nil {
		return nil, errors.New("\u95e8\u5e97\u4e0d\u5b58\u5728")
	}

	if productID == 0 {
		product := &model.StoreProduct{
			StoreID:     req.StoreID,
			Name:        name,
			Price:       req.Price,
			Description: description,
			ImageURL:    imageURL,
			Category:    category,
			Status:      status,
			Stock:       req.Stock,
			Sort:        req.Sort,
		}
		if product.Sort == 0 {
			product.Sort = 100
		}
		if err := s.products.Create(product); err != nil {
			return nil, err
		}
		return product, nil
	}

	product, err := s.products.FindByID(productID)
	if err != nil || product.Store.MerchantID != merchantID {
		return nil, errors.New("\u5546\u54c1\u4e0d\u5b58\u5728")
	}
	product.StoreID = req.StoreID
	product.Name = name
	product.Price = req.Price
	product.Description = description
	product.ImageURL = imageURL
	product.Category = category
	product.Status = status
	product.Stock = req.Stock
	product.Sort = req.Sort
	if product.Sort == 0 {
		product.Sort = 100
	}
	if err := s.products.Save(product); err != nil {
		return nil, err
	}
	return product, nil
}

func (s *MerchantService) DisableStoreProduct(merchantID, productID uint) error {
	product, err := s.products.FindByID(productID)
	if err != nil || product.Store.MerchantID != merchantID {
		return errors.New("\u5546\u54c1\u4e0d\u5b58\u5728")
	}
	product.Status = "inactive"
	return s.products.Save(product)
}

func (s *MerchantService) ListPromotions(merchantID uint, page, pageSize int) ([]model.Promotion, int64, error) {
	return s.promotions.ListByMerchant(merchantID, (page-1)*pageSize, pageSize)
}

func (s *MerchantService) SavePromotion(merchantID, promotionID uint, req dto.SavePromotionRequest) (*model.Promotion, error) {
	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	promotionType := strings.TrimSpace(req.Type)
	status := strings.TrimSpace(req.Status)
	if promotionType == "" {
		promotionType = "amount"
	}
	if status == "" {
		status = "draft"
	}
	if title == "" {
		return nil, errors.New("\u6d3b\u52a8\u6807\u9898\u4e0d\u80fd\u4e3a\u7a7a")
	}
	if status != "draft" && status != "published" && status != "inactive" {
		return nil, errors.New("\u6d3b\u52a8\u72b6\u6001\u65e0\u6548")
	}
	if promotionType != "amount" && promotionType != "discount" {
		return nil, errors.New("\u6d3b\u52a8\u7c7b\u578b\u65e0\u6548")
	}
	if req.Threshold < 0 || req.Discount < 0 {
		return nil, errors.New("\u95e8\u69db\u548c\u4f18\u60e0\u91d1\u989d\u4e0d\u80fd\u4e3a\u8d1f\u6570")
	}
	if promotionType == "discount" && (req.DiscountRate <= 0 || req.DiscountRate >= 100) {
		return nil, errors.New("\u6298\u6263\u6d3b\u52a8\u9700\u8981\u586b\u5199 1-99 \u4e4b\u95f4\u7684\u6298\u7387")
	}
	if req.StoreID != nil {
		if _, err := s.stores.FindByMerchantAndID(merchantID, *req.StoreID); err != nil {
			return nil, errors.New("\u95e8\u5e97\u4e0d\u5b58\u5728")
		}
	}
	validFrom, err := parsePromotionTime(req.ValidFrom)
	if err != nil {
		return nil, err
	}
	validTo, err := parsePromotionTime(req.ValidTo)
	if err != nil {
		return nil, err
	}
	if validFrom != nil && validTo != nil && validFrom.After(*validTo) {
		return nil, errors.New("\u5f00\u59cb\u65f6\u95f4\u4e0d\u80fd\u665a\u4e8e\u7ed3\u675f\u65f6\u95f4")
	}

	if promotionID == 0 {
		promotion := &model.Promotion{
			MerchantID:   merchantID,
			StoreID:      req.StoreID,
			Title:        title,
			Description:  description,
			Type:         promotionType,
			Threshold:    req.Threshold,
			Discount:     req.Discount,
			DiscountRate: req.DiscountRate,
			Status:       status,
			ValidFrom:    validFrom,
			ValidTo:      validTo,
		}
		if err := s.promotions.Create(promotion); err != nil {
			return nil, err
		}
		return promotion, nil
	}

	promotion, err := s.promotions.FindByMerchantAndID(merchantID, promotionID)
	if err != nil {
		return nil, errors.New("\u6d3b\u52a8\u4e0d\u5b58\u5728")
	}
	promotion.StoreID = req.StoreID
	promotion.Title = title
	promotion.Description = description
	promotion.Type = promotionType
	promotion.Threshold = req.Threshold
	promotion.Discount = req.Discount
	promotion.DiscountRate = req.DiscountRate
	promotion.Status = status
	promotion.ValidFrom = validFrom
	promotion.ValidTo = validTo
	if err := s.promotions.Save(promotion); err != nil {
		return nil, err
	}
	return promotion, nil
}

func (s *MerchantService) DisablePromotion(merchantID, promotionID uint) error {
	promotion, err := s.promotions.FindByMerchantAndID(merchantID, promotionID)
	if err != nil {
		return errors.New("\u6d3b\u52a8\u4e0d\u5b58\u5728")
	}
	promotion.Status = "inactive"
	return s.promotions.Save(promotion)
}

func (s *MerchantService) DeletePromotion(merchantID, promotionID uint) error {
	promotion, err := s.promotions.FindByMerchantAndID(merchantID, promotionID)
	if err != nil {
		return errors.New("\u6d3b\u52a8\u4e0d\u5b58\u5728")
	}
	return s.promotions.Delete(promotion)
}

func (s *MerchantService) GeneratePromotionDraft(merchantID uint, req dto.GeneratePromotionDraftRequest) (*model.Promotion, error) {
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, errors.New("\u5546\u5bb6\u4fe1\u606f\u4e0d\u5b58\u5728")
	}

	stats, err := s.leads.CountByStatus(merchantID)
	if err != nil {
		return nil, err
	}

	totalLeads := stats["new"] + stats["contacted"] + stats["converted"] + stats["invalid"]
	conversionRate := 0.0
	if totalLeads > 0 {
		conversionRate = float64(stats["converted"]) / float64(totalLeads)
	}

	title := "\u65b0\u5ba2\u626b\u7801\u5230\u5e97\u7acb\u51cf\u6d3b\u52a8"
	description := "\u9488\u5bf9\u9996\u6b21\u626b\u7801\u7559\u8d44\u7684\u987e\u5ba2\uff0c\u5f15\u5bfc\u5230\u5e97\u54a8\u8be2\u5e76\u4eab\u53d7\u4e13\u5c5e\u4f18\u60e0\u3002"
	threshold := int64(10000)
	discount := int64(2000)

	if stats["new"] > 0 {
		title = "\u65b0\u7ebf\u7d22 24 \u5c0f\u65f6\u5230\u5e97\u798f\u5229"
		description = "\u76ee\u524d\u5b58\u5728\u672a\u8ddf\u8fdb\u65b0\u7ebf\u7d22\uff0c\u5efa\u8bae\u7528\u77ed\u65f6\u6548\u4f18\u60e0\u63d0\u5347\u56de\u8bbf\u548c\u5230\u5e97\u7387\u3002"
		threshold = 5000
		discount = 1000
	}
	if stats["contacted"] > stats["converted"] {
		title = "\u5df2\u54a8\u8be2\u5ba2\u6237\u9650\u65f6\u8f6c\u5316\u5238"
		description = "\u5df2\u8054\u7cfb\u7ebf\u7d22\u591a\u4e8e\u6210\u4ea4\u7ebf\u7d22\uff0c\u5efa\u8bae\u4ee5\u9650\u65f6\u798f\u5229\u63a8\u52a8\u4e8c\u6b21\u5230\u5e97\u548c\u51b3\u7b56\u3002"
		threshold = 20000
		discount = 3000
	}
	if conversionRate >= 0.3 {
		title = "\u9ad8\u8f6c\u5316\u590d\u8d2d\u52a0\u7801\u6d3b\u52a8"
		description = "\u5f53\u524d\u8f6c\u5316\u8868\u73b0\u8f83\u597d\uff0c\u53ef\u4ee5\u5c06\u73b0\u6709\u8bdd\u672f\u548c\u4f18\u60e0\u590d\u7528\u5230\u66f4\u591a\u987e\u5ba2\u573a\u666f\u3002"
		threshold = 30000
		discount = 5000
	}
	switch strings.TrimSpace(req.CustomerTag) {
	case "新顾客", "新客", "new":
		title = "新客复购券"
		description = "面向首次下单或刚完成首单的顾客，鼓励 3-7 天内再次消费，尽快形成复购习惯。"
		threshold = 5000
		discount = 800
	case "复购顾客", "repeat":
		title = "复购加码券"
		description = "面向已有复购行为的顾客，适合搭配热销商品或套餐，提高客单价和购买频次。"
		threshold = 10000
		discount = 1500
	case "沉睡顾客", "流失风险", "sleeping", "risk":
		title = "沉睡召回券"
		description = "面向长期未下单或最近订单关闭的顾客，使用低门槛优惠降低回流阻力。"
		threshold = 3000
		discount = 1000
	case "高价值顾客", "high_value":
		title = "高价值专属券"
		description = "面向累计消费高或多次复购的核心顾客，建议作为专属权益进行维护。"
		threshold = 20000
		discount = 3000
	}

	validFrom := time.Now()
	validTo := validFrom.AddDate(0, 0, 14)
	promotion := &model.Promotion{
		MerchantID:  merchant.ID,
		Title:       title,
		Description: description,
		Type:        "amount",
		Threshold:   threshold,
		Discount:    discount,
		Status:      "draft",
		ValidFrom:   &validFrom,
		ValidTo:     &validTo,
	}
	if err := s.promotions.Create(promotion); err != nil {
		return nil, err
	}
	return promotion, nil
}

func (s *MerchantService) CustomerStoreURL(storeID uint) string {
	base := strings.TrimRight(s.cfg.FrontendURL, "/")
	return fmt.Sprintf("%s/customer/store/%d", base, storeID)
}

func (s *MerchantService) GetPublicStore(storeID uint) (*model.Store, error) {
	store, err := s.stores.FindByIDWithMerchant(storeID)
	if err != nil {
		return nil, errors.New("\u95e8\u5e97\u4e0d\u5b58\u5728")
	}
	return store, nil
}

func (s *MerchantService) ListPublicPromotions(storeID uint) ([]model.Promotion, error) {
	store, err := s.stores.FindByIDWithMerchant(storeID)
	if err != nil {
		return nil, errors.New("\u95e8\u5e97\u4e0d\u5b58\u5728")
	}
	return s.promotions.ListActiveForStore(store.MerchantID, store.ID)
}

func (s *MerchantService) CreateCustomerLead(storeID uint, req dto.CreateCustomerLeadRequest) (*model.CustomerLead, error) {
	name := strings.TrimSpace(req.CustomerName)
	phone := normalizePhone(req.CustomerPhone)
	message := strings.TrimSpace(req.Message)

	if !isValidMainlandPhone(phone) {
		return nil, errors.New("\u8bf7\u8f93\u5165 11 \u4f4d\u624b\u673a\u53f7")
	}
	if len(message) > 500 {
		return nil, errors.New("\u7559\u8a00\u5185\u5bb9\u4e0d\u80fd\u8d85\u8fc7 500 \u5b57")
	}

	store, err := s.stores.FindByIDWithMerchant(storeID)
	if err != nil {
		return nil, errors.New("\u95e8\u5e97\u4e0d\u5b58\u5728")
	}
	if store.Status != "active" || store.Merchant.Status != "active" {
		return nil, errors.New("\u5f53\u524d\u95e8\u5e97\u6682\u4e0d\u53ef\u63d0\u4ea4\u7ebf\u7d22")
	}

	lead := &model.CustomerLead{
		MerchantID:    store.MerchantID,
		StoreID:       store.ID,
		CustomerName:  name,
		CustomerPhone: phone,
		Message:       message,
		Source:        "store_qr",
		Status:        "new",
	}
	if err := s.leads.Create(lead); err != nil {
		return nil, err
	}
	return lead, nil
}

func (s *MerchantService) GenerateAIMarketingCopy(merchantID uint, req dto.GenerateAIMarketingCopyRequest) (*AIResult, error) {
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, errors.New("商家不存在")
	}
	quota, err := s.AIQuota(merchantID)
	if err != nil {
		return nil, err
	}
	if intValue(quota["remaining"]) <= 0 {
		return nil, errors.New("今日 AI 生成次数已用完，请明天再试或升级订阅套餐")
	}
	stats := s.marketingStatsSummary(merchantID)
	goal := strings.TrimSpace(req.Goal)
	if goal == "" {
		goal = "提升新客到店、复购和转介绍"
	}
	customerTag := strings.TrimSpace(req.CustomerTag)
	if customerTag == "" {
		customerTag = "新客与复购顾客"
	}
	scenario := strings.TrimSpace(req.Scenario)
	if scenario == "" {
		scenario = "裂变海报"
	}

	systemPrompt := "你是本地生活商家AI运营顾问，擅长餐饮、零售、服务业的低成本获客、复购和裂变活动。请输出中文，内容必须可直接用于商家后台。"
	userPrompt := fmt.Sprintf(`商家名称：%s
使用场景：%s
目标人群：%s
目标：%s
主推商品：%s
经营数据摘要：%v

请生成一份可执行的AI营销方案，格式包含：
1. 活动标题
2. 海报主文案
3. 优惠/奖励机制
4. 顾客分享话术
5. 商家执行步骤
6. 风险提醒和成本控制
要求：短句、适合手机海报、不要夸大承诺。`, merchant.Name, scenario, customerTag, goal, strings.TrimSpace(req.ProductName), stats)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.AITimeoutSeconds)*time.Second)
	defer cancel()
	start := time.Now()
	result, err := s.ai.Generate(ctx, AIRequest{SystemPrompt: systemPrompt, UserPrompt: userPrompt})
	if err != nil {
		_ = s.recordAIUsage(merchantID, scenario, nil, err, time.Since(start))
		return nil, err
	}
	_ = s.recordAIUsage(merchantID, scenario, result, nil, time.Since(start))
	return result, nil
}

func (s *MerchantService) GenerateAIReferralCopies(merchantID uint, req dto.GenerateAIReferralCopyRequest) (map[string]interface{}, error) {
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, errors.New("商家不存在")
	}
	quota, err := s.AIQuota(merchantID)
	if err != nil {
		return nil, err
	}
	if intValue(quota["remaining"]) <= 0 {
		return nil, errors.New("今日 AI 生成次数已用完，请明天再试或升级订阅套餐")
	}

	shareService := NewShareService(s.db)
	config, err := shareService.GetActivityConfig(merchantID)
	if err != nil {
		return nil, err
	}
	stats, campaigns, _, err := shareService.StatsByMerchant(merchantID, ShareStatsParams{})
	if err != nil {
		return nil, err
	}
	businessStats := s.marketingStatsSummary(merchantID)
	productName := strings.TrimSpace(req.ProductName)
	if productName == "" {
		productName = s.bestProductName(merchantID)
	}
	if productName == "" {
		productName = "门店招牌商品/服务"
	}
	tone := strings.TrimSpace(req.Tone)
	if tone == "" {
		tone = "亲切、可信、适合本地生活商家"
	}
	goal := strings.TrimSpace(req.Goal)
	if goal == "" {
		goal = "提升分享扫码、好友首单和老客复购"
	}

	shareSummary := map[string]interface{}{
		"campaigns":          stats.TotalCampaigns,
		"scan_count":         stats.ScanCount,
		"coupon_count":       stats.RewardCouponCount,
		"used_coupon_count":  stats.UsedCouponCount,
		"conversion_count":   stats.ConversionCount,
		"conversion_amount":  stats.ConversionAmount,
		"friend_coupon":      config.FriendCouponAmount,
		"friend_threshold":   config.FriendCouponThreshold,
		"referrer_coupon":    config.ReferrerCouponAmount,
		"referrer_threshold": config.ReferrerCouponThreshold,
		"valid_days":         config.ValidDays,
	}
	if len(campaigns) > 0 {
		shareSummary["best_campaign_scan_count"] = campaigns[0].ScanCount
		shareSummary["best_campaign_conversion_count"] = campaigns[0].ConversionCount
	}

	systemPrompt := "你是本地生活商家 AI 增长顾问，擅长把订单、优惠券和裂变数据转化成可直接发布的营销内容。请输出中文，内容要真实克制、可落地，不要夸大承诺。"
	userPrompt := fmt.Sprintf(`商家名称：%s
主推商品/服务：%s
语气要求：%s
营销目标：%s
经营数据：%v
裂变数据和券配置：%v

请生成三套可直接使用的内容，必须按以下标题输出：
【朋友圈文案】
【社群话术】
【短视频口播脚本】

每套内容要求：
- 朋友圈文案 80-150 字，突出好友券和复购奖励。
- 社群话术 60-120 字，适合微信群/客户群。
- 短视频口播脚本 15-30 秒，包含镜头建议和口播词。
- 明确提醒商家观察扫码、领券、下单和核销数据。`, merchant.Name, productName, tone, goal, businessStats, shareSummary)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.AITimeoutSeconds)*time.Second)
	defer cancel()
	start := time.Now()
	result, err := s.ai.Generate(ctx, AIRequest{SystemPrompt: systemPrompt, UserPrompt: userPrompt})
	if err != nil {
		_ = s.recordAIUsage(merchantID, "referral_multi_copy", nil, err, time.Since(start))
		return nil, err
	}
	_ = s.recordAIUsage(merchantID, "referral_multi_copy", result, nil, time.Since(start))

	variants := buildReferralCopyVariants(merchant.Name, productName, config, stats, result.Content)
	return map[string]interface{}{
		"variants": variants,
		"raw":      result,
		"quota":    mustQuota(s.AIQuota(merchantID)),
	}, nil
}

func (s *MerchantService) GenerateAIShareReview(merchantID uint, req dto.GenerateAIShareReviewRequest) (map[string]interface{}, error) {
	merchant, err := s.merchants.FindByID(merchantID)
	if err != nil {
		return nil, errors.New("商家不存在")
	}
	quota, err := s.AIQuota(merchantID)
	if err != nil {
		return nil, err
	}
	if intValue(quota["remaining"]) <= 0 {
		return nil, errors.New("今日 AI 生成次数已用完，请明天再试或升级订阅套餐")
	}

	params, err := parseAIShareReviewRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	shareService := NewShareService(s.db)
	config, err := shareService.GetActivityConfig(merchantID)
	if err != nil {
		return nil, err
	}
	stats, campaigns, coupons, err := shareService.StatsByMerchant(merchantID, params)
	if err != nil {
		return nil, err
	}

	review := buildShareReviewDecision(config, stats, campaigns, coupons)
	systemPrompt := "你是本地生活商家的 AI 活动复盘顾问。请基于裂变海报、扫码、领券、核销、下单和转化金额数据，判断活动是否应该加大优惠、继续观察、优化文案或停用。输出中文，简短、可执行。"
	userPrompt := fmt.Sprintf(`商家名称：%s
裂变配置：好友券%s，门槛%s，分享人奖励%s，有效期%d天
复盘数据：海报%d张，扫码%d次，发券%d张，已核销%d张，过期%d张，作废%d张，转化订单%d单，转化金额%s
系统初步判断：%v

请给出：
1. 总体结论
2. 哪类文案或海报值得继续推
3. 哪张券核销率高或低
4. 是否建议加大优惠、降低门槛、继续观察或停用活动
5. 下一步 3 个动作`, merchant.Name, formatFen(config.FriendCouponAmount), formatFen(config.FriendCouponThreshold), formatFen(config.ReferrerCouponAmount), config.ValidDays, stats.TotalCampaigns, stats.ScanCount, stats.RewardCouponCount, stats.UsedCouponCount, stats.ExpiredCouponCount, stats.VoidedCouponCount, stats.ConversionCount, formatFen(stats.ConversionAmount), review)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.AITimeoutSeconds)*time.Second)
	defer cancel()
	start := time.Now()
	result, err := s.ai.Generate(ctx, AIRequest{SystemPrompt: systemPrompt, UserPrompt: userPrompt})
	if err != nil {
		_ = s.recordAIUsage(merchantID, "share_campaign_review", nil, err, time.Since(start))
		return nil, err
	}
	_ = s.recordAIUsage(merchantID, "share_campaign_review", result, nil, time.Since(start))

	return map[string]interface{}{
		"review": review,
		"raw":    result,
		"quota":  mustQuota(s.AIQuota(merchantID)),
	}, nil
}

func (s *MerchantService) AIQuota(merchantID uint) (map[string]interface{}, error) {
	var merchant model.Merchant
	if err := s.db.Preload("MerchantPlan").First(&merchant, merchantID).Error; err != nil {
		return nil, errors.New("商家不存在")
	}
	limit := s.aiDailyLimit(&merchant)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	var used int64
	if err := s.db.Model(&model.MerchantAIUsageLog{}).
		Where("merchant_id = ? AND used_at >= ? AND used_at < ?", merchantID, start, end).
		Count(&used).Error; err != nil {
		return nil, err
	}
	remaining := limit - int(used)
	if remaining < 0 {
		remaining = 0
	}
	return map[string]interface{}{
		"limit":      limit,
		"used":       used,
		"remaining":  remaining,
		"reset_at":   end,
		"plan":       merchant.SubscriptionPlan,
		"plan_id":    merchant.SubscriptionPlanID,
		"ai_enabled": s.cfg.AIEnabled,
		"provider":   s.cfg.AIProvider,
		"model":      s.cfg.AIModel,
	}, nil
}

func (s *MerchantService) aiDailyLimit(merchant *model.Merchant) int {
	if merchant == nil {
		return 5
	}
	if merchant.SubscriptionExpireAt == nil || merchant.SubscriptionExpireAt.Before(time.Now()) {
		return 5
	}
	if merchant.MerchantPlan != nil {
		if merchant.MerchantPlan.DurationDays >= 365 {
			return 100
		}
		if merchant.MerchantPlan.DurationDays >= 30 {
			return 30
		}
	}
	plan := strings.ToLower(merchant.SubscriptionPlan)
	if strings.Contains(plan, "year") || strings.Contains(plan, "年") {
		return 100
	}
	if strings.Contains(plan, "month") || strings.Contains(plan, "月") {
		return 30
	}
	return 10
}

func (s *MerchantService) recordAIUsage(merchantID uint, scenario string, result *AIResult, callErr error, duration time.Duration) error {
	provider := strings.TrimSpace(s.cfg.AIProvider)
	modelName := strings.TrimSpace(s.cfg.AIModel)
	fallback := false
	success := callErr == nil
	errorText := ""
	if result != nil {
		provider = strings.TrimSpace(result.Provider)
		modelName = strings.TrimSpace(result.Model)
		fallback = result.Fallback
	}
	if callErr != nil {
		errorText = trimRunes(callErr.Error(), 500)
	}
	log := &model.MerchantAIUsageLog{
		MerchantID: merchantID,
		Scenario:   strings.TrimSpace(scenario),
		Model:      modelName,
		Provider:   provider,
		Fallback:   fallback,
		Success:    success,
		Error:      errorText,
		DurationMS: int(duration.Milliseconds()),
		UsedAt:     time.Now(),
	}
	return s.db.Create(log).Error
}

func intValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func (s *MerchantService) bestProductName(merchantID uint) string {
	var product model.StoreProduct
	if err := s.db.Joins("JOIN stores ON stores.id = store_products.store_id").
		Where("stores.merchant_id = ? AND store_products.status = ?", merchantID, "active").
		Order("store_products.sort ASC, store_products.id DESC").
		First(&product).Error; err == nil {
		return strings.TrimSpace(product.Name)
	}
	return ""
}

func buildReferralCopyVariants(merchantName, productName string, config *model.ShareActivityConfig, stats *ShareStats, raw string) []map[string]string {
	sections := parseReferralSections(raw)
	if len(sections) >= 3 {
		return sections
	}

	friendCoupon := int64(500)
	friendThreshold := int64(3000)
	referrerCoupon := int64(500)
	if config != nil {
		friendCoupon = config.FriendCouponAmount
		friendThreshold = config.FriendCouponThreshold
		referrerCoupon = config.ReferrerCouponAmount
	}
	if strings.TrimSpace(merchantName) == "" {
		merchantName = "本店"
	}
	if strings.TrimSpace(productName) == "" {
		productName = "招牌商品"
	}
	scanCount := int64(0)
	conversionCount := int64(0)
	if stats != nil {
		scanCount = stats.ScanCount
		conversionCount = stats.ConversionCount
	}

	friend := formatFen(friendCoupon)
	threshold := formatFen(friendThreshold)
	referrer := formatFen(referrerCoupon)
	return []map[string]string{
		{
			"type":  "朋友圈版",
			"title": "老客分享领券",
			"text":  fmt.Sprintf("%s 最近在做好友福利，推荐 %s。好友扫码可领 %s 优惠券，满 %s 可用；分享人下次复购也能得 %s 奖励券。", merchantName, productName, friend, threshold, referrer),
			"copy":  fmt.Sprintf("我刚在 %s 发现一个不错的福利：好友扫码领 %s 券，满 %s 可用。推荐先试试%s，适合想省钱又想尝鲜的朋友。", merchantName, friend, threshold, productName),
		},
		{
			"type":  "社群版",
			"title": "好友券社群话术",
			"text":  fmt.Sprintf("今天给群友准备了 %s 好友券，满 %s 可用；通过分享海报下单后，老顾客还能得复购券。", friend, threshold),
			"copy":  fmt.Sprintf("群友福利来了：%s 的 %s 好友券，满 %s 可用。扫码进店即可领取，下单后还有复购奖励，数量有限，先到先得。", merchantName, friend, threshold),
		},
		{
			"type":  "短视频口播版",
			"title": "30 秒门店种草脚本",
			"text":  fmt.Sprintf("当前海报扫码 %d 次，转化 %d 单，可围绕热卖品和领券动作拍一条短视频。", scanCount, conversionCount),
			"copy":  fmt.Sprintf("镜头1：展示%s和门店环境。口播：今天给新朋友准备了一张%s券。\n镜头2：展示扫码海报。口播：扫码进店就能领，满%s可用。\n镜头3：展示出餐或服务过程。口播：老顾客分享后也有复购奖励，想试试%s的朋友可以直接扫码。", productName, friend, threshold, merchantName),
		},
	}
}

func parseReferralSections(raw string) []map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	markers := []struct {
		Type  string
		Title string
		Keys  []string
	}{
		{Type: "朋友圈版", Title: "朋友圈裂变文案", Keys: []string{"【朋友圈文案】", "朋友圈文案", "朋友圈版"}},
		{Type: "社群版", Title: "社群转化话术", Keys: []string{"【社群话术】", "社群话术", "社群版"}},
		{Type: "短视频口播版", Title: "短视频口播脚本", Keys: []string{"【短视频口播脚本】", "短视频口播脚本", "短视频口播版"}},
	}
	var result []map[string]string
	for i, marker := range markers {
		start := -1
		usedKey := ""
		for _, key := range marker.Keys {
			if idx := strings.Index(raw, key); idx >= 0 && (start == -1 || idx < start) {
				start = idx
				usedKey = key
			}
		}
		if start < 0 {
			continue
		}
		contentStart := start + len(usedKey)
		end := len(raw)
		for _, next := range markers[i+1:] {
			for _, key := range next.Keys {
				if idx := strings.Index(raw[contentStart:], key); idx >= 0 && contentStart+idx < end {
					end = contentStart + idx
				}
			}
		}
		text := strings.TrimSpace(raw[contentStart:end])
		if text == "" {
			continue
		}
		result = append(result, map[string]string{
			"type":  marker.Type,
			"title": marker.Title,
			"text":  firstLine(text),
			"copy":  text,
		})
	}
	return result
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	first := strings.TrimSpace(lines[0])
	if len([]rune(first)) > 90 {
		runes := []rune(first)
		return string(runes[:90]) + "..."
	}
	return first
}

func trimRunes(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func mustQuota(quota map[string]interface{}, err error) map[string]interface{} {
	if err != nil {
		return map[string]interface{}{}
	}
	return quota
}

func parseAIShareReviewRange(startDate, endDate string) (ShareStatsParams, error) {
	var params ShareStatsParams
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)
	if startDate != "" {
		start, err := time.ParseInLocation("2006-01-02", startDate, time.Local)
		if err != nil {
			return params, errors.New("开始日期格式无效")
		}
		params.StartAt = &start
	}
	if endDate != "" {
		end, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
		if err != nil {
			return params, errors.New("结束日期格式无效")
		}
		end = end.AddDate(0, 0, 1).Add(-time.Nanosecond)
		params.EndAt = &end
	}
	return params, nil
}

func buildShareReviewDecision(config *model.ShareActivityConfig, stats *ShareStats, campaigns []model.ShareCampaign, coupons []model.ReferralCoupon) map[string]interface{} {
	if stats == nil {
		stats = &ShareStats{}
	}
	scanConversionRate := ratioFloat(stats.ConversionCount, stats.ScanCount)
	couponUseRate := ratioFloat(stats.UsedCouponCount, stats.RewardCouponCount)
	expiredRate := ratioFloat(stats.ExpiredCouponCount, stats.RewardCouponCount)
	avgConversionAmount := int64(0)
	if stats.ConversionCount > 0 {
		avgConversionAmount = stats.ConversionAmount / stats.ConversionCount
	}

	decision := "继续观察"
	level := "info"
	if stats.TotalCampaigns == 0 || stats.ScanCount == 0 {
		decision = "先开启活动并增加曝光"
		level = "warning"
	} else if scanConversionRate >= 0.15 && couponUseRate >= 0.2 {
		decision = "建议加大投放"
		level = "success"
	} else if stats.ScanCount >= 10 && stats.ConversionCount == 0 {
		decision = "建议优化文案和优惠门槛"
		level = "danger"
	} else if couponUseRate < 0.08 && stats.RewardCouponCount >= 10 {
		decision = "建议降低门槛或提高券吸引力"
		level = "warning"
	} else if expiredRate > 0.35 {
		decision = "建议缩短触达链路并增加到期提醒"
		level = "warning"
	}

	bestCampaign := map[string]interface{}{}
	stopCampaign := map[string]interface{}{}
	if len(campaigns) > 0 {
		best := campaigns[0]
		bestCampaign = map[string]interface{}{
			"title":             best.PosterTitle,
			"scan_count":        best.ScanCount,
			"conversion_count":  best.ConversionCount,
			"conversion_amount": best.ConversionAmount,
			"suggestion":        "优先复制这张海报的标题、话术和优惠配置，继续投放到朋友圈或社群。",
		}
		for i := len(campaigns) - 1; i >= 0; i-- {
			item := campaigns[i]
			if item.ScanCount > 0 && item.ConversionCount == 0 {
				stopCampaign = map[string]interface{}{
					"title":      item.PosterTitle,
					"scan_count": item.ScanCount,
					"suggestion": "这张海报有访问但无下单，建议暂停或重写首屏文案。",
				}
				break
			}
		}
	}

	var unused int64
	var used int64
	var expired int64
	for _, coupon := range coupons {
		switch coupon.Status {
		case "used":
			used++
		case "expired":
			expired++
		case "unused":
			unused++
		}
	}

	couponAdvice := "当前券表现正常，建议继续观察核销率和客单价。"
	if couponUseRate >= 0.25 {
		couponAdvice = "券核销率较好，可以适当扩大曝光或提高门槛，测试客单价提升。"
	} else if couponUseRate < 0.08 && stats.RewardCouponCount > 0 {
		couponAdvice = "券核销率偏低，建议降低使用门槛、增加到期提醒，或把券与热销商品绑定。"
	}
	if config != nil && config.FriendCouponAmount > 0 && config.FriendCouponThreshold > 0 {
		discountRatio := float64(config.FriendCouponAmount) / float64(config.FriendCouponThreshold)
		if discountRatio > 0.25 {
			couponAdvice += " 当前优惠力度较大，建议关注毛利，避免为了转化牺牲利润。"
		}
	}

	actions := []map[string]string{
		{"title": "复用最佳海报", "detail": "把扫码和下单最多的海报作为默认版本，继续投放到朋友圈、社群和店内桌贴。"},
		{"title": "优化低转化链路", "detail": "对有扫码无下单的海报，优先调整首屏标题、券门槛和商品卖点。"},
		{"title": "追踪券核销", "detail": "每天查看未使用、已核销和过期券，核销偏低时增加提醒或降低门槛。"},
	}

	return map[string]interface{}{
		"decision":               decision,
		"level":                  level,
		"scan_conversion_rate":   scanConversionRate,
		"coupon_use_rate":        couponUseRate,
		"expired_rate":           expiredRate,
		"avg_conversion_amount":  avgConversionAmount,
		"best_campaign":          bestCampaign,
		"stop_campaign":          stopCampaign,
		"coupon_advice":          couponAdvice,
		"coupon_status_snapshot": map[string]int64{"unused": unused, "used": used, "expired": expired},
		"actions":                actions,
	}
}

func ratioFloat(value, base int64) float64 {
	if base <= 0 {
		return 0
	}
	return float64(value) / float64(base)
}

func (s *MerchantService) marketingStatsSummary(merchantID uint) map[string]interface{} {
	var orderCount int64
	var tradeAmount int64
	var refundAmount int64
	var customerCount int64
	s.db.Model(&model.Order{}).
		Where("merchant_id = ? AND order_type = ?", merchantID, "store_order").
		Where("status IN ?", []string{"received", "accepted", "completed", "closed"}).
		Count(&orderCount)
	s.db.Model(&model.Order{}).
		Where("merchant_id = ? AND order_type = ?", merchantID, "store_order").
		Where("status IN ?", []string{"received", "accepted", "completed", "closed"}).
		Select("COALESCE(SUM(total_amount),0)").Scan(&tradeAmount)
	s.db.Model(&model.RefundRecord{}).
		Where("merchant_id = ? AND status = ?", merchantID, "success").
		Select("COALESCE(SUM(amount),0)").Scan(&refundAmount)
	s.db.Model(&model.Order{}).
		Where("merchant_id = ? AND order_type = ? AND customer_phone <> ''", merchantID, "store_order").
		Distinct("customer_phone").Count(&customerCount)
	return map[string]interface{}{
		"order_count":    orderCount,
		"trade_amount":   tradeAmount,
		"refund_amount":  refundAmount,
		"customer_count": customerCount,
	}
}

func (s *MerchantService) issueMerchantVerificationCode(phone, scene string, userID *uint) error {
	code := devFixedSMSCode
	if strings.EqualFold(s.cfg.AppEnv, "production") {
		generated, err := generateNumericCode(6)
		if err != nil {
			return err
		}
		code = generated
	}

	if err := s.codes.InvalidateActive(phone, scene); err != nil {
		return err
	}

	record := &model.VerificationCode{
		Phone:     phone,
		Scene:     scene,
		Code:      code,
		UserID:    userID,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := s.codes.Create(record); err != nil {
		return err
	}

	log.Printf("[mock-sms] scene=%s phone=%s code=%s expires_at=%s", scene, phone, code, record.ExpiresAt.Format(time.RFC3339))
	return nil
}

func (s *MerchantService) consumeMerchantVerificationCode(phone, scene, code string) error {
	if !strings.EqualFold(s.cfg.AppEnv, "production") && strings.TrimSpace(code) == devFixedSMSCode {
		log.Printf("[mock-sms] accepted fixed dev code scene=%s phone=%s code=%s", scene, phone, strings.TrimSpace(code))
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		codes := repository.NewVerificationCodeRepository(tx)
		record, err := codes.FindValid(phone, scene, strings.TrimSpace(code))
		if err != nil {
			return errors.New("\u9a8c\u8bc1\u7801\u9519\u8bef\u6216\u5df2\u8fc7\u671f")
		}

		now := time.Now()
		record.ConsumedAt = &now
		return codes.Save(record)
	})
}

func ptrUint(v uint) *uint {
	return &v
}

func parsePromotionTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return nil, errors.New("\u6d3b\u52a8\u65f6\u95f4\u683c\u5f0f\u65e0\u6548")
	}
	return &parsed, nil
}
