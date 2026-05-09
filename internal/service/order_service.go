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
}

func NewOrderService(db *gorm.DB) *OrderService {
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

	promotion, discount := s.bestPromotionForStore(store.MerchantID, store.ID, total)
	payable := total - discount
	if payable < 0 {
		payable = 0
	}

	channel := req.PaymentChannel
	if channel == "" {
		channel = "alipay"
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
		Status:         "pending",
		PaymentChannel: channel,
	}
	if promotion != nil {
		order.PromotionID = &promotion.ID
		order.OperationLogs = s.appendOrderLog("", "promotion_applied", fmt.Sprintf("已自动匹配优惠：%s，优惠 %s 元", promotion.Title, formatFen(discount)))
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
	if store.Status != "active" || !store.IsOpen || store.Merchant.Status != "active" {
		return nil, errors.New("store is unavailable")
	}
	return s.products.ListActiveByStore(storeID)
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
		if expectedAmount != req.Amount {
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
			if err := tx.Save(&order).Error; err != nil {
				return err
			}
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
	if order.Status != "received" {
		return errors.New("only received orders can be accepted")
	}
	order.Status = "accepted"
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "accepted", "商家已接单")
	return s.orders.Save(order)
}

func (s *OrderService) CompleteMerchantOrder(merchantID, orderID uint) error {
	order, err := s.orders.FindByMerchantAndID(merchantID, orderID)
	if err != nil {
		return errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return errors.New("order not found")
	}
	if order.Status != "accepted" {
		return errors.New("only accepted orders can be completed")
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
	if order.Status != "received" && order.Status != "accepted" {
		return errors.New("only received or accepted orders can be cancelled")
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
			RefundNo:      fmt.Sprintf("RF%d", time.Now().UnixNano()),
			TransactionNo: current.TransactionNo,
			Amount:        amount,
			Reason:        reason,
			OperatorRole:  operatorRole,
			OperatorID:    operatorID,
			Status:        "success",
			RawPayload:    `{"mode":"manual_record","note":"真实退款需由商家收款账户或支付渠道执行，本记录用于财务核对"}`,
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
