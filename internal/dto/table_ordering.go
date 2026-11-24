package dto

// ConfigureStoreTablesRequest changes active table QR codes and their display labels.
type ConfigureStoreTablesRequest struct {
	TableCount int                            `json:"table_count" binding:"gte=0,lte=300"`
	Tables     []ConfigureStoreTableLabelItem `json:"tables"`
}

type ConfigureStoreTableLabelItem struct {
	TableNo    int    `json:"table_no"`
	TableLabel string `json:"table_label"`
}

type JoinTableSessionRequest struct {
	StoreID   uint   `json:"store_id" binding:"required"`
	TableNo   int    `json:"table_no" binding:"required,gte=1,lte=300"`
	TableCode string `json:"table_code" binding:"required,min=32,max=64"`
}

type UpdateTableCartItemRequest struct {
	ProductID uint   `json:"product_id" binding:"required"`
	Action    string `json:"action" binding:"required,oneof=set add remove"`
	Quantity  int    `json:"quantity" binding:"gte=0,lte=99"`
}

type TableCheckoutRequest struct {
	RequestKey     string `json:"request_key" binding:"required,len=64,hexadecimal"`
	AccessToken    string `json:"access_token" binding:"required,len=64,hexadecimal"`
	CustomerPhone  string `json:"customer_phone"`
	CustomerNote   string `json:"customer_note"`
	ShareCode      string `json:"share_code"`
	CouponNo       string `json:"coupon_no"`
	PaymentChannel string `json:"payment_channel" binding:"omitempty,oneof=alipay wechat"`
	PayMode        string `json:"pay_mode"`
}
