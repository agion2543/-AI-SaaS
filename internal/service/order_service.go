package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
)

type StoreOrderItemSnapshot struct {
	ProductID              uint                     `json:"product_id"`
	Name                   string                   `json:"name"`
	Price                  int64                    `json:"price"`
	BasePrice              int64                    `json:"base_price,omitempty"`
	Quantity               int                      `json:"quantity"`
	LineAmount             int64                    `json:"line_amount"`
	Description            string                   `json:"description,omitempty"`
	ImageURL               string                   `json:"image_url,omitempty"`
	SalesUnit              string                   `json:"sales_unit,omitempty"`
	Specification          string                   `json:"specification,omitempty"`
	MinimumOrderQuantity   int                      `json:"minimum_order_quantity,omitempty"`
	AppliedTierMinQuantity int                      `json:"applied_tier_min_quantity,omitempty"`
	PriceTiers             []model.ProductPriceTier `json:"price_tiers,omitempty"`
}

// orderItemsFromSnapshots converts priced order snapshots into order_items
// rows (schema v11). UnitPrice is the applied tier unit price and Amount the
// priced line amount; both are cents, the same unit as Order.TotalAmount.
// ProductNameSnapshot freezes the product name at order time.
func orderItemsFromSnapshots(orderID uint, snapshots []StoreOrderItemSnapshot) []model.OrderItem {
	rows := make([]model.OrderItem, 0, len(snapshots))
	for _, snap := range snapshots {
		rows = append(rows, model.OrderItem{
			OrderID:             orderID,
			ProductID:           snap.ProductID,
			ProductNameSnapshot: snap.Name,
			Quantity:            snap.Quantity,
			UnitPrice:           snap.Price,
			Amount:              snap.LineAmount,
		})
	}
	return rows
}

// writeStoreOrderItems persists the priced snapshots of one store order into
// order_items, in the caller's transaction. Callers create the order and its
// detail rows atomically so a store order never exists without its lines;
// idempotent replays return the existing order (rows already written by the
// first attempt) instead of writing again.
func writeStoreOrderItems(tx *gorm.DB, orderID uint, snapshots []StoreOrderItemSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	return tx.Create(orderItemsFromSnapshots(orderID, snapshots)).Error
}

