package dto

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PaginationQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}
