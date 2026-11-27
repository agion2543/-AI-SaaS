package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const (
	tableSessionOpen    = "open"
	tableSessionLocked  = "checkout_locked"
	maxConfiguredTables = 300
)

type TableOrderingService struct {
	db     *gorm.DB
	orders *OrderService
}

func NewTableOrderingService(db *gorm.DB, orders *OrderService) *TableOrderingService {
	if orders == nil {
		orders = NewOrderService(db)
	}
	return &TableOrderingService{db: db, orders: orders}
}

func (s *TableOrderingService) PublicPaymentConfigForOrder(order *model.Order) (*model.MerchantPaymentConfig, error) {
	return s.orders.PublicPaymentConfigForOrder(order)
}

type TableQRCode struct {
	TableNo    int    `json:"table_no"`
	TableLabel string `json:"table_label"`
	URL        string `json:"url"`
}

type TableCartLine struct {
	ProductID     uint                     `json:"product_id"`
	Name          string                   `json:"name"`
	Price         int64                    `json:"price"`
	Quantity      int                      `json:"quantity"`
	LineAmount    int64                    `json:"line_amount"`
	ImageURL      string                   `json:"image_url,omitempty"`
	Description   string                   `json:"description,omitempty"`
	SalesUnit     string                   `json:"sales_unit,omitempty"`
	Specification string                   `json:"specification,omitempty"`
	PriceTiers    []model.ProductPriceTier `json:"price_tiers,omitempty"`
}

