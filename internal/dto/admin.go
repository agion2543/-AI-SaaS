package dto

type UpdateUserStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active banned"`
}

type ManualOpenMemberRequest struct {
	PackageID    uint   `json:"package_id"`
	MemberLevel  string `json:"member_level"`
	DurationDays int    `json:"duration_days"`
	QuotaDelta   int    `json:"quota_delta"`
	ExpiredAt    string `json:"expired_at"`
	AutoRenew    bool   `json:"auto_renew"`
}

type UpdateUserPhoneRequest struct {
	Phone string `json:"phone" binding:"required,len=11,numeric"`
}

type UpdateMerchantStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active suspended pending"`
}

type AdminSaveStoreRequest struct {
	Name          string `json:"name" binding:"required"`
	Address       string `json:"address"`
	ContactPhone  string `json:"contact_phone" binding:"required,len=11,numeric"`
	Status        string `json:"status" binding:"omitempty,oneof=active inactive"`
	IsOpen        *bool  `json:"is_open"`
	BusinessHours string `json:"business_hours"`
	PauseReason   string `json:"pause_reason"`
}

type OpenMerchantSubscriptionRequest struct {
	Plan         string `json:"plan" binding:"required,oneof=month year"`
	DurationDays int    `json:"duration_days"`
	Note         string `json:"note"`
}

type SaveMerchantPlanRequest struct {
	Name         string `json:"name" binding:"required"`
	PriceCents   int64  `json:"price_cents" binding:"required"`
	DurationDays int    `json:"duration_days" binding:"required"`
	Sort         int    `json:"sort"`
}

type UpdateCustomerLeadRequest struct {
	Status       string `json:"status" binding:"required,oneof=new contacted converted invalid"`
	FollowUpNote string `json:"follow_up_note"`
}

type SavePackageRequest struct {
	Name          string `json:"name" binding:"required"`
	Code          string `json:"code" binding:"required"`
	DurationDays  int    `json:"duration_days"`
	Price         int64  `json:"price" binding:"required"`
	OriginalPrice int64  `json:"original_price"`
	Quota         int    `json:"quota"`
	Sort          int    `json:"sort"`
	Status        string `json:"status" binding:"required,oneof=published hidden"`
	IsLifetime    bool   `json:"is_lifetime"`
	Description   string `json:"description"`
}

type BatchCardRequest struct {
	PackageID uint `json:"package_id" binding:"required"`
	Count     int  `json:"count" binding:"required,min=1,max=500"`
	Quota     int  `json:"quota"`
}

type SaveSystemConfigRequest struct {
	SiteName                 string `json:"site_name"`
	PaymentGateway           string `json:"payment_gateway"`
	PlatformAlipayQRCode     string `json:"platform_alipay_qr_code"`
	PlatformWechatQRCode     string `json:"platform_wechat_qr_code"`
	PlatformSubscriptionNote string `json:"platform_subscription_note"`
	FilingInfo               string `json:"filing_info"`
	Notice                   string `json:"notice"`
}

type ConfirmSubscriptionPaymentRequest struct {
	Remark string `json:"remark"`
}

type CreateMerchantSettlementRequest struct {
	Remark string `json:"remark"`
}

type MarkMerchantSettlementPaidRequest struct {
	Remark string `json:"remark"`
}

type CreateMerchantFollowUpRequest struct {
	Type         string `json:"type" binding:"omitempty,oneof=risk settlement refund payment subscription operation"`
	Priority     string `json:"priority" binding:"omitempty,oneof=low normal high urgent"`
	Content      string `json:"content" binding:"required"`
	Source       string `json:"source"`
	SourceID     uint   `json:"source_id"`
	OrderID      uint   `json:"order_id"`
	OrderNo      string `json:"order_no"`
	NextFollowAt string `json:"next_follow_at"`
}

type UpdateMerchantFollowUpStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=open closed"`
}
