package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
)

type StoreOrderItemSnapshot struct {
	ProductID   uint   `json:"product_id"`
	Name        string `json:"name"`
	Price       int64  `json:"price"`
	Quantity    int    `json:"quantity"`
	LineAmount  int64  `json:"line_amount"`
	Description string `json:"description,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

type OrderOperationLog struct {
	Action string `json:"action"`
	Text   string `json:"text"`
	Time   string `json:"time"`
}

type MerchantSettlementSummary struct {
	PendingOrderCount      int64 `json:"pending_order_count"`
	PendingOrderAmount     int64 `json:"pending_order_amount"`
	PendingRefundAmount    int64 `json:"pending_refund_amount"`
	PendingNetAmount       int64 `json:"pending_net_amount"`
	SettlementCount        int64 `json:"settlement_count"`
	PendingSettlementCount int64 `json:"pending_settlement_count"`
	PaidSettlementCount    int64 `json:"paid_settlement_count"`
	PaidSettlementAmount   int64 `json:"paid_settlement_amount"`
}

type OrderService struct {
	db            *gorm.DB
	users         *repository.UserRepository
	packages      *repository.PackageRepository
	orders        *repository.OrderRepository
	merchants     *repository.MerchantRepository
	merchantPlans *repository.MerchantPlanRepository
	stores        *repository.StoreRepository
	products      *repository.StoreProductRepository
	promotions    *repository.PromotionRepository
	cards         *repository.CardRepository
	systems       *repository.SystemRepository
	alipay        *AlipayService
	shares        *ShareService
}

func NewOrderService(db *gorm.DB, alipayServices ...*AlipayService) *OrderService {
	var alipayService *AlipayService
	if len(alipayServices) > 0 {
		alipayService = alipayServices[0]
	}
	return &OrderService{
		db:            db,
		users:         repository.NewUserRepository(db),
		packages:      repository.NewPackageRepository(db),
		orders:        repository.NewOrderRepository(db),
		merchants:     repository.NewMerchantRepository(db),
		merchantPlans: repository.NewMerchantPlanRepository(db),
		stores:        repository.NewStoreRepository(db),
		products:      repository.NewStoreProductRepository(db),
		promotions:    repository.NewPromotionRepository(db),
		cards:         repository.NewCardRepository(db),
		systems:       repository.NewSystemRepository(db),
		alipay:        alipayService,
		shares:        NewShareService(db),
	}
}

func (s *OrderService) CreateOrder(userID uint, req dto.CreateOrderRequest) (*model.Order, error) {
	user, err := s.users.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, errors.New("account disabled")
	}

	pkg, err := s.packages.FindByID(req.PackageID)
	if err != nil {
		return nil, err
	}
	if pkg.Status != "published" {
		return nil, errors.New("package unavailable")
	}

	order := &model.Order{
		OrderNo:        fmt.Sprintf("ORD%d", time.Now().UnixNano()),
		OrderType:      "user_membership",
		UserID:         &userID,
		PackageID:      &pkg.ID,
		Amount:         pkg.Price,
		TotalAmount:    pkg.Price,
		Status:         "pending",
		PaymentChannel: req.PaymentChannel,
		AutoRenew:      req.AutoRenew,
	}

	if err := s.orders.Create(order); err != nil {
		return nil, err
	}

	order.User = user
	order.Package = pkg
	return order, nil
}

func (s *OrderService) CreateCustomerStoreOrder(req dto.CreateCustomerOrderRequest) (*model.Order, error) {
	store, err := s.stores.FindByIDWithMerchant(req.StoreID)
	if err != nil {
		return nil, errors.New("store not found")
	}
	if store.Status != "active" || !store.IsOpen || store.Merchant.Status != "active" {
		return nil, errors.New("store is unavailable")
	}
	if req.CustomerPhone != "" && !isValidMainlandPhone(normalizePhone(req.CustomerPhone)) {
		return nil, errors.New("customer phone must be 11 digits")
	}
	if len(req.Items) == 0 {
		return nil, errors.New("please select at least one product")
	}

	productIDs := make([]uint, 0, len(req.Items))
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, errors.New("product quantity must be greater than 0")
		}
		productIDs = append(productIDs, item.ProductID)
	}

	products, err := s.products.FindActiveByIDs(req.StoreID, productIDs)
	if err != nil {
		return nil, err
	}
	if len(products) != len(productIDs) {
		return nil, errors.New("some products are unavailable")
	}

	productMap := make(map[uint]model.StoreProduct, len(products))
	for _, product := range products {
		productMap[product.ID] = product
	}

	snapshots := make([]StoreOrderItemSnapshot, 0, len(req.Items))
	var total int64
	for _, item := range req.Items {
		product, ok := productMap[item.ProductID]
		if !ok {
			return nil, errors.New("some products are unavailable")
		}
		if product.Stock != nil && item.Quantity > *product.Stock {
			if *product.Stock <= 0 {
				return nil, fmt.Errorf("%s 已售罄", product.Name)
			}
			return nil, fmt.Errorf("%s 库存仅剩 %d 件", product.Name, *product.Stock)
		}
		lineAmount := product.Price * int64(item.Quantity)
		total += lineAmount
		snapshots = append(snapshots, StoreOrderItemSnapshot{
			ProductID:   product.ID,
			Name:        product.Name,
			Price:       product.Price,
			Quantity:    item.Quantity,
			LineAmount:  lineAmount,
			Description: product.Description,
			ImageURL:    product.ImageURL,
		})
	}

	itemsJSON, err := json.Marshal(snapshots)
	if err != nil {
		return nil, err
	}

	promotion, promotionDiscount := s.bestPromotionForStore(store.MerchantID, store.ID, total)
	discount := promotionDiscount
	coupon, couponDiscount, err := s.shares.ValidateCouponForOrder(store.ID, req.CustomerPhone, req.CouponNo, total-discount)
	if err != nil {
		return nil, err
	}
	if couponDiscount > 0 {
		discount += couponDiscount
	}
	payable := total - discount
	if payable < 0 {
		payable = 0
	}

	channel := req.PaymentChannel
	if channel == "" {
		channel = "alipay"
	}
	status := "pending"
	if store.OrderMode == "submit_later" {
		status = "submitted"
		if store.AutoAccept {
			status = "preparing"
		}
	}

	order := &model.Order{
		OrderNo:        fmt.Sprintf("SORD%d", time.Now().UnixNano()),
		OrderType:      "store_order",
		MerchantID:     &store.MerchantID,
		StoreID:        &store.ID,
		CustomerPhone:  normalizePhone(req.CustomerPhone),
		CustomerNote:   strings.TrimSpace(req.CustomerNote),
		Amount:         total,
		TotalAmount:    payable,
		DiscountAmount: discount,
		Items:          string(itemsJSON),
		Status:         status,
		PaymentChannel: channel,
		ShareCode:      strings.TrimSpace(req.ShareCode),
	}
	if store.OrderMode == "submit_later" {
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "submitted", "顾客已提交订单，稍后结算")
		if store.AutoAccept {
			order.OperationLogs = s.appendOrderLog(order.OperationLogs, "auto_accepted", "门店已开启自动接单，订单进入处理中")
		}
	}
	if coupon != nil {
		order.CouponID = &coupon.ID
		order.CouponNo = coupon.CouponNo
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "coupon_locked", fmt.Sprintf("已锁定奖励券：%s，优惠 %s 元", coupon.Title, formatFen(couponDiscount)))
	}
	if promotion != nil {
		order.PromotionID = &promotion.ID
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "promotion_applied", fmt.Sprintf("已自动匹配优惠：%s，优惠 %s 元", promotion.Title, formatFen(promotionDiscount)))
	}

	if err := s.orders.Create(order); err != nil {
		return nil, err
	}

	order.Store = store
	order.Merchant = &store.Merchant
	return order, nil
}

func (s *OrderService) PublicStoreProducts(storeID uint) ([]model.StoreProduct, error) {
	store, err := s.stores.FindByIDWithMerchant(storeID)
	if err != nil {
		return nil, errors.New("store not found")
	}
	if store.Status != "active" || store.Merchant.Status != "active" {
		return nil, errors.New("store is unavailable")
	}
	return s.products.ListActiveByStore(storeID)
}

func (s *OrderService) PublicStoreCoupons(storeID uint, phone string) ([]model.ReferralCoupon, error) {
	store, err := s.stores.FindByIDWithMerchant(storeID)
	if err != nil {
		return nil, errors.New("store not found")
	}
	if store.Status != "active" || !store.IsOpen || store.Merchant.Status != "active" {
		return nil, errors.New("store is unavailable")
	}
	if !isValidMainlandPhone(normalizePhone(phone)) {
		return nil, errors.New("手机号需要是 11 位数字")
	}
	return s.shares.ListUsableCoupons(storeID, phone)
}

func (s *OrderService) PublicOrderDetail(orderNo string) (*model.Order, error) {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return nil, errors.New("order not found")
	}
	return order, nil
}

func (s *OrderService) PublicPaymentConfigForOrder(order *model.Order) (*model.MerchantPaymentConfig, error) {
	if order == nil || order.OrderType != "store_order" || order.MerchantID == nil {
		return nil, nil
	}
	var config model.MerchantPaymentConfig
	err := s.db.Where(
		"merchant_id = ? AND mode = ? AND status = ? AND audit_status = ?",
		*order.MerchantID,
		"direct",
		"enabled",
		"approved",
	).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *OrderService) MarkCustomerStoreOrderPaid(orderNo string) (*model.Order, error) {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return nil, errors.New("order not found")
	}
	if order.PaidAt != nil || order.Status == "received" || order.Status == "accepted" || order.Status == "completed" {
		return order, nil
	}
	if order.Status != "submitted" && order.Status != "preparing" {
		return nil, errors.New("only submitted orders can be marked as paid")
	}
	order.Status = "payment_confirming"
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "customer_paid", "顾客已标记付款，等待商家确认收款")
	if err := s.orders.Save(order); err != nil {
		return nil, err
	}
	return s.orders.FindByOrderNo(orderNo)
}

func (s *OrderService) AppendCustomerStoreOrderItems(orderNo string, req dto.AppendCustomerOrderItemsRequest) (*model.Order, error) {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if order.OrderType != "store_order" || order.StoreID == nil {
		return nil, errors.New("order not found")
	}
	if order.PaidAt != nil || order.Status == "payment_confirming" || order.Status == "received" || order.Status == "accepted" || order.Status == "completed" || order.Status == "closed" {
		return nil, errors.New("current order cannot append items")
	}
	if order.Status != "submitted" && order.Status != "preparing" {
		return nil, errors.New("only submitted orders can append items")
	}
	if len(req.Items) == 0 {
		return nil, errors.New("please select at least one product")
	}

	store, err := s.stores.FindByIDWithMerchant(*order.StoreID)
	if err != nil {
		return nil, errors.New("store not found")
	}
	if store.OrderMode != "submit_later" {
		return nil, errors.New("only submit-later stores can append items")
	}
	if store.Status != "active" || !store.IsOpen || store.Merchant.Status != "active" {
		return nil, errors.New("store is unavailable")
	}

	productIDs := make([]uint, 0, len(req.Items))
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, errors.New("product quantity must be greater than 0")
		}
		productIDs = append(productIDs, item.ProductID)
	}
	products, err := s.products.FindActiveByIDs(store.ID, productIDs)
	if err != nil {
		return nil, err
	}
	if len(products) != len(productIDs) {
		return nil, errors.New("some products are unavailable")
	}
	productMap := make(map[uint]model.StoreProduct, len(products))
	for _, product := range products {
		productMap[product.ID] = product
	}

	var existing []StoreOrderItemSnapshot
	if order.Items != "" {
		if err := json.Unmarshal([]byte(order.Items), &existing); err != nil {
			return nil, errors.New("order items are invalid")
		}
	}
	existingMap := make(map[uint]int, len(existing))
	for index, item := range existing {
		existingMap[item.ProductID] = index
	}

	var appendedAmount int64
	var appendedQuantity int
	appendedNames := make([]string, 0, len(req.Items))
	for _, item := range req.Items {
		product, ok := productMap[item.ProductID]
		if !ok {
			return nil, errors.New("some products are unavailable")
		}
		if product.Stock != nil && item.Quantity > *product.Stock {
			if *product.Stock <= 0 {
				return nil, fmt.Errorf("%s 已售罄", product.Name)
			}
			return nil, fmt.Errorf("%s 库存仅剩 %d 件", product.Name, *product.Stock)
		}
		lineAmount := product.Price * int64(item.Quantity)
		appendedAmount += lineAmount
		appendedQuantity += item.Quantity
		appendedNames = append(appendedNames, fmt.Sprintf("%s x%d", product.Name, item.Quantity))
		if index, ok := existingMap[product.ID]; ok {
			existing[index].Quantity += item.Quantity
			existing[index].LineAmount += lineAmount
			continue
		}
		existingMap[product.ID] = len(existing)
		existing = append(existing, StoreOrderItemSnapshot{
			ProductID:   product.ID,
			Name:        product.Name,
			Price:       product.Price,
			Quantity:    item.Quantity,
			LineAmount:  lineAmount,
			Description: product.Description,
			ImageURL:    product.ImageURL,
		})
	}

	itemsJSON, err := json.Marshal(existing)
	if err != nil {
		return nil, err
	}
	order.Items = string(itemsJSON)
	order.Amount += appendedAmount
	order.TotalAmount += appendedAmount
	if note := strings.TrimSpace(req.CustomerNote); note != "" {
		if order.CustomerNote != "" {
			order.CustomerNote = strings.TrimSpace(order.CustomerNote + "；追加备注：" + note)
		} else {
			order.CustomerNote = note
		}
		if len([]rune(order.CustomerNote)) > 500 {
			order.CustomerNote = string([]rune(order.CustomerNote)[:500])
		}
	}
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "items_appended", fmt.Sprintf("顾客追加商品：%s，新增 %d 件，新增金额 %s 元", strings.Join(appendedNames, "、"), appendedQuantity, formatFen(appendedAmount)))
	if err := s.orders.Save(order); err != nil {
		return nil, err
	}
	return s.orders.FindByOrderNo(orderNo)
}

func (s *OrderService) PrepareCustomerOrderRetry(orderNo string) (*model.Order, error) {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return nil, errors.New("order not found")
	}
	if order.Status != "pending" && order.Status != "failed" {
		return nil, errors.New("order status does not allow payment retry")
	}
	if order.Status == "failed" {
		order.Status = "pending"
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "payment_retry", "顾客重新发起支付")
		if err := s.orders.Save(order); err != nil {
			return nil, err
		}
	}
	return order, nil
}

func (s *OrderService) ReviewCustomerOrder(orderNo string, req dto.ReviewCustomerOrderRequest) (*model.Order, error) {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return nil, errors.New("order not found")
	}
	if order.Status != "accepted" && order.Status != "completed" {
		return nil, errors.New("only accepted or completed orders can be reviewed")
	}
	if req.Rating < 1 || req.Rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}
	review := strings.TrimSpace(req.Review)
	if len([]rune(review)) > 1000 {
		return nil, errors.New("review cannot exceed 1000 characters")
	}
	now := time.Now()
	order.CustomerRating = req.Rating
	order.CustomerReview = review
	order.ReviewedAt = &now
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "customer_reviewed", fmt.Sprintf("顾客评价：%d 星", req.Rating))
	if err := s.orders.Save(order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) applyPackageToUser(user *model.User, pkg *model.MembershipPackage, autoRenew bool) error {
	now := time.Now()
	base := now
	if user.ExpiredAt != nil && user.ExpiredAt.After(now) {
		base = *user.ExpiredAt
	}

	if pkg.IsLifetime {
		expire := time.Date(2099, 12, 31, 23, 59, 59, 0, time.Local)
		user.ExpiredAt = &expire
	} else {
		expire := base.AddDate(0, 0, pkg.DurationDays)
		user.ExpiredAt = &expire
	}

	user.MemberLevel = pkg.Code
	user.CurrentPackageID = &pkg.ID
	user.AutoRenew = autoRenew
	user.RemainingQuota += pkg.Quota
	return s.users.Save(user)
}

func (s *OrderService) RedeemCard(userID uint, code string) error {
	card, err := s.cards.FindByCode(code)
	if err != nil {
		return errors.New("card code not found")
	}
	if card.Status != "unused" {
		return errors.New("card code already redeemed")
	}
	if card.ExpiredAt != nil && card.ExpiredAt.Before(time.Now()) {
		return errors.New("card code expired")
	}

	user, err := s.users.FindByID(userID)
	if err != nil {
		return err
	}

	if err := s.applyPackageToUser(user, &card.Package, false); err != nil {
		return err
	}

	card.Status = "used"
	card.RedeemedBy = &userID
	now := time.Now()
	card.RedeemedAt = &now
	return s.cards.Save(card)
}

func (s *OrderService) BindDevice(userID uint, deviceID string) error {
	user, err := s.users.FindByID(userID)
	if err != nil {
		return err
	}

	if user.BoundDevices == "" {
		user.BoundDevices = deviceID
	} else if user.BoundDevices != deviceID {
		user.BoundDevices += "," + deviceID
	}

	if err := s.users.Save(user); err != nil {
		return err
	}

	return s.systems.CreateUsage(&model.UsageRecord{
		UserID:      userID,
		Scene:       "device_bind",
		Description: "User bound a device",
		QuotaUsed:   0,
		DeviceID:    deviceID,
	})
}

func (s *OrderService) UserOrders(userID uint) ([]model.Order, error) {
	return s.orders.ListByUser(userID)
}

func (s *OrderService) UserOrderDetail(userID uint, orderNo string) (*model.Order, error) {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, err
	}
	if order.UserID == nil || *order.UserID != userID {
		return nil, errors.New("order not found")
	}
	return order, nil
}

func (s *OrderService) CancelOrder(userID uint, orderNo string) error {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return err
	}
	if order.UserID == nil || *order.UserID != userID {
		return errors.New("order not found")
	}
	if order.Status != "pending" {
		return errors.New("only pending orders can be cancelled")
	}

	order.Status = "closed"
	return s.orders.Save(order)
}

func (s *OrderService) HandlePaymentCallback(req dto.PaymentCallbackRequest) (*model.Order, error) {
	var result model.Order

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Preload("Package").Preload("User").Preload("Merchant").Preload("MerchantPlan").Preload("Store").Where("order_no = ?", req.OrderNo).First(&order).Error; err != nil {
			return err
		}

		expectedAmount := order.TotalAmount
		if expectedAmount <= 0 {
			expectedAmount = order.Amount
		}
		if req.Status != "failed" && expectedAmount != req.Amount {
			return errors.New("payment amount mismatch")
		}

		var existingPayment model.PaymentRecord
		paymentErr := tx.Where("transaction_no = ?", req.TransactionNo).First(&existingPayment).Error
		if paymentErr == nil {
			if existingPayment.OrderID != order.ID {
				return errors.New("duplicate transaction number")
			}
			result = order
			return nil
		}
		if paymentErr != nil && !errors.Is(paymentErr, gorm.ErrRecordNotFound) {
			return paymentErr
		}

		rawPayload := ""
		if req.RawPayload != nil {
			bytes, err := json.Marshal(req.RawPayload)
			if err != nil {
				return err
			}
			rawPayload = string(bytes)
		}

		paymentStatus := "success"
		if req.Status == "failed" {
			paymentStatus = "failed"
		}

		record := &model.PaymentRecord{
			OrderID:        order.ID,
			PaymentChannel: req.PaymentChannel,
			TransactionNo:  req.TransactionNo,
			Amount:         req.Amount,
			Status:         paymentStatus,
			RawPayload:     rawPayload,
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}

		if req.Status == "failed" {
			if order.Status == "pending" {
				order.Status = "failed"
				order.OperationLogs = s.appendOrderLog(order.OperationLogs, "payment_failed", "支付关闭或失败，订单可重新发起支付")
				if err := tx.Save(&order).Error; err != nil {
					return err
				}
			}
			result = order
			return nil
		}

		if order.Status == "paid" || order.Status == "received" || order.Status == "accepted" || order.Status == "completed" || order.Status == "closed" {
			result = order
			return nil
		}
		if order.Status != "pending" {
			return errors.New("order status does not allow payment confirmation")
		}

		paidAt := time.Now()
		order.PaidAt = &paidAt
		order.TransactionNo = req.TransactionNo

		if order.OrderType == "store_order" {
			order.Status = "received"
			order.OperationLogs = s.appendOrderLog(order.OperationLogs, "paid", "支付成功，订单已发送给商家处理")
			stockLog, err := s.deductStoreOrderStockTx(tx, &order)
			if err != nil {
				return err
			}
			if stockLog != "" {
				order.OperationLogs = s.appendOrderLog(order.OperationLogs, "stock_deducted", stockLog)
			}
			if err := tx.Save(&order).Error; err != nil {
				return err
			}
			if err := NewShareService(tx).ConsumeCouponForOrder(&order); err != nil {
				return err
			}
			NewShareService(tx).RecordConversion(order.ShareCode, &order)
			result = order
			return nil
		}

		order.Status = "paid"
		if order.OrderType == "merchant_subscription" {
			if order.MerchantID == nil || order.MerchantPlanID == nil || order.MerchantPlan == nil {
				return errors.New("merchant subscription order is invalid")
			}
			var merchant model.Merchant
			if err := tx.First(&merchant, *order.MerchantID).Error; err != nil {
				return err
			}
			base := paidAt
			if merchant.SubscriptionExpireAt != nil && merchant.SubscriptionExpireAt.After(base) {
				base = *merchant.SubscriptionExpireAt
			}
			expire := base.AddDate(0, 0, order.MerchantPlan.DurationDays)
			order.SubscriptionEndAt = &expire
			if err := tx.Save(&order).Error; err != nil {
				return err
			}
			merchant.SubscriptionPlanID = order.MerchantPlanID
			merchant.SubscriptionExpireAt = &expire
			merchant.SubscriptionPlan = order.MerchantPlan.Name
			merchant.SubscriptionStatus = "active"
			merchant.SubscriptionExpiredAt = &expire
			if err := tx.Save(&merchant).Error; err != nil {
				return err
			}
			result = order
			return nil
		}

		if order.Package == nil || order.User == nil {
			return errors.New("membership order is invalid")
		}

		if order.Package.IsLifetime {
			expire := time.Date(2099, 12, 31, 23, 59, 59, 0, time.Local)
			order.SubscriptionEndAt = &expire
		} else {
			base := paidAt
			if order.User.ExpiredAt != nil && order.User.ExpiredAt.After(base) {
				base = *order.User.ExpiredAt
			}
			expire := base.AddDate(0, 0, order.Package.DurationDays)
			order.SubscriptionEndAt = &expire
		}

		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		if err := s.applyPackageToUserTx(tx, order.User, order.Package, order.AutoRenew); err != nil {
			return err
		}

		result = order
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *OrderService) ConfirmMerchantSubscriptionPayment(orderID uint, remark string) (*model.Order, error) {
	var result model.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Preload("MerchantPlan").Preload("Merchant").Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		if order.OrderType != "merchant_subscription" {
			return errors.New("only merchant subscription orders can be confirmed")
		}
		if order.Status != "pending" {
			return errors.New("only pending subscription orders can be confirmed")
		}
		if order.MerchantID == nil || order.MerchantPlanID == nil || order.MerchantPlan == nil {
			return errors.New("merchant subscription order is invalid")
		}

		var merchant model.Merchant
		if err := tx.First(&merchant, *order.MerchantID).Error; err != nil {
			return err
		}

		amount := order.TotalAmount
		if amount <= 0 {
			amount = order.Amount
		}
		paidAt := time.Now()
		base := paidAt
		if merchant.SubscriptionExpireAt != nil && merchant.SubscriptionExpireAt.After(base) {
			base = *merchant.SubscriptionExpireAt
		}
		expire := base.AddDate(0, 0, order.MerchantPlan.DurationDays)
		transactionNo := fmt.Sprintf("MANUAL-MSUB-%d-%d", order.ID, paidAt.UnixNano())
		rawPayload, err := json.Marshal(map[string]interface{}{
			"source": "admin_manual_confirm",
			"remark": strings.TrimSpace(remark),
		})
		if err != nil {
			return err
		}

		payment := &model.PaymentRecord{
			OrderID:        order.ID,
			PaymentChannel: "platform_qr",
			TransactionNo:  transactionNo,
			Amount:         amount,
			Status:         "success",
			RawPayload:     string(rawPayload),
		}
		if err := tx.Create(payment).Error; err != nil {
			return err
		}

		order.Status = "paid"
		if strings.TrimSpace(order.Items) == "" {
			order.Items = "[]"
		}
		order.PaidAt = &paidAt
		order.TransactionNo = transactionNo
		order.PaymentChannel = "platform_qr"
		order.SubscriptionEndAt = &expire
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "subscription_payment_confirmed", "平台已确认订阅款到账，商家订阅已开通或续期")
		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		merchant.SubscriptionPlanID = order.MerchantPlanID
		merchant.SubscriptionPlan = order.MerchantPlan.Name
		merchant.SubscriptionStatus = "active"
		merchant.SubscriptionExpireAt = &expire
		merchant.SubscriptionExpiredAt = &expire
		merchant.SubscriptionNote = "平台确认订阅款到账后开通"
		if err := tx.Save(&merchant).Error; err != nil {
			return err
		}

		result = order
		result.Merchant = &merchant
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.db.Preload("MerchantPlan").Preload("Merchant").Where("id = ?", result.ID).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *OrderService) deductStoreOrderStockTx(tx *gorm.DB, order *model.Order) (string, error) {
	var items []StoreOrderItemSnapshot
	if err := json.Unmarshal([]byte(order.Items), &items); err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", nil
	}

	deducted := make([]string, 0, len(items))
	shortage := make([]string, 0)
	for _, item := range items {
		if item.ProductID == 0 || item.Quantity <= 0 {
			continue
		}

		var product model.StoreProduct
		if err := tx.Select("id", "name", "stock").Where("id = ?", item.ProductID).First(&product).Error; err != nil {
			return "", err
		}
		if product.Stock == nil {
			continue
		}

		currentStock := *product.Stock
		if currentStock <= 0 {
			shortage = append(shortage, fmt.Sprintf("%s 已售罄但订单已付款，请人工处理", item.Name))
			continue
		}
		if currentStock < item.Quantity {
			if err := tx.Model(&model.StoreProduct{}).Where("id = ?", item.ProductID).Update("stock", 0).Error; err != nil {
				return "", err
			}
			shortage = append(shortage, fmt.Sprintf("%s 库存不足，付款数量 %d，扣减前仅剩 %d", item.Name, item.Quantity, currentStock))
			continue
		}
		update := tx.Model(&model.StoreProduct{}).
			Where("id = ? AND stock IS NOT NULL AND stock >= ?", item.ProductID, item.Quantity).
			Update("stock", gorm.Expr("stock - ?", item.Quantity))
		if update.Error != nil {
			return "", update.Error
		}
		if update.RowsAffected == 0 {
			var latest model.StoreProduct
			if err := tx.Select("id", "name", "stock").Where("id = ?", item.ProductID).First(&latest).Error; err != nil {
				return "", err
			}
			if latest.Stock == nil {
				continue
			}
			shortage = append(shortage, fmt.Sprintf("%s 库存并发变动，付款数量 %d，当前仅剩 %d", item.Name, item.Quantity, *latest.Stock))
			if *latest.Stock > 0 {
				if err := tx.Model(&model.StoreProduct{}).Where("id = ?", item.ProductID).Update("stock", 0).Error; err != nil {
					return "", err
				}
			}
			continue
		}
		deducted = append(deducted, fmt.Sprintf("%s -%d", item.Name, item.Quantity))
	}

	parts := make([]string, 0, 2)
	if len(deducted) > 0 {
		parts = append(parts, "库存已扣减："+strings.Join(deducted, "、"))
	}
	if len(shortage) > 0 {
		parts = append(parts, "库存异常："+strings.Join(shortage, "；"))
	}
	return strings.Join(parts, "；"), nil
}

func (s *OrderService) MerchantOrders(merchantID uint, status string, storeID *uint, keyword string, page, pageSize int) ([]model.Order, int64, error) {
	return s.orders.ListByMerchant(merchantID, status, storeID, keyword, (page-1)*pageSize, pageSize)
}

func (s *OrderService) MerchantOrderDetail(merchantID, orderID uint) (*model.Order, error) {
	order, err := s.orders.FindByMerchantAndID(merchantID, orderID)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return nil, errors.New("order not found")
	}
	return order, nil
}

func (s *OrderService) AcceptMerchantOrder(merchantID, orderID uint) error {
	order, err := s.orders.FindByMerchantAndID(merchantID, orderID)
	if err != nil {
		return errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return errors.New("order not found")
	}
	if order.Status != "received" && order.Status != "submitted" {
		return errors.New("only submitted or received orders can be accepted")
	}
	order.Status = "accepted"
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "accepted", "商家已接单")
	return s.orders.Save(order)
}

func (s *OrderService) ConfirmMerchantOrderPayment(merchantID, orderID uint) (*model.Order, error) {
	var result model.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Preload("Store").Where("merchant_id = ? AND id = ?", merchantID, orderID).First(&order).Error; err != nil {
			return errors.New("order not found")
		}
		if order.OrderType != "store_order" {
			return errors.New("order not found")
		}
		if order.PaidAt != nil {
			result = order
			return nil
		}
		if order.Status != "payment_confirming" && order.Status != "submitted" && order.Status != "preparing" {
			return errors.New("only unpaid submitted orders can be confirmed")
		}
		paidAt := time.Now()
		transactionNo := fmt.Sprintf("manual-%s-%d", order.OrderNo, paidAt.Unix())
		record := &model.PaymentRecord{
			OrderID:        order.ID,
			PaymentChannel: "merchant_qr",
			TransactionNo:  transactionNo,
			Amount:         order.TotalAmount,
			Status:         "success",
			RawPayload:     `{"source":"merchant_confirmed_qr"}`,
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		order.PaidAt = &paidAt
		order.TransactionNo = transactionNo
		order.PaymentChannel = "merchant_qr"
		if orderHasAction(order.OperationLogs, "auto_accepted") {
			order.Status = "accepted"
		} else {
			order.Status = "received"
		}
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "merchant_payment_confirmed", "商家已确认收到顾客付款")
		stockLog, err := s.deductStoreOrderStockTx(tx, &order)
		if err != nil {
			return err
		}
		if stockLog != "" {
			order.OperationLogs = s.appendOrderLog(order.OperationLogs, "stock_deducted", stockLog)
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		if err := NewShareService(tx).ConsumeCouponForOrder(&order); err != nil {
			return err
		}
		NewShareService(tx).RecordConversion(order.ShareCode, &order)
		result = order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *OrderService) CompleteMerchantOrder(merchantID, orderID uint) error {
	order, err := s.orders.FindByMerchantAndID(merchantID, orderID)
	if err != nil {
		return errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return errors.New("order not found")
	}
	if order.Status != "accepted" && order.Status != "preparing" {
		return errors.New("only accepted or preparing orders can be completed")
	}
	order.Status = "completed"
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "completed", "商家已完成订单")
	return s.orders.Save(order)
}

func (s *OrderService) CloseMerchantOrder(merchantID, orderID uint) error {
	order, err := s.orders.FindByMerchantAndID(merchantID, orderID)
	if err != nil {
		return errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return errors.New("order not found")
	}
	if order.Status != "submitted" && order.Status != "payment_confirming" && order.Status != "preparing" && order.Status != "received" && order.Status != "accepted" {
		return errors.New("only submitted, received or accepted orders can be cancelled")
	}
	order.Status = "closed"
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "closed", "商家已取消订单")
	return s.orders.Save(order)
}

func (s *OrderService) UpdateMerchantOrderNote(merchantID, orderID uint, note string) (*model.Order, error) {
	order, err := s.orders.FindByMerchantAndID(merchantID, orderID)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return nil, errors.New("order not found")
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) > 500 {
		return nil, errors.New("merchant note cannot exceed 500 characters")
	}
	order.MerchantNote = note
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "note_updated", "商家更新了内部备注")
	if err := s.orders.Save(order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) RefundMerchantOrder(merchantID, orderID uint, req dto.RefundOrderRequest, operatorID uint, operatorRole string) (*model.Order, error) {
	order, err := s.orders.FindByMerchantAndID(merchantID, orderID)
	if err != nil {
		return nil, errors.New("order not found")
	}
	return s.refundOrder(order, req, operatorID, operatorRole)
}

func (s *OrderService) RefundAdminOrder(orderID uint, req dto.RefundOrderRequest, operatorID uint) (*model.Order, error) {
	var order model.Order
	if err := s.db.Preload("Store").Preload("Merchant").First(&order, orderID).Error; err != nil {
		return nil, errors.New("order not found")
	}
	return s.refundOrder(&order, req, operatorID, "admin")
}

func (s *OrderService) refundOrder(order *model.Order, req dto.RefundOrderRequest, operatorID uint, operatorRole string) (*model.Order, error) {
	if order.OrderType != "store_order" {
		return nil, errors.New("only store orders can be refunded here")
	}
	if order.Status != "received" && order.Status != "accepted" && order.Status != "completed" && order.Status != "closed" {
		return nil, errors.New("order status does not allow refund")
	}
	refundable := order.TotalAmount - order.RefundedAmount
	if refundable <= 0 {
		return nil, errors.New("order has no refundable amount")
	}
	amount := req.Amount
	if req.Full || amount <= 0 {
		amount = refundable
	}
	if amount <= 0 || amount > refundable {
		return nil, errors.New("refund amount is invalid")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "售后退款"
	}
	if len([]rune(reason)) > 255 {
		return nil, errors.New("refund reason cannot exceed 255 characters")
	}

	refundNo := fmt.Sprintf("RF%d", time.Now().UnixNano())
	refundStatus := "success"
	rawPayload := `{"mode":"manual_record","note":"真实退款需由商家收款账户或支付渠道执行，本记录用于财务核对"}`
	if order.PaymentChannel == "alipay" && order.TransactionNo != "" && s.alipay != nil && s.alipay.Enabled() {
		payload, err := s.alipay.Refund(order, refundNo, amount, reason)
		if err != nil {
			return nil, err
		}
		rawPayload = payload
	}

	var updated model.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var current model.Order
		if err := tx.Preload("Store").Preload("Merchant").First(&current, order.ID).Error; err != nil {
			return err
		}
		refundable := current.TotalAmount - current.RefundedAmount
		if amount > refundable {
			return errors.New("refund amount exceeds remaining refundable amount")
		}
		refund := &model.RefundRecord{
			OrderID:       current.ID,
			MerchantID:    current.MerchantID,
			StoreID:       current.StoreID,
			RefundNo:      refundNo,
			TransactionNo: current.TransactionNo,
			Amount:        amount,
			Reason:        reason,
			OperatorRole:  operatorRole,
			OperatorID:    operatorID,
			Status:        refundStatus,
			RawPayload:    rawPayload,
		}
		if err := tx.Create(refund).Error; err != nil {
			return err
		}
		current.RefundedAmount += amount
		if current.RefundedAmount >= current.TotalAmount {
			current.RefundStatus = "full"
		} else {
			current.RefundStatus = "partial"
		}
		current.OperationLogs = s.appendOrderLog(current.OperationLogs, "refund", fmt.Sprintf("%s发起退款：%s 元，原因：%s", operatorRole, formatFen(amount), reason))
		if err := tx.Save(&current).Error; err != nil {
			return err
		}
		updated = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (s *OrderService) bestPromotionForStore(merchantID, storeID uint, amount int64) (*model.Promotion, int64) {
	promotions, err := s.promotions.ListActiveForStore(merchantID, storeID)
	if err != nil {
		return nil, 0
	}
	var best *model.Promotion
	var bestDiscount int64
	for idx := range promotions {
		promotion := &promotions[idx]
		if amount < promotion.Threshold {
			continue
		}
		discount := promotion.Discount
		if promotion.Type == "discount" && promotion.DiscountRate > 0 && promotion.DiscountRate < 100 {
			discount = amount - (amount * int64(promotion.DiscountRate) / 100)
		}
		if discount > amount {
			discount = amount
		}
		if discount > bestDiscount {
			best = promotion
			bestDiscount = discount
		}
	}
	return best, bestDiscount
}

func (s *OrderService) appendOrderLog(raw, action, text string) string {
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

func orderHasAction(raw, action string) bool {
	var logs []OrderOperationLog
	if strings.TrimSpace(raw) == "" {
		return false
	}
	if err := json.Unmarshal([]byte(raw), &logs); err != nil {
		return false
	}
	for _, log := range logs {
		if log.Action == action {
			return true
		}
	}
	return false
}

func formatFen(value int64) string {
	return fmt.Sprintf("%.2f", float64(value)/100)
}

func (s *OrderService) UsageRecords(userID uint) ([]model.UsageRecord, error) {
	return s.systems.UserUsages(userID)
}

func (s *OrderService) AllOrders() ([]model.Order, error) {
	return s.orders.ListAll()
}

func (s *OrderService) AllPayments() ([]model.PaymentRecord, error) {
	return s.orders.ListPayments()
}

func (s *OrderService) AllRefunds() ([]model.RefundRecord, error) {
	return s.orders.ListRefunds()
}

func (s *OrderService) MerchantRefunds(merchantID uint) ([]model.RefundRecord, error) {
	return s.orders.ListRefundsByMerchant(merchantID)
}

func (s *OrderService) MerchantSettlements(merchantID uint) (*MerchantSettlementSummary, []model.MerchantSettlement, error) {
	var settlements []model.MerchantSettlement
	if err := s.db.Where("merchant_id = ?", merchantID).Order("id desc").Find(&settlements).Error; err != nil {
		return nil, nil, err
	}

	summary := &MerchantSettlementSummary{}
	summary.SettlementCount = int64(len(settlements))
	for _, item := range settlements {
		if item.Status == "paid" {
			summary.PaidSettlementCount++
			summary.PaidSettlementAmount += item.NetAmountCents
		}
		if item.Status == "pending" {
			summary.PendingSettlementCount++
		}
	}

	var orders []model.Order
	if err := s.db.Where("merchant_id = ? AND order_type = ? AND settlement_id IS NULL", merchantID, "store_order").
		Where("status IN ?", []string{"accepted", "completed"}).
		Find(&orders).Error; err != nil {
		return nil, nil, err
	}
	for _, order := range orders {
		summary.PendingOrderCount++
		amount := order.TotalAmount
		if amount <= 0 {
			amount = order.Amount
		}
		summary.PendingOrderAmount += amount
		summary.PendingRefundAmount += order.RefundedAmount
	}
	summary.PendingNetAmount = summary.PendingOrderAmount - summary.PendingRefundAmount
	if summary.PendingNetAmount < 0 {
		summary.PendingNetAmount = 0
	}

	return summary, settlements, nil
}

func (s *OrderService) applyPackageToUserTx(tx *gorm.DB, user *model.User, pkg *model.MembershipPackage, autoRenew bool) error {
	now := time.Now()
	base := now
	if user.ExpiredAt != nil && user.ExpiredAt.After(now) {
		base = *user.ExpiredAt
	}

	if pkg.IsLifetime {
		expire := time.Date(2099, 12, 31, 23, 59, 59, 0, time.Local)
		user.ExpiredAt = &expire
	} else {
		expire := base.AddDate(0, 0, pkg.DurationDays)
		user.ExpiredAt = &expire
	}

	user.MemberLevel = pkg.Code
	user.CurrentPackageID = &pkg.ID
	user.AutoRenew = autoRenew
	user.RemainingQuota += pkg.Quota
	return tx.Save(user).Error
}