type OrderOperationLog struct {
	Action                   string `json:"action"`
	Text                     string `json:"text"`
	Time                     string `json:"time"`
	AppendRequestKeyHash     string `json:"append_request_key_hash,omitempty"`
	AppendRequestFingerprint string `json:"append_request_fingerprint,omitempty"`
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

type refundGateway interface {
	Enabled() bool
	Refund(order *model.Order, refundNo string, amount int64, reason string) (string, error)
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
	alipay        refundGateway
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
		OrderNo:     fmt.Sprintf("ORD%d", time.Now().UnixNano()),
		OrderType:   "user_membership",
		UserID:      &userID,
		PackageID:   &pkg.ID,
		Amount:      pkg.Price,
		TotalAmount: pkg.Price,
		// Member orders have no product line items, but orders.items
		// is a MySQL JSON column: it rejects the empty string GORM would
		// otherwise write for the zero value (Error 3140). Store the empty
		// JSON array, the same value ConfirmMerchantSubscriptionPayment
		// already normalizes to before saving.
		Items:          "[]",
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
	if err := validateCustomerOrderRequestCredentials(req.RequestKey, req.AccessToken); err != nil {
		return nil, err
	}
	if req.RequestKey != "" {
		if existing, found, err := s.findIdempotentCustomerOrder(req); err != nil {
			return nil, err
		} else if found {
			return existing, nil
		}
	}

	store, err := s.stores.FindByIDWithMerchant(req.StoreID)
	if err != nil {
		return nil, errors.New("store not found")
	}
	if store.Status != "active" || !store.IsOpen || store.Merchant.Status != "active" {
		return nil, errors.New("store is unavailable")
	}
	// 订阅门控：过期/暂停/未开通订阅的商家不得接新单（H1）。只拦新下单，
	// 在途订单的付款、确认与退款不受影响。试用期商家是 active + 未到期，
	// 正常通过本门。
	if err := requireMerchantAcceptsNewStoreOrders(store.Merchant, time.Now()); err != nil {
		return nil, err
	}
	// 维护进行中时拦截新下单（在途订单/支付/退款不受影响，继续完成）。
	if blocked, plan, err := MaintenanceBlocksModule(s.db, MaintenanceModuleOrdering); err != nil {
		return nil, err
	} else if blocked {
		return nil, MaintenanceOrderBlockError(plan)
	}
	if req.CustomerPhone != "" && !isValidMainlandPhone(normalizePhone(req.CustomerPhone)) {
		return nil, errors.New("customer phone must be 11 digits")
	}
	orderItems, err := aggregateCustomerOrderItems(req.Items)
	if err != nil {
		return nil, err
	}
	productIDs := make([]uint, 0, len(orderItems))
	for _, item := range orderItems {
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

	snapshots := make([]StoreOrderItemSnapshot, 0, len(orderItems))
	var total int64
	for _, item := range orderItems {
		product, ok := productMap[item.ProductID]
		if !ok {
			return nil, errors.New("some products are unavailable")
		}
		snapshot, lineAmount, err := buildStoreOrderItemSnapshot(product, item.Quantity)
		if err != nil {
			return nil, err
		}
		total, err = addStoreOrderAmount(total, lineAmount)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
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
	} else if payable == 0 {
		status = "submitted"
	}

	shareCode := strings.TrimSpace(req.ShareCode)
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
		Channel:        OrderChannelForShareCode(shareCode),
		ShareCode:      shareCode,
	}
	if req.RequestKey != "" {
		order.OrderNo = customerOrderNoForRequestKey(req.RequestKey)
	}
	if store.OrderMode == "submit_later" {
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "submitted", "顾客已提交订单，稍后结算")
		if store.AutoAccept {
			order.OperationLogs = s.appendOrderLog(order.OperationLogs, "auto_accepted", "门店已开启自动接单，订单进入处理中")
		}
	} else if payable == 0 {
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "submitted", "本单无需付款，已发送给门店")
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
	accessToken := req.AccessToken
	var tokenHash string
	if req.RequestKey != "" {
		tokenHash = hashCustomerOrderAccessToken(accessToken)
	} else {
		accessToken, tokenHash, err = newCustomerOrderAccessToken()
		if err != nil {
			return nil, errors.New("could not generate customer order access token")
		}
	}
	order.CustomerAccessTokenHash = tokenHash
	order.CustomerAccessToken = accessToken

	// The order and its order_items rows are created in one transaction so a
	// store order never exists without its detail lines. A duplicate order_no
	// (idempotent replay) aborts the transaction; the lookup below then
	// returns the order created by the first attempt, whose rows are already
	// in place.
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := repository.NewOrderRepository(tx).Create(order); err != nil {
			return err
		}
		return writeStoreOrderItems(tx, order.ID, snapshots)
	}); err != nil {
		if req.RequestKey != "" {
			if existing, found, lookupErr := s.findIdempotentCustomerOrder(req); lookupErr != nil {
				return nil, lookupErr
			} else if found {
				return existing, nil
			}
		}
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

func (s *OrderService) PublicOrderDetail(orderNo, accessToken string) (*model.Order, error) {
	order, err := s.findCustomerOrderWithAccess(orderNo, accessToken)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if isUnpaidCheckoutExpired(*order, now) {
		if err := s.expireCheckoutOrderIfDue(order.ID, now); err != nil {
			return nil, err
		}
		return s.findCustomerOrderWithAccess(orderNo, accessToken)
	}
	return order, nil
}

func (s *OrderService) findCustomerOrderWithAccess(orderNo, accessToken string) (*model.Order, error) {
	order, err := s.orders.FindByOrderNo(orderNo)
	if err != nil || order.OrderType != "store_order" || !customerOrderAccessTokenMatches(order, accessToken) {
		return nil, errors.New("order not found")
	}
	if err := NewTableOrderingService(s.db, s).AttachTableNumber(order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) PublicPaymentConfigForOrder(order *model.Order) (*model.MerchantPaymentConfig, error) {
	if order == nil || order.OrderType != "store_order" || order.MerchantID == nil {
		return nil, nil
	}
	return loadPublicMerchantPaymentConfig(s.db, *order.MerchantID)
}

func loadPublicMerchantPaymentConfig(db *gorm.DB, merchantID uint) (*model.MerchantPaymentConfig, error) {
	var config model.MerchantPaymentConfig
	err := db.Where(
		"merchant_id = ? AND mode = ? AND status = ? AND audit_status = ?",
		merchantID,
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
	config.AlipayQRCode = strings.TrimSpace(config.AlipayQRCode)
	config.WechatQRCode = strings.TrimSpace(config.WechatQRCode)
	if config.AlipayQRCode == "" && config.WechatQRCode == "" {
		return nil, nil
	}
	return &config, nil
}

func (s *OrderService) MarkCustomerStoreOrderPaid(orderNo, accessToken string) (*model.Order, error) {
	order, err := s.PublicOrderDetail(orderNo, accessToken)
	if err != nil {
		return nil, err
	}
	if order.PaidAt != nil {
		return order, nil
	}
	if order.TotalAmount <= 0 {
		return nil, errors.New("this order does not require payment")
	}
	isPendingTableMerchantQR := order.TableNo > 0 && order.PaymentChannel == "merchant_qr" && order.Status == "pending"
	if order.Status != "submitted" && order.Status != "preparing" && !isPendingTableMerchantQR {
		return nil, errors.New("only submitted orders or merchant-QR table checkouts can be marked as paid")
	}
	if isPendingTableMerchantQR {
		config, err := s.PublicPaymentConfigForOrder(order)
		if err != nil {
			return nil, err
		}
		if config == nil {
			return nil, errors.New("商家收款码当前不可用，请联系门店核实")
		}
	}
	order.Status = "payment_confirming"
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "customer_paid", "顾客已标记付款，等待商家确认收款")
	if err := s.orders.Save(order); err != nil {
		return nil, err
	}
	updated, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, err
	}
	if err := NewTableOrderingService(s.db, s).AttachTableNumber(updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *OrderService) appendCustomerStoreOrderItemsLocked(orderNo, accessToken string, req dto.AppendCustomerOrderItemsRequest) (*model.Order, error) {
	order, err := s.findCustomerOrderWithAccess(orderNo, accessToken)
	if err != nil || order.StoreID == nil {
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
	// 订阅门控（H1）：追加菜品属于新增消费，过期/暂停/未开通订阅的商家不得
	// 再追加；在途订单的付款与确认不受影响。
	if err := requireMerchantAcceptsNewStoreOrders(store.Merchant, time.Now()); err != nil {
		return nil, err
	}

	orderItems, err := aggregateCustomerOrderItems(req.Items)
	if err != nil {
		return nil, err
	}
	productIDs := make([]uint, 0, len(orderItems))
	for _, item := range orderItems {
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
	var appendedAmount int64
	var appendedQuantity int
	appendedSnapshots := make([]StoreOrderItemSnapshot, 0, len(orderItems))
	appendedNames := make([]string, 0, len(orderItems))
	for _, item := range orderItems {
		product, ok := productMap[item.ProductID]
		if !ok {
			return nil, errors.New("some products are unavailable")
		}
		snapshot, lineAmount, err := buildStoreOrderItemSnapshot(product, item.Quantity)
		if err != nil {
			return nil, err
		}
		appendedAmount, err = addStoreOrderAmount(appendedAmount, lineAmount)
		if err != nil {
			return nil, err
		}
		if item.Quantity > int(^uint(0)>>1)-appendedQuantity {
			return nil, errors.New("product quantity exceeds the allowed range")
		}
		appendedQuantity += item.Quantity
		appendedNames = append(appendedNames, fmt.Sprintf("%s x%d", product.Name, item.Quantity))
		appendedSnapshots = append(appendedSnapshots, snapshot)
		existing = append(existing, snapshot)
	}

	itemsJSON, err := json.Marshal(existing)
	if err != nil {
		return nil, err
	}
	order.Items = string(itemsJSON)
	order.Amount, err = addStoreOrderAmount(order.Amount, appendedAmount)
	if err != nil {
		return nil, err
	}
	order.TotalAmount, err = addStoreOrderAmount(order.TotalAmount, appendedAmount)
	if err != nil {
		return nil, err
	}
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
	order.OperationLogs, err = appendCustomerOrderItemsLog(order.OperationLogs, fmt.Sprintf("顾客追加商品：%s，新增 %d 件，新增金额 %s 元", strings.Join(appendedNames, "、"), appendedQuantity, formatFen(appendedAmount)), req.RequestKey, req.RequestFingerprint)
	if err != nil {
		return nil, err
	}
	if err := s.orders.Save(order); err != nil {
		return nil, err
	}
	// s.db is the caller's transaction here, so the appended detail rows land
	// atomically with the updated order totals.
	if err := writeStoreOrderItems(s.db, order.ID, appendedSnapshots); err != nil {
		return nil, err
	}
	updated, err := s.orders.FindByOrderNo(orderNo)
	if err != nil {
		return nil, err
	}
	if err := NewTableOrderingService(s.db, s).AttachTableNumber(updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *OrderService) PrepareCustomerOrderRetry(orderNo, accessToken string) (*model.Order, error) {
	order, err := s.findCustomerOrderWithAccess(orderNo, accessToken)
	if err != nil {
		return nil, err
	}
	if isUnpaidCheckoutExpired(*order, time.Now()) {
		if err := s.expireCheckoutOrderIfDue(order.ID, time.Now()); err != nil {
			return nil, err
		}
		return nil, errors.New("payment window expired; order is closed")
	}
	if order.TableNo > 0 {
		return nil, errors.New("桌位订单不可使用平台支付入口，请使用商家收款码并等待门店核对")
	}
	if order.TotalAmount <= 0 || order.PaidAt != nil {
		return nil, errors.New("this order does not require payment")
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

func (s *OrderService) ReviewCustomerOrder(orderNo, accessToken string, req dto.ReviewCustomerOrderRequest) (*model.Order, error) {
	order, err := s.findCustomerOrderWithAccess(orderNo, accessToken)
	if err != nil {
		return nil, err
	}
	if !orderPaymentSatisfied(*order) {
		return nil, errors.New("unpaid orders cannot be reviewed")
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

type merchantSubscriptionOrderTerms struct {
	Name         string
	PriceCents   int64
	DurationDays int
}

func merchantSubscriptionTermsFromOrder(order *model.Order) (merchantSubscriptionOrderTerms, error) {
	if order == nil || order.OrderType != "merchant_subscription" || order.MerchantID == nil || order.MerchantPlanID == nil || order.PlanNameSnapshot == nil || order.PlanPriceCentsSnapshot == nil || order.PlanDurationDaysSnapshot == nil {
		return merchantSubscriptionOrderTerms{}, errors.New("merchant subscription order is invalid or missing immutable snapshot")
	}
	terms := merchantSubscriptionOrderTerms{
		Name:         strings.TrimSpace(*order.PlanNameSnapshot),
		PriceCents:   *order.PlanPriceCentsSnapshot,
		DurationDays: *order.PlanDurationDaysSnapshot,
	}
	if terms.Name == "" || terms.PriceCents < 0 || terms.DurationDays <= 0 {
		return merchantSubscriptionOrderTerms{}, errors.New("merchant subscription order snapshot is invalid")
	}
	if order.Amount != terms.PriceCents || order.TotalAmount != terms.PriceCents {
		return merchantSubscriptionOrderTerms{}, errors.New("merchant subscription order amount does not match its snapshot")
	}
	return terms, nil
}

func (s *OrderService) HandlePaymentCallback(req dto.PaymentCallbackRequest) (*model.Order, error) {
	var result model.Order

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Package").Preload("User").Preload("Merchant").Preload("MerchantPlan").Preload("Store").Where("order_no = ?", req.OrderNo).First(&order).Error; err != nil {
			return err
		}

		if req.Status != "success" && req.Status != "failed" {
			return errors.New("unsupported payment callback status")
		}
		if err := validatePaymentCallbackAmount(order, req.Amount); err != nil {
			return err
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
		if order.OrderType == "merchant_subscription" && order.PaymentChannel == "platform_qr" {
			return errors.New("platform static QR subscription payments require manual reconciliation")
		}
		if isUnpaidCheckoutExpired(order, time.Now()) {
			order.Status = "closed"
			order.OperationLogs = s.appendOrderLog(order.OperationLogs, "checkout_expired", "未付款订单超过30分钟有效期，系统已关闭")
			if err := tx.Save(&order).Error; err != nil {
				return err
			}
		}

		var subscriptionTerms merchantSubscriptionOrderTerms
		if req.Status == "success" && order.OrderType == "merchant_subscription" && order.Status == "pending" {
			terms, err := merchantSubscriptionTermsFromOrder(&order)
			if err != nil {
				return err
			}
			subscriptionTerms = terms
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
		exceptionAction := ""
		exceptionText := ""
		if req.Status == "success" {
			alreadySettled := order.PaidAt != nil || order.Status == "paid" || order.Status == "received" || order.Status == "accepted" || order.Status == "completed"
			if alreadySettled {
				paymentStatus = "duplicate_success"
				exceptionAction = "duplicate_payment_exception"
				exceptionText = fmt.Sprintf("检测到同一订单的另一笔成功收款，已登记为重复收款异常；交易号：%s，金额：%s 元", req.TransactionNo, formatFen(req.Amount))
			} else if order.Status != "pending" {
				paymentStatus = model.PaymentStatusPaidReviewRequired
				exceptionAction = "late_payment_exception"
				exceptionText = fmt.Sprintf("支付渠道已确认付款，但订单已过期或不允许自动履约；已保留为已付款待人工核对，系统不会自动履约、开通订阅权益或退款；交易号：%s，金额：%s 元", req.TransactionNo, formatFen(req.Amount))
			}
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
		if exceptionAction != "" {
			if order.PaidAt == nil {
				paidAt := time.Now()
				order.PaidAt = &paidAt
				order.TransactionNo = req.TransactionNo
				order.PaymentChannel = req.PaymentChannel
			}
			order.OperationLogs = s.appendOrderLog(order.OperationLogs, exceptionAction, exceptionText)
			if err := tx.Save(&order).Error; err != nil {
				return err
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
			var merchant model.Merchant
			if err := tx.First(&merchant, *order.MerchantID).Error; err != nil {
				return err
			}
			base := paidAt
			if merchant.SubscriptionExpireAt != nil && merchant.SubscriptionExpireAt.After(base) {
				base = *merchant.SubscriptionExpireAt
			}
			expire := base.AddDate(0, 0, subscriptionTerms.DurationDays)
			order.SubscriptionEndAt = &expire
			if err := tx.Save(&order).Error; err != nil {
				return err
			}
			merchant.SubscriptionPlanID = order.MerchantPlanID
			merchant.SubscriptionExpireAt = &expire
			merchant.SubscriptionPlan = subscriptionTerms.Name
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

func (s *OrderService) ConfirmMerchantSubscriptionPayment(orderID uint, remark string, audit ...AuditEvent) (*model.Order, error) {
	var result model.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Merchant").Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		if order.OrderType != "merchant_subscription" {
			return errors.New("only merchant subscription orders can be confirmed")
		}
		reconcileExpiredStaticQR := order.Status == "closed" && order.PaymentChannel == "platform_qr" && order.PaidAt == nil && orderHasAction(order.OperationLogs, "checkout_expired")
		if order.Status != "pending" && !reconcileExpiredStaticQR {
			return errors.New("only pending orders or expired unpaid platform QR subscriptions can be confirmed")
		}
		terms, err := merchantSubscriptionTermsFromOrder(&order)
		if err != nil {
			return err
		}

		var merchant model.Merchant
		if err := tx.First(&merchant, *order.MerchantID).Error; err != nil {
			return err
		}

		paidAt := time.Now()
		base := paidAt
		if merchant.SubscriptionExpireAt != nil && merchant.SubscriptionExpireAt.After(base) {
			base = *merchant.SubscriptionExpireAt
		}
		expire := base.AddDate(0, 0, terms.DurationDays)
		transactionNo := fmt.Sprintf("MANUAL-MSUB-%d", order.ID)
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
			Amount:         terms.PriceCents,
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
		confirmationAction := "subscription_payment_confirmed"
		confirmationText := "平台已确认订阅款到账，商家订阅已开通或续期"
		if reconcileExpiredStaticQR {
			confirmationAction = "subscription_payment_reconciled_after_expiry"
			confirmationText = "管理员已人工核对平台静态二维码收款账户确认到账；已关闭的订阅订单经人工对账后开通或续期"
		}
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, confirmationAction, confirmationText)
		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		merchant.SubscriptionPlanID = order.MerchantPlanID
		merchant.SubscriptionPlan = terms.Name
		merchant.SubscriptionStatus = "active"
		merchant.SubscriptionExpireAt = &expire
		merchant.SubscriptionExpiredAt = &expire
		merchant.SubscriptionNote = "平台确认订阅款到账后开通"
		if err := tx.Save(&merchant).Error; err != nil {
			return err
		}
		if len(audit) > 0 {
			event := audit[0]
			event.Input.TargetType = "order"
			event.Input.TargetID = order.ID
			event.Input.TargetName = order.OrderNo
			event.Input.MerchantID = order.MerchantID
			if event.Input.Detail == nil {
				event.Input.Detail = map[string]interface{}{}
			}
			event.Input.Detail["amount"] = terms.PriceCents
			event.Input.Detail["merchant"] = merchant.Name
			event.Input.Detail["plan"] = terms.Name
			event.Input.Detail["order_no"] = order.OrderNo
			event.Input.Detail["payment_mode"] = "platform_qr"
			event.Input.Detail["reconciliation_after_expiry"] = reconcileExpiredStaticQR
			event.Input.Detail["expire_at"] = expire
			event.Input.Detail["result"] = "subscription_opened"
			if err := appendOptionalAuditTx(tx, []AuditEvent{event}); err != nil {
				return fmt.Errorf("persist subscription payment audit: %w", err)
			}
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

func (s *OrderService) MerchantOrders(merchantID uint, status string, storeID *uint, keyword string, page, pageSize int) ([]model.Order, int64, error) {
	orders, total, err := s.orders.ListByMerchant(merchantID, status, storeID, keyword, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if err := NewTableOrderingService(s.db, s).AttachTableNumbers(orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (s *OrderService) MerchantOrderDetail(merchantID, orderID uint) (*model.Order, error) {
	order, err := s.orders.FindByMerchantAndID(merchantID, orderID)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if order.OrderType != "store_order" {
		return nil, errors.New("order not found")
	}
	if err := NewTableOrderingService(s.db, s).AttachTableNumber(order); err != nil {
		return nil, err
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
	if !orderCanProceedToFulfillment(*order) {
		return errors.New("payment must be confirmed before accepting this order")
	}
	order.Status = "accepted"
	order.OperationLogs = s.appendOrderLog(order.OperationLogs, "accepted", "商家已接单")
	return s.orders.Save(order)
}

func (s *OrderService) ConfirmMerchantOrderPayment(merchantID, orderID uint, audit ...AuditEvent) (*model.Order, error) {
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
		if order.TotalAmount <= 0 {
			return errors.New("this order does not require payment")
		}
		if order.Status != "payment_confirming" && order.Status != "submitted" && order.Status != "preparing" && order.Status != "accepted" {
			return errors.New("only unpaid submitted, processing or accepted orders can be confirmed")
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
		// v15: record who confirmed the payment on the payment record
		// itself, in the same transaction as the audit row below (M1
		// follow-up). The operator comes from the audit actor; without an
		// audit event the columns stay 0/"" (unknown operator) rather
		// than guessing from the merchant id.
		if len(audit) > 0 {
			record.OperatorID = audit[0].Actor.ID
			record.OperatorRole = audit[0].Actor.Type
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		order.PaidAt = &paidAt
		order.TransactionNo = transactionNo
		order.PaymentChannel = "merchant_qr"
		if order.Status == "accepted" || orderHasAction(order.OperationLogs, "auto_accepted") {
			order.Status = "accepted"
		} else {
			order.Status = "received"
		}
		order.OperationLogs = s.appendOrderLog(order.OperationLogs, "merchant_payment_confirmed", "商家已确认收到顾客付款")
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		// M1: merchant payment confirmation is a money-relevant act: record
		// who confirmed it in the same transaction as the PaymentRecord.
		// Amount and transaction number are recoverable from the order and
		// its payment record; the audit row carries actor/action/target.
		if len(audit) > 0 {
			event := audit[0]
			event.Input.TargetType = "order"
			event.Input.TargetID = order.ID
			event.Input.TargetName = order.OrderNo
			event.Input.MerchantID = &merchantID
			if err := appendOptionalAuditTx(tx, []AuditEvent{event}); err != nil {
				return err
			}
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
	if !orderPaymentSatisfied(*order) {
		return errors.New("payment must be confirmed before completing this order")
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

func (s *OrderService) RefundMerchantOrder(merchantID, orderID uint, req dto.RefundOrderRequest, operatorID uint, operatorRole string, audit ...AuditEvent) (*model.Order, bool, error) {
	order, err := s.orders.FindByMerchantAndID(merchantID, orderID)
	if err != nil {
		return nil, false, errors.New("order not found")
	}
	return s.refundOrder(order, req, operatorID, operatorRole, audit...)
}

func (s *OrderService) RefundAdminOrder(orderID uint, req dto.RefundOrderRequest, operatorID uint, audit ...AuditEvent) (*model.Order, bool, error) {
	var order model.Order
	if err := s.db.Preload("Store").Preload("Merchant").First(&order, orderID).Error; err != nil {
		return nil, false, errors.New("order not found")
	}
	// Record the admin's effective role (super_admin vs admin) from the
	// audit actor instead of a fixed "admin": the M2 review check compares
	// (role, id) pairs, and a super-admin applicant must not be able to
	// approve their own refund under a different label.
	operatorRole := "admin"
	if len(audit) > 0 && audit[0].Actor.Type != "" {
		operatorRole = audit[0].Actor.Type
	}
	return s.refundOrder(&order, req, operatorID, operatorRole, audit...)
}

func (s *OrderService) refundOrder(order *model.Order, req dto.RefundOrderRequest, operatorID uint, operatorRole string, audit ...AuditEvent) (*model.Order, bool, error) {
	return s.processReservedRefund(order, req, operatorID, operatorRole, audit...)
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
	orders, err := s.orders.ListAll()
	if err != nil || len(orders) == 0 {
		return orders, err
	}
	if err := NewTableOrderingService(s.db, s).AttachTableNumbers(orders); err != nil {
		return nil, err
	}
	hasReviews := s.db.Migrator().HasTable(&model.MerchantSubscriptionManualReview{})
	hasAudits := s.db.Migrator().HasTable(&model.MerchantSubscriptionManualReviewAudit{})
	if !hasReviews && !hasAudits {
		return orders, nil
	}
	if hasReviews != hasAudits {
		return nil, errors.New("subscription manual review schema is incomplete")
	}
	orderIDs := make([]uint, 0, len(orders))
	for _, order := range orders {
		if order.OrderType == "merchant_subscription" {
			orderIDs = append(orderIDs, order.ID)
		}
	}
	if len(orderIDs) == 0 {
		return orders, nil
	}
	var reviews []model.MerchantSubscriptionManualReview
	if err := s.db.Where("order_id IN ?", orderIDs).Find(&reviews).Error; err != nil {
		return nil, fmt.Errorf("load subscription manual review records: %w", err)
	}
	reviewByOrder := make(map[uint]model.MerchantSubscriptionManualReview, len(reviews))
	reviewIDs := make([]uint, 0, len(reviews))
	for _, review := range reviews {
		reviewByOrder[review.OrderID] = review
		reviewIDs = append(reviewIDs, review.ID)
	}
	audits := make([]model.MerchantSubscriptionManualReviewAudit, 0, len(reviewIDs))
	if len(reviewIDs) > 0 {
		if err := s.db.Where("review_id IN ?", reviewIDs).Find(&audits).Error; err != nil {
			return nil, fmt.Errorf("load subscription manual review audits: %w", err)
		}
	}
	auditByReview := make(map[uint]model.MerchantSubscriptionManualReviewAudit, len(audits))
	for _, audit := range audits {
		auditByReview[audit.ReviewID] = audit
	}
	for i := range orders {
		review, ok := reviewByOrder[orders[i].ID]
		if !ok {
			continue
		}
		audit, auditOK := auditByReview[review.ID]
		if !auditOK {
			return nil, fmt.Errorf("manual review %d is missing its immutable audit", review.ID)
		}
		orders[i].ManualReview = &review
		orders[i].ManualReviewAudit = &audit
	}
	return orders, nil
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
		Where("status IN ? AND paid_at IS NOT NULL AND total_amount > 0", []string{"accepted", "completed"}).
		Find(&orders).Error; err != nil {
		return nil, nil, err
	}
	for _, order := range orders {
		summary.PendingOrderCount++
		summary.PendingOrderAmount += settlementOrderAmount(order)
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