type TableSessionSnapshot struct {
	ID              uint            `json:"id"`
	StoreID         uint            `json:"store_id"`
	TableNo         int             `json:"table_no"`
	TableLabel      string          `json:"table_label"`
	Status          string          `json:"status"`
	Locked          bool            `json:"locked"`
	CartVersion     uint64          `json:"cart_version"`
	CheckoutOrderNo string          `json:"checkout_order_no,omitempty"`
	Items           []TableCartLine `json:"items"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type StoreTableConfigView struct {
	TableCount int           `json:"table_count"`
	Tables     []TableQRCode `json:"tables"`
}

func (s *TableOrderingService) ConfigureStoreTables(merchantID, storeID uint, count int, labels []dto.ConfigureStoreTableLabelItem, makeURL func(int, string) string) (*StoreTableConfigView, error) {
	if count < 0 || count > maxConfiguredTables {
		return nil, fmt.Errorf("桌数需在0到%d之间", maxConfiguredTables)
	}
	requestedLabels := make(map[int]string, len(labels))
	for _, item := range labels {
		if item.TableNo < 1 || item.TableNo > count {
			return nil, fmt.Errorf("桌位编号需在1到当前桌数%d之间", count)
		}
		if _, exists := requestedLabels[item.TableNo]; exists {
			return nil, fmt.Errorf("桌位编号%d重复", item.TableNo)
		}
		label := strings.TrimSpace(item.TableLabel)
		if label == "" {
			label = defaultTableLabel(item.TableNo)
		}
		if utf8.RuneCountInString(label) > 64 {
			return nil, fmt.Errorf("桌位标签不能超过64个字符（桌位%d）", item.TableNo)
		}
		requestedLabels[item.TableNo] = label
	}
	var view *StoreTableConfigView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var store model.Store
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND merchant_id = ?", storeID, merchantID).First(&store).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("门店不存在或无权管理")
			}
			return err
		}
		var config model.StoreTableConfig
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("store_id = ?", storeID).First(&config).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			config = model.StoreTableConfig{StoreID: storeID, TableCount: count}
			if err := tx.Create(&config).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if err := tx.Model(&config).Update("table_count", count).Error; err != nil {
			return err
		}
		for tableNo := 1; tableNo <= count; tableNo++ {
			var table model.StoreTable
			err := tx.Where("store_id = ? AND table_no = ?", storeID, tableNo).First(&table).Error
			if err == nil {
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			code, err := newTableQRCodeSecret()
			if err != nil {
				return err
			}
			if err := tx.Create(&model.StoreTable{StoreID: storeID, TableNo: tableNo, TableLabel: defaultTableLabel(tableNo), TableCode: code}).Error; err != nil {
				return err
			}
		}
		rows := make([]model.StoreTable, 0, count)
		if count > 0 {
			if err := tx.Where("store_id = ? AND table_no <= ?", storeID, count).Order("table_no ASC").Find(&rows).Error; err != nil {
				return err
			}
		}
		seenLabels := make(map[string]int, len(rows))
		for i := range rows {
			row := &rows[i]
			if label, ok := requestedLabels[row.TableNo]; ok {
				row.TableLabel = label
				if err := tx.Model(row).Update("table_label", label).Error; err != nil {
					return err
				}
			}
			label := effectiveTableLabel(row.TableNo, row.TableLabel)
			key := strings.ToLower(label)
			if prior, exists := seenLabels[key]; exists {
				return fmt.Errorf("桌位标签“%s”重复（桌位%d和%d）", label, prior, row.TableNo)
			}
			seenLabels[key] = row.TableNo
		}
		view, err = s.buildStoreTableConfig(tx, storeID, count, makeURL)
		return err
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

func (s *TableOrderingService) StoreTableConfig(merchantID, storeID uint, makeURL func(int, string) string) (*StoreTableConfigView, error) {
	var store model.Store
	if err := s.db.Where("id = ? AND merchant_id = ?", storeID, merchantID).First(&store).Error; err != nil {
		return nil, errors.New("门店不存在或无权管理")
	}
	var config model.StoreTableConfig
	if err := s.db.Where("store_id = ?", storeID).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &StoreTableConfigView{TableCount: 0, Tables: []TableQRCode{}}, nil
		}
		return nil, err
	}
	return s.buildStoreTableConfig(s.db, storeID, config.TableCount, makeURL)
}

func (s *TableOrderingService) buildStoreTableConfig(db *gorm.DB, storeID uint, count int, makeURL func(int, string) string) (*StoreTableConfigView, error) {
	rows := make([]model.StoreTable, 0, count)
	if count > 0 {
		if err := db.Where("store_id = ? AND table_no <= ?", storeID, count).Order("table_no ASC").Find(&rows).Error; err != nil {
			return nil, err
		}
	}
	view := &StoreTableConfigView{TableCount: count, Tables: make([]TableQRCode, 0, len(rows))}
	for _, row := range rows {
		view.Tables = append(view.Tables, TableQRCode{TableNo: row.TableNo, TableLabel: effectiveTableLabel(row.TableNo, row.TableLabel), URL: makeURL(row.TableNo, row.TableCode)})
	}
	return view, nil
}

func defaultTableLabel(tableNo int) string {
	return fmt.Sprintf("%d号桌", tableNo)
}

func effectiveTableLabel(tableNo int, label string) string {
	if trimmed := strings.TrimSpace(label); trimmed != "" {
		return trimmed
	}
	return defaultTableLabel(tableNo)
}

func newTableQRCodeSecret() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("生成桌码失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *TableOrderingService) JoinTableSession(req dto.JoinTableSessionRequest) (*TableSessionSnapshot, error) {
	var session model.TableSession
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.authorizeTableCode(tx, req.StoreID, req.TableNo, req.TableCode); err != nil {
			return err
		}
		var store model.Store
		if err := tx.Preload("Merchant").First(&store, req.StoreID).Error; err != nil {
			return errors.New("门店不存在")
		}
		if store.Status != "active" || store.Merchant.Status != "active" {
			return errors.New("门店暂不可点单")
		}
		lookupErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("store_id = ? AND table_no = ?", req.StoreID, req.TableNo).First(&session).Error
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			session = model.TableSession{StoreID: req.StoreID, TableNo: req.TableNo, Status: tableSessionOpen}
			if err := tx.Create(&session).Error; err != nil {
				return err
			}
		} else if lookupErr != nil {
			return lookupErr
		}
		if err := s.reopenTableSessionIfOrderEnded(tx, &session); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.SessionSnapshot(session.ID, req.TableCode)
}

func (s *TableOrderingService) SessionSnapshot(sessionID uint, tableCode string) (*TableSessionSnapshot, error) {
	var session model.TableSession
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, sessionID).Error; err != nil {
			return errors.New("桌位会话不存在")
		}
		if _, err := s.authorizeTableCode(tx, session.StoreID, session.TableNo, tableCode); err != nil {
			return err
		}
		if err := s.reopenTableSessionIfOrderEnded(tx, &session); err != nil {
			return err
		}
		return s.removeUnavailableCartItems(tx, &session)
	})
	if err != nil {
		return nil, err
	}
	return s.snapshotForSession(s.db, session)
}

func (s *TableOrderingService) UpdateCartItem(sessionID uint, tableCode string, req dto.UpdateTableCartItemRequest) (*TableSessionSnapshot, error) {
	var session model.TableSession
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, sessionID).Error; err != nil {
			return errors.New("桌位会话不存在")
		}
		if _, err := s.authorizeTableCode(tx, session.StoreID, session.TableNo, tableCode); err != nil {
			return err
		}
		if err := s.reopenTableSessionIfOrderEnded(tx, &session); err != nil {
			return err
		}
		if session.Status != tableSessionOpen {
			return errors.New("本桌已有顾客发起结算，购物车已锁定，不能继续加购")
		}
		if err := s.removeUnavailableCartItems(tx, &session); err != nil {
			return err
		}
		var store model.Store
		if err := tx.Preload("Merchant").First(&store, session.StoreID).Error; err != nil || store.Status != "active" || !store.IsOpen || store.Merchant.Status != "active" {
			return errors.New("门店当前不接收新点单")
		}
		switch req.Action {
		case "remove":
			if err := tx.Where("session_id = ? AND product_id = ?", session.ID, req.ProductID).Delete(&model.TableCartItem{}).Error; err != nil {
				return err
			}
		case "add":
			if req.Quantity < 1 || req.Quantity > 99 {
				return errors.New("每次增加数量需在1到99之间")
			}
			var product model.StoreProduct
			if err := tx.Where("id = ? AND store_id = ? AND status = ?", req.ProductID, session.StoreID, "active").First(&product).Error; err != nil {
				return errors.New("商品已下架或不存在")
			}
			var item model.TableCartItem
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("session_id = ? AND product_id = ?", session.ID, req.ProductID).First(&item).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				item = model.TableCartItem{SessionID: session.ID, ProductID: req.ProductID, Quantity: req.Quantity}
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				item.Quantity += req.Quantity
				if item.Quantity > 99 {
					return errors.New("单品数量不能超过99")
				}
				if err := tx.Model(&item).Update("quantity", item.Quantity).Error; err != nil {
					return err
				}
			}
		case "set":
			if req.Quantity == 0 {
				if err := tx.Where("session_id = ? AND product_id = ?", session.ID, req.ProductID).Delete(&model.TableCartItem{}).Error; err != nil {
					return err
				}
			} else {
				var product model.StoreProduct
				if err := tx.Where("id = ? AND store_id = ? AND status = ?", req.ProductID, session.StoreID, "active").First(&product).Error; err != nil {
					return errors.New("商品已下架或不存在")
				}
				var item model.TableCartItem
				err := tx.Where("session_id = ? AND product_id = ?", session.ID, req.ProductID).First(&item).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					item = model.TableCartItem{SessionID: session.ID, ProductID: req.ProductID, Quantity: req.Quantity}
					if err := tx.Create(&item).Error; err != nil {
						return err
					}
				} else if err != nil {
					return err
				} else if err := tx.Model(&item).Update("quantity", req.Quantity).Error; err != nil {
					return err
				}
			}
		default:
			return errors.New("无效的购物车操作")
		}
		session.CartVersion++
		if err := tx.Model(&session).Update("cart_version", session.CartVersion).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.snapshotForSession(s.db, session)
}

func (s *TableOrderingService) ClearCart(sessionID uint, tableCode string) (*TableSessionSnapshot, error) {
	var session model.TableSession
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, sessionID).Error; err != nil {
			return errors.New("桌位会话不存在")
		}
		if _, err := s.authorizeTableCode(tx, session.StoreID, session.TableNo, tableCode); err != nil {
			return err
		}
		if err := s.reopenTableSessionIfOrderEnded(tx, &session); err != nil {
			return err
		}
		if session.Status != tableSessionOpen {
			return errors.New("本桌已有顾客发起结算，购物车已锁定，不能清空")
		}
		if err := tx.Where("session_id = ?", session.ID).Delete(&model.TableCartItem{}).Error; err != nil {
			return err
		}
		session.CartVersion++
		return tx.Model(&session).Update("cart_version", session.CartVersion).Error
	})
	if err != nil {
		return nil, err
	}
	return s.snapshotForSession(s.db, session)
}

func (s *TableOrderingService) Checkout(sessionID uint, tableCode string, req dto.TableCheckoutRequest) (*model.Order, bool, error) {
	createReq := dto.CreateCustomerOrderRequest{
		CustomerPhone:  req.CustomerPhone,
		CustomerNote:   req.CustomerNote,
		ShareCode:      req.ShareCode,
		CouponNo:       req.CouponNo,
		PaymentChannel: req.PaymentChannel,
		PayMode:        req.PayMode,
		RequestKey:     req.RequestKey,
		AccessToken:    req.AccessToken,
	}
	if err := validateCustomerOrderRequestCredentials(createReq.RequestKey, createReq.AccessToken); err != nil {
		return nil, false, err
	}
	var order model.Order
	created := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var session model.TableSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, sessionID).Error; err != nil {
			return errors.New("桌位会话不存在")
		}
		table, err := s.authorizeTableCode(tx, session.StoreID, session.TableNo, tableCode)
		if err != nil {
			return err
		}
		tableLabel := effectiveTableLabel(session.TableNo, table.TableLabel)
		if err := s.reopenTableSessionIfOrderEnded(tx, &session); err != nil {
			return err
		}
		if session.Status != tableSessionOpen {
			if session.CheckoutOrderID == nil {
				return errors.New("本桌结算状态异常，请联系门店处理")
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, *session.CheckoutOrderID).Error; err != nil {
				return errors.New("本桌已有结算订单，请联系门店处理")
			}
			if order.OrderType != "store_order" || order.StoreID == nil || *order.StoreID != session.StoreID {
				return errors.New("本桌结算订单与门店不匹配，请联系门店处理")
			}
			if order.OrderNo != customerOrderNoForRequestKey(req.RequestKey) || !customerOrderAccessTokenMatches(&order, req.AccessToken) {
				return errors.New("本桌已有人发起结算，请勿重复付款")
			}
			var detail model.TableOrderDetail
			if err := tx.Where("order_id = ? AND session_id = ? AND table_no = ?", order.ID, session.ID, session.TableNo).First(&detail).Error; err != nil {
				return errors.New("结算订单与桌位会话不匹配")
			}
			order.TableNo = detail.TableNo
			order.TableLabel = effectiveTableLabel(detail.TableNo, tableLabelSnapshotValue(detail.TableLabelSnapshot))
			return nil
		}
		var store model.Store
		if err := tx.Preload("Merchant").First(&store, session.StoreID).Error; err != nil || store.Status != "active" || !store.IsOpen || store.Merchant.Status != "active" {
			return errors.New("门店当前不接收新点单")
		}
		var cartItems []model.TableCartItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("session_id = ?", session.ID).Order("product_id ASC").Find(&cartItems).Error; err != nil {
			return err
		}
		if len(cartItems) == 0 {
			return errors.New("购物车为空，无法结算")
		}
		createReq.StoreID = session.StoreID
		createReq.Items = make([]dto.CustomerOrderItemRequest, 0, len(cartItems))
		for _, item := range cartItems {
			createReq.Items = append(createReq.Items, dto.CustomerOrderItemRequest{ProductID: item.ProductID, Quantity: item.Quantity})
		}
		createdOrder, err := s.createTableOrderInTransaction(tx, store, createReq, session)
		if err != nil {
			return err
		}
		order = *createdOrder
		created = true
		lockedAt := time.Now()
		result := tx.Model(&session).Updates(map[string]interface{}{
			"status":            tableSessionLocked,
			"checkout_order_id": order.ID,
			"locked_at":         lockedAt,
			"cart_version":      gorm.Expr("cart_version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("无法锁定桌位购物车")
		}
		labelSnapshot := tableLabel
		if err := tx.Create(&model.TableOrderDetail{OrderID: order.ID, SessionID: session.ID, TableNo: session.TableNo, TableLabelSnapshot: &labelSnapshot}).Error; err != nil {
			return err
		}
		order.TableNo = session.TableNo
		order.TableLabel = tableLabel
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &order, created, nil
}

func (s *TableOrderingService) createTableOrderInTransaction(tx *gorm.DB, store model.Store, req dto.CreateCustomerOrderRequest, session model.TableSession) (*model.Order, error) {
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
	var products []model.StoreProduct
	if err := tx.Where("store_id = ? AND status = ? AND id IN ?", store.ID, "active", productIDs).Find(&products).Error; err != nil {
		return nil, err
	}
	if len(products) != len(productIDs) {
		return nil, errors.New("部分商品已下架或不可用，请刷新购物车")
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
			return nil, errors.New("部分商品已下架或不可用，请刷新购物车")
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
	promotion, promotionDiscount := s.orders.bestPromotionForStore(store.MerchantID, store.ID, total)
	discount := promotionDiscount
	coupon, couponDiscount, err := s.orders.shares.ValidateCouponForOrder(store.ID, req.CustomerPhone, req.CouponNo, total-discount)
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
	if payable > 0 && store.OrderMode != "submit_later" {
		config, err := loadPublicMerchantPaymentConfig(tx, store.MerchantID)
		if err != nil {
			return nil, err
		}
		if config == nil {
			return nil, errors.New("门店尚未启用已审核的商家收款码，暂不能进行桌位线上结算")
		}
	}
	channel := "merchant_qr"
	status := "pending"
	if store.OrderMode == "submit_later" {
		status = "submitted"
		if store.AutoAccept {
			status = "preparing"
		}
	} else if payable == 0 {
		status = "submitted"
	}
	order := &model.Order{
		OrderNo:                 customerOrderNoForRequestKey(req.RequestKey),
		OrderType:               "store_order",
		MerchantID:              &store.MerchantID,
		StoreID:                 &store.ID,
		CustomerPhone:           normalizePhone(req.CustomerPhone),
		CustomerNote:            strings.TrimSpace(req.CustomerNote),
		Amount:                  total,
		TotalAmount:             payable,
		DiscountAmount:          discount,
		Items:                   string(itemsJSON),
		Status:                  status,
		PaymentChannel:          channel,
		Channel:                 model.OrderChannelTable,
		ShareCode:               strings.TrimSpace(req.ShareCode),
		CustomerAccessTokenHash: hashCustomerOrderAccessToken(req.AccessToken),
		CustomerAccessToken:     req.AccessToken,
		TableNo:                 session.TableNo,
	}
	if store.OrderMode == "submit_later" {
		order.OperationLogs = s.orders.appendOrderLog(order.OperationLogs, "submitted", "顾客已提交订单，稍后结算")
		if store.AutoAccept {
			order.OperationLogs = s.orders.appendOrderLog(order.OperationLogs, "auto_accepted", "门店已开启自动接单，订单进入处理中")
		}
	} else if payable == 0 {
		order.OperationLogs = s.orders.appendOrderLog(order.OperationLogs, "submitted", "本单无需付款，已发送给门店")
	}
	if coupon != nil {
		order.CouponID = &coupon.ID
		order.CouponNo = coupon.CouponNo
		order.OperationLogs = s.orders.appendOrderLog(order.OperationLogs, "coupon_locked", fmt.Sprintf("已锁定奖励券：%s，优惠 %s 元", coupon.Title, formatFen(couponDiscount)))
	}
	if promotion != nil {
		order.PromotionID = &promotion.ID
		order.OperationLogs = s.orders.appendOrderLog(order.OperationLogs, "promotion_applied", fmt.Sprintf("已自动匹配优惠：%s，优惠 %s 元", promotion.Title, formatFen(promotionDiscount)))
	}
	if err := tx.Create(order).Error; err != nil {
		return nil, err
	}
	if err := writeStoreOrderItems(tx, order.ID, snapshots); err != nil {
		return nil, err
	}
	order.Store = &store
	order.Merchant = &store.Merchant
	return order, nil
}

func (s *TableOrderingService) authorizeTableCode(tx *gorm.DB, storeID uint, tableNo int, code string) (*model.StoreTable, error) {
	var config model.StoreTableConfig
	if err := tx.Where("store_id = ?", storeID).First(&config).Error; err != nil || tableNo < 1 || tableNo > config.TableCount {
		return nil, errors.New("桌码无效或桌位已停用")
	}
	var table model.StoreTable
	if err := tx.Where("store_id = ? AND table_no = ?", storeID, tableNo).First(&table).Error; err != nil {
		return nil, errors.New("桌码无效或桌位已停用")
	}
	if len(code) != len(table.TableCode) || subtle.ConstantTimeCompare([]byte(code), []byte(table.TableCode)) != 1 {
		return nil, errors.New("桌码无效")
	}
	return &table, nil
}

func (s *TableOrderingService) reopenTableSessionIfOrderEnded(tx *gorm.DB, session *model.TableSession) error {
	if session.Status == tableSessionOpen {
		return nil
	}
	if session.CheckoutOrderID == nil {
		return errors.New("桌位会话结算状态异常，请联系门店处理")
	}
	var order model.Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, *session.CheckoutOrderID).Error; err != nil {
		return err
	}
	ended := order.Status == "closed" || order.Status == "completed"
	if !ended && isUnpaidCheckoutExpired(order, time.Now()) {
		order.Status = "closed"
		order.OperationLogs = s.orders.appendOrderLog(order.OperationLogs, "checkout_expired", "未付款订单超过30分钟有效期，系统已关闭")
		if err := tx.Model(&model.Order{}).Where("id = ? AND paid_at IS NULL AND status IN ?", order.ID, []string{"pending", "failed"}).Updates(map[string]interface{}{"status": order.Status, "operation_logs": order.OperationLogs}).Error; err != nil {
			return err
		}
		ended = true
	}
	if !ended {
		return nil
	}
	if err := tx.Where("session_id = ?", session.ID).Delete(&model.TableCartItem{}).Error; err != nil {
		return err
	}
	session.Status = tableSessionOpen
	session.CheckoutOrderID = nil
	session.LockedAt = nil
	session.CartVersion++
	return tx.Model(session).Updates(map[string]interface{}{
		"status":            tableSessionOpen,
		"checkout_order_id": nil,
		"locked_at":         nil,
		"cart_version":      session.CartVersion,
	}).Error
}

func (s *TableOrderingService) removeUnavailableCartItems(tx *gorm.DB, session *model.TableSession) error {
	var cartItems []model.TableCartItem
	if err := tx.Where("session_id = ?", session.ID).Find(&cartItems).Error; err != nil {
		return err
	}
	unavailableIDs := make([]uint, 0)
	for _, item := range cartItems {
		var product model.StoreProduct
		err := tx.Select("id").Where("id = ? AND store_id = ? AND status = ?", item.ProductID, session.StoreID, "active").First(&product).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			unavailableIDs = append(unavailableIDs, item.ProductID)
			continue
		}
		if err != nil {
			return err
		}
	}
	if len(unavailableIDs) == 0 {
		return nil
	}
	result := tx.Where("session_id = ? AND product_id IN ?", session.ID, unavailableIDs).Delete(&model.TableCartItem{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	session.CartVersion++
	return tx.Model(session).Update("cart_version", session.CartVersion).Error
}

func (s *TableOrderingService) snapshotForSession(db *gorm.DB, session model.TableSession) (*TableSessionSnapshot, error) {
	var table model.StoreTable
	label := defaultTableLabel(session.TableNo)
	err := db.Where("store_id = ? AND table_no = ?", session.StoreID, session.TableNo).First(&table).Error
	if err == nil {
		label = effectiveTableLabel(session.TableNo, table.TableLabel)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var cart []model.TableCartItem
	if err := db.Where("session_id = ?", session.ID).Order("product_id ASC").Find(&cart).Error; err != nil {
		return nil, err
	}
	lines := make([]TableCartLine, 0, len(cart))
	for _, row := range cart {
		var product model.StoreProduct
		if err := db.Where("id = ? AND store_id = ?", row.ProductID, session.StoreID).First(&product).Error; err != nil {
			continue
		}
		snapshot, lineAmount, err := buildStoreOrderItemSnapshot(product, row.Quantity)
		if err != nil {
			lines = append(lines, TableCartLine{ProductID: product.ID, Name: product.Name, Price: product.Price, Quantity: row.Quantity, LineAmount: product.Price * int64(row.Quantity), ImageURL: product.ImageURL, Description: product.Description})
			continue
		}
		lines = append(lines, TableCartLine{ProductID: product.ID, Name: snapshot.Name, Price: snapshot.Price, Quantity: row.Quantity, LineAmount: lineAmount, ImageURL: snapshot.ImageURL, Description: snapshot.Description, SalesUnit: snapshot.SalesUnit, Specification: snapshot.Specification, PriceTiers: snapshot.PriceTiers})
	}
	view := &TableSessionSnapshot{
		ID:          session.ID,
		StoreID:     session.StoreID,
		TableNo:     session.TableNo,
		TableLabel:  label,
		Status:      session.Status,
		Locked:      session.Status != tableSessionOpen,
		CartVersion: session.CartVersion,
		Items:       lines,
		UpdatedAt:   session.UpdatedAt,
	}
	if session.CheckoutOrderID != nil {
		var order model.Order
		if err := db.Select("id", "order_no").First(&order, *session.CheckoutOrderID).Error; err == nil {
			view.CheckoutOrderNo = order.OrderNo
		}
	}
	return view, nil
}

func (s *TableOrderingService) AttachTableNumber(order *model.Order) error {
	if order == nil || order.ID == 0 || !s.db.Migrator().HasTable(&model.TableOrderDetail{}) {
		return nil
	}
	var detail model.TableOrderDetail
	err := s.db.Where("order_id = ?", order.ID).First(&detail).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	order.TableNo = detail.TableNo
	order.TableLabel = effectiveTableLabel(detail.TableNo, tableLabelSnapshotValue(detail.TableLabelSnapshot))
	return nil
}

func (s *TableOrderingService) AttachTableNumbers(orders []model.Order) error {
	if len(orders) == 0 || !s.db.Migrator().HasTable(&model.TableOrderDetail{}) {
		return nil
	}
	ids := make([]uint, 0, len(orders))
	for i := range orders {
		ids = append(ids, orders[i].ID)
	}
	var details []model.TableOrderDetail
	if err := s.db.Where("order_id IN ?", ids).Find(&details).Error; err != nil {
		return err
	}
	byOrder := make(map[uint]model.TableOrderDetail, len(details))
	for _, detail := range details {
		byOrder[detail.OrderID] = detail
	}
	for i := range orders {
		if detail, ok := byOrder[orders[i].ID]; ok {
			orders[i].TableNo = detail.TableNo
			orders[i].TableLabel = effectiveTableLabel(detail.TableNo, tableLabelSnapshotValue(detail.TableLabelSnapshot))
		}
	}
	return nil
}

func tableLabelSnapshotValue(snapshot *string) string {
	if snapshot == nil {
		return ""
	}
	return *snapshot
}
