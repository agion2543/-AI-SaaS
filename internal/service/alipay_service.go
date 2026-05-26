package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"time"

	"github.com/smartwalle/alipay/v3"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

type AlipayService struct {
	cfg    *config.Config
	client *alipay.Client
}

type AlipayCheckoutPayload struct {
	Channel     string `json:"channel"`
	Mode        string `json:"mode"`
	PaymentURL  string `json:"payment_url,omitempty"`
	PaymentForm string `json:"payment_form,omitempty"`
	QRCode      string `json:"qr_code,omitempty"`
	NotifyURL   string `json:"notify_url"`
	ReturnURL   string `json:"return_url,omitempty"`
}

func NewAlipayService(cfg *config.Config) (*AlipayService, error) {
	service := &AlipayService{cfg: cfg}
	if cfg.AlipayAppID == "" || cfg.AlipayPrivateKey == "" || cfg.AlipayPublicKey == "" {
		return service, nil
	}

	client, err := alipay.New(cfg.AlipayAppID, cfg.AlipayPrivateKey, !cfg.AlipaySandbox)
	if err != nil {
		return nil, err
	}
	if err := client.LoadAliPayPublicKey(cfg.AlipayPublicKey); err != nil {
		return nil, err
	}

	service.client = client
	return service, nil
}

func (s *AlipayService) Enabled() bool {
	return s.client != nil
}

func (s *AlipayService) BuildCheckoutPayload(order *model.Order, payMode string) (*AlipayCheckoutPayload, error) {
	if !s.Enabled() {
		return nil, errors.New("alipay sandbox is not configured")
	}

	if payMode == "" {
		payMode = "page"
	}

	switch payMode {
	case "page":
		return s.buildPagePayload(order)
	case "qr":
		return s.buildQRPayload(order)
	default:
		return nil, fmt.Errorf("unsupported pay mode: %s", payMode)
	}
}

func (s *AlipayService) VerifyNotification(values url.Values) error {
	if !s.Enabled() {
		return errors.New("alipay sandbox is not configured")
	}
	return s.client.VerifySign(context.Background(), values)
}

func (s *AlipayService) Refund(order *model.Order, refundNo string, amount int64, reason string) (string, error) {
	if !s.Enabled() {
		return "", errors.New("alipay sandbox is not configured")
	}
	if order.TransactionNo == "" && order.OrderNo == "" {
		return "", errors.New("alipay transaction number is missing")
	}

	param := alipay.TradeRefund{
		OutTradeNo:   order.OrderNo,
		TradeNo:      order.TransactionNo,
		RefundAmount: utils.FenToYuan(amount),
		RefundReason: reason,
		OutRequestNo: refundNo,
		QueryOptions: []string{"refund_detail_item_list"},
		AppAuthToken: "",
	}
	result, err := s.client.TradeRefund(context.Background(), param)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", errors.New("alipay refund returned empty response")
	}
	if result.IsFailure() {
		return "", fmt.Errorf("alipay refund failed: %s %s", result.Code, result.Msg)
	}
	raw, _ := json.Marshal(result)
	return string(raw), nil
}

func (s *AlipayService) buildPagePayload(order *model.Order) (*AlipayCheckoutPayload, error) {
	returnURL := s.returnURL(order)
	param := alipay.TradePagePay{
		Trade: alipay.Trade{
			NotifyURL:      s.cfg.AlipayNotifyURL,
			ReturnURL:      returnURL,
			Subject:        s.subject(order),
			OutTradeNo:     order.OrderNo,
			TotalAmount:    utils.FenToYuan(payableAmount(order)),
			ProductCode:    "FAST_INSTANT_TRADE_PAY",
			Body:           s.body(order),
			TimeoutExpress: "30m",
			PassbackParams: url.QueryEscape(order.OrderNo),
		},
	}

	payURL, err := s.client.TradePagePay(param)
	if err != nil {
		return nil, err
	}

	paymentURL := payURL.String()
	return &AlipayCheckoutPayload{
		Channel:     "alipay",
		Mode:        "page",
		PaymentURL:  paymentURL,
		PaymentForm: fmt.Sprintf(`<form id="alipay-page-pay" action="%s" method="get"></form><script>document.getElementById('alipay-page-pay').submit();</script>`, html.EscapeString(paymentURL)),
		NotifyURL:   s.cfg.AlipayNotifyURL,
		ReturnURL:   returnURL,
	}, nil
}

func (s *AlipayService) buildQRPayload(order *model.Order) (*AlipayCheckoutPayload, error) {
	param := alipay.TradePreCreate{
		Trade: alipay.Trade{
			NotifyURL:      s.cfg.AlipayNotifyURL,
			Subject:        s.subject(order),
			OutTradeNo:     order.OrderNo,
			TotalAmount:    utils.FenToYuan(payableAmount(order)),
			ProductCode:    "FACE_TO_FACE_PAYMENT",
			Body:           s.body(order),
			TimeoutExpress: "30m",
			PassbackParams: url.QueryEscape(order.OrderNo),
		},
	}

	result, err := s.client.TradePreCreate(context.Background(), param)
	if err != nil {
		return nil, err
	}
	if result == nil || result.IsFailure() || result.QRCode == "" {
		return nil, fmt.Errorf("alipay precreate failed: %v", result)
	}

	return &AlipayCheckoutPayload{
		Channel:   "alipay",
		Mode:      "qr",
		QRCode:    result.QRCode,
		NotifyURL: s.cfg.AlipayNotifyURL,
		ReturnURL: s.returnURL(order),
	}, nil
}

func payableAmount(order *model.Order) int64 {
	if order.TotalAmount > 0 {
		return order.TotalAmount
	}
	return order.Amount
}

func (s *AlipayService) subject(order *model.Order) string {
	if order.OrderType == "store_order" {
		storeName := "门店"
		if order.Store != nil && order.Store.Name != "" {
			storeName = order.Store.Name
		}
		return fmt.Sprintf("%s点单 - %s", storeName, order.OrderNo)
	}
	if order.OrderType == "merchant_subscription" && order.MerchantPlan != nil && order.MerchantPlan.Name != "" {
		return fmt.Sprintf("商家订阅%s - %s", order.MerchantPlan.Name, order.OrderNo)
	}
	if order.Package != nil && order.Package.Name != "" {
		return fmt.Sprintf("%s - %s", order.Package.Name, order.OrderNo)
	}
	return "SaaS Order - " + order.OrderNo
}

func (s *AlipayService) body(order *model.Order) string {
	if order.OrderType == "store_order" {
		return "顾客扫码下单，等待商家接单处理"
	}
	if order.OrderType == "merchant_subscription" && order.MerchantPlan != nil {
		return fmt.Sprintf("商家工作台订阅服务，周期 %d 天", order.MerchantPlan.DurationDays)
	}
	if order.Package != nil {
		return order.Package.Description
	}
	return "SaaS payment"
}

func (s *AlipayService) returnURL(order *model.Order) string {
	if order.OrderType == "store_order" {
		return fmt.Sprintf("%s/customer/orders/%s?from=pay_return", s.cfg.FrontendURL, url.PathEscape(order.OrderNo))
	}
	return s.cfg.AlipayReturnURL
}

func IsAlipaySuccessStatus(status string) bool {
	return status == string(alipay.TradeStatusSuccess) || status == string(alipay.TradeStatusFinished)
}

func IsAlipayClosedStatus(status string) bool {
	return status == string(alipay.TradeStatusClosed)
}

func BuildTransactionNo() string {
	return fmt.Sprintf("ALI%d", time.Now().UnixNano())
}
