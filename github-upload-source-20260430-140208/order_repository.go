package repository

import (
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(order *model.Order) error {
	return r.db.Create(order).Error
}

func (r *OrderRepository) FindByOrderNo(orderNo string) (*model.Order, error) {
	var order model.Order
	err := r.db.Preload("Package").Preload("User").Preload("Merchant").Preload("MerchantPlan").Preload("Store").Preload("Promotion").Where("order_no = ?", orderNo).First(&order).Error
	return &order, err
}

func (r *OrderRepository) Save(order *model.Order) error {
	return r.db.Save(order).Error
}

func (r *OrderRepository) ListByUser(userID uint) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.Preload("Package").Preload("Store").Where("user_id = ?", userID).Order("id desc").Find(&orders).Error
	return orders, err
}

func (r *OrderRepository) ListAll() ([]model.Order, error) {
	var orders []model.Order
	err := r.db.Preload("User").
		Preload("Package").
		Preload("Store").
		Preload("Merchant").
		Preload("MerchantPlan").
		Preload("Promotion").
		Order("id desc").
		Find(&orders).Error
	return orders, err
}

func (r *OrderRepository) ListByMerchant(merchantID uint, status string, storeID *uint, keyword string, offset, limit int) ([]model.Order, int64, error) {
	var orders []model.Order
	var total int64

	query := r.db.Model(&model.Order{}).Where("merchant_id = ? AND order_type = ?", merchantID, "store_order")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if storeID != nil {
		query = query.Where("store_id = ?", *storeID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("order_no LIKE ? OR customer_phone LIKE ? OR items LIKE ?", like, like, like)
	}

	query.Count(&total)
	err := query.Preload("Store").Preload("Promotion").Order("id desc").Offset(offset).Limit(limit).Find(&orders).Error
	return orders, total, err
}

func (r *OrderRepository) FindByMerchantAndID(merchantID, orderID uint) (*model.Order, error) {
	var order model.Order
	err := r.db.Preload("Store").Preload("Promotion").Where("merchant_id = ? AND id = ?", merchantID, orderID).First(&order).Error
	return &order, err
}

func (r *OrderRepository) CreatePayment(record *model.PaymentRecord) error {
	return r.db.Create(record).Error
}

func (r *OrderRepository) FindPaymentByTransactionNo(transactionNo string) (*model.PaymentRecord, error) {
	var record model.PaymentRecord
	err := r.db.Where("transaction_no = ?", transactionNo).First(&record).Error
	return &record, err
}

func (r *OrderRepository) ListPayments() ([]model.PaymentRecord, error) {
	var list []model.PaymentRecord
	err := r.db.Preload("Order").
		Preload("Order.Merchant").
		Preload("Order.Store").
		Preload("Order.User").
		Order("id desc").
		Find(&list).Error
	return list, err
}

func (r *OrderRepository) ListRefunds() ([]model.RefundRecord, error) {
	var list []model.RefundRecord
	err := r.db.Preload("Order").Preload("Order.Merchant").Preload("Order.Store").Order("id desc").Find(&list).Error
	return list, err
}

func (r *OrderRepository) ListRefundsByMerchant(merchantID uint) ([]model.RefundRecord, error) {
	var list []model.RefundRecord
	err := r.db.Preload("Order").Preload("Order.Store").
		Where("merchant_id = ?", merchantID).
		Order("id desc").
		Find(&list).Error
	return list, err
}
