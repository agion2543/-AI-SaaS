package dto

type MerchantRegisterRequest struct {
	Name            string `json:"name" binding:"required"`
	ContactPhone    string `json:"contact_phone" binding:"required,len=11,numeric"`
	SMSCode         string `json:"sms_code" binding:"required,len=6,numeric"`
	Password        string `json:"password" binding:"required,min=6"`
	ConfirmPassword string `json:"confirm_password" binding:"required,min=6"`
}

type MerchantLoginRequest struct {
	Phone    string `json:"phone" binding:"required,len=11,numeric"`
	Password string `json:"password" binding:"required"`
}

type MerchantSendPasswordResetCodeRequest struct {
	Phone string `json:"phone" binding:"required,len=11,numeric"`
}

type MerchantResetPasswordRequest struct {
	Phone           string `json:"phone" binding:"required,len=11,numeric"`
	SMSCode         string `json:"sms_code" binding:"required,len=6,numeric"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
	ConfirmPassword string `json:"confirm_password" binding:"required,min=6"`
}

type MerchantChangePasswordRequest struct {
	OldPassword     string `json:"old_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
	ConfirmPassword string `json:"confirm_password" binding:"required,min=6"`
}

type MerchantRedeemCardRequest struct {
	Code string `json:"code" binding:"required"`
}

type UpdateMerchantInfoRequest struct {
	Name         string `json:"name" binding:"required"`
	ContactPhone string `json:"contact_phone" binding:"required,len=11,numeric"`
	ContactEmail string `json:"contact_email" binding:"omitempty,email"`
}

type CreateStoreRequest struct {
	Name          string `json:"name" binding:"required"`
	Address       string `json:"address"`
	ContactPhone  string `json:"contact_phone" binding:"required,len=11,numeric"`
	IsOpen        *bool  `json:"is_open"`
	BusinessHours string `json:"business_hours"`
	PauseReason   string `json:"pause_reason"`
}

type UpdateStoreRequest struct {
	Name          string `json:"name" binding:"required"`
	Address       string `json:"address"`
	ContactPhone  string `json:"contact_phone" binding:"required,len=11,numeric"`
	Status        string `json:"status"`
	IsOpen        *bool  `json:"is_open"`
	BusinessHours string `json:"business_hours"`
	PauseReason   string `json:"pause_reason"`
}

type SaveStoreProductRequest struct {
	StoreID     uint   `json:"store_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Price       int64  `json:"price" binding:"gte=0"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	Category    string `json:"category"`
	Status      string `json:"status" binding:"omitempty,oneof=active inactive"`
	Sort        int    `json:"sort"`
}

type CreateCustomerLeadRequest struct {
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone" binding:"required,len=11,numeric"`
	Message       string `json:"message"`
}

type SavePromotionRequest struct {
	StoreID      *uint  `json:"store_id"`
	Title        string `json:"title" binding:"required"`
	Description  string `json:"description"`
	Type         string `json:"type" binding:"omitempty,oneof=amount discount"`
	Threshold    int64  `json:"threshold"`
	Discount     int64  `json:"discount"`
	DiscountRate int    `json:"discount_rate"`
	Status       string `json:"status" binding:"omitempty,oneof=draft published inactive"`
	ValidFrom    string `json:"valid_from"`
	ValidTo      string `json:"valid_to"`
}

type GeneratePromotionDraftRequest struct {
	CustomerTag string `json:"customer_tag"`
}

type GenerateAIMarketingCopyRequest struct {
	Scenario    string `json:"scenario"`
	CustomerTag string `json:"customer_tag"`
	Goal        string `json:"goal"`
	ProductName string `json:"product_name"`
}

type GenerateAIReferralCopyRequest struct {
	ProductName string `json:"product_name"`
	Tone        string `json:"tone"`
	Goal        string `json:"goal"`
}

type GenerateAIShareReviewRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type CreateMerchantSubscriptionOrderRequest struct {
	PlanID         uint   `json:"plan_id" binding:"required"`
	PaymentChannel string `json:"payment_channel"`
	PayMode        string `json:"pay_mode"`
}

type SaveMerchantPaymentConfigRequest struct {
	Channel      string `json:"channel" binding:"required,oneof=alipay wechat bank"`
	Mode         string `json:"mode" binding:"omitempty,oneof=direct platform service_provider"`
	AccountName  string `json:"account_name" binding:"required"`
	AccountNo    string `json:"account_no" binding:"required"`
	AppID        string `json:"app_id"`
	ContactPhone string `json:"contact_phone" binding:"omitempty,len=11,numeric"`
	Remark       string `json:"remark"`
}

type ReviewMerchantPaymentConfigRequest struct {
	Status      string `json:"status" binding:"omitempty,oneof=enabled disabled"`
	AuditStatus string `json:"audit_status" binding:"required,oneof=pending approved rejected"`
	AuditRemark string `json:"audit_remark"`
}
