package dto

type RegisterRequest struct {
	Phone       string `json:"phone" binding:"required,len=11,numeric"`
	SMSCode     string `json:"sms_code" binding:"required,len=6"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name" binding:"required"`
}

type LoginRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ForgotPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type SendSMSCodeRequest struct {
	Phone string `json:"phone" binding:"required,min=11,max=11,numeric"`
}

type SendLoginCodeRequest struct {
	Account string `json:"account" binding:"required"`
}

type PhoneLoginRequest struct {
	Phone string `json:"phone" binding:"required,len=11,numeric"`
	Code  string `json:"code" binding:"required,len=6"`
}

type ResetPasswordByPhoneRequest struct {
	Phone       string `json:"phone" binding:"required,min=11,max=11,numeric"`
	Code        string `json:"code" binding:"required,len=6"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type DevResetPasswordRequest struct {
	AccountType string `json:"account_type" binding:"required,oneof=user admin"`
	Account     string `json:"account" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type BindPhoneRequest struct {
	Phone string `json:"phone" binding:"required,min=11,max=11,numeric"`
	Code  string `json:"code" binding:"required,len=6"`
}

type SendEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type BindEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
