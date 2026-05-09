package dto

type CreateOrderRequest struct {
	PackageID      uint   `json:"package_id" binding:"required"`
	PaymentChannel string `json:"payment_channel" binding:"required"`
	PayMode        string `json:"pay_mode,omitempty"`
	AutoRenew      bool   `json:"auto_renew"`
}

type CustomerOrderItemRequest struct {
	ProductID uint  `json:"product_id" binding:"required"`
	Quantity  int   `json:"quantity" binding:"required,min=1"`
	Price     int64 `json:"price"`
}

type CreateCustomerOrderRequest struct {
	StoreID        uint                       `json:"store_id" binding:"required"`
	CustomerPhone  string                     `json:"customer_phone"`
	CustomerNote   string                     `json:"customer_note"`
	PaymentChannel string                     `json:"payment_channel"`
	PayMode        string                     `json:"pay_mode"`
	Items          []CustomerOrderItemRequest `json:"items" binding:"required"`
}

type MerchantOrderFilterRequest struct {
	Status   string `form:"status"`
	StoreID  uint   `form:"store_id"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type UpdateMerchantOrderNoteRequest struct {
	MerchantNote string `json:"merchant_note"`
}

type RefundOrderRequest struct {
	Amount int64  `json:"amount"`
	Full   bool   `json:"full"`
	Reason string `json:"reason"`
}

type PaymentCallbackRequest struct {
	OrderNo        string      `json:"order_no" binding:"required"`
	TransactionNo  string      `json:"transaction_no" binding:"required"`
	PaymentChannel string      `json:"payment_channel" binding:"required"`
	Amount         int64       `json:"amount" binding:"required"`
	Status         string      `json:"status" binding:"required,oneof=success failed"`
	RawPayload     interface{} `json:"raw_payload"`
}

type RedeemCardRequest struct {
	Code string `json:"code" binding:"required"`
}

type BindDeviceRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
}
