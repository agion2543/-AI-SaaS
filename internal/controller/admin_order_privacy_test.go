package controller

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

const (
	syntheticCustomerPhone = "13800138000"
	syntheticUserPhone     = "13600136000"
)

func TestSerializeAdminOrderUsesMinimalMaskedContactByDefault(t *testing.T) {
	userID := uint(7)
	order := &model.Order{
		BaseModel:      model.BaseModel{ID: 9},
		OrderNo:        "SYNTHETIC-ORDER-PRIVACY-001",
		OrderType:      "store_order",
		UserID:         &userID,
		User:           &model.User{BaseModel: model.BaseModel{ID: userID}, DisplayName: "合成顾客", Phone: syntheticUserPhone, Email: "private@example.invalid", BoundDevices: "synthetic-device-private"},
		CustomerPhone:  syntheticCustomerPhone,
		CustomerNote:   "private customer note with contact",
		MerchantNote:   "private merchant note",
		CustomerReview: "private review",
		OperationLogs:  `[{"text":"private operation log"}]`,
		Store:          &model.Store{BaseModel: model.BaseModel{ID: 3}, Name: "合成门店", Address: "private store address", ContactPhone: "13100002222"},
		Merchant:       &model.Merchant{BaseModel: model.BaseModel{ID: 4}, Name: "合成商家", ContactPhone: "13200003333", ContactEmail: "merchant-private@example.invalid"},
	}

	maskedJSON, err := json.Marshal(serializeAdminOrder(order, false))
	if err != nil {
		t.Fatalf("marshal masked admin order: %v", err)
	}
	masked := string(maskedJSON)
	for _, secret := range []string{syntheticCustomerPhone, syntheticUserPhone, "private@example.invalid", "synthetic-device-private", "private customer note", "private merchant note", "private review", "private operation log", "private store address", "13100002222", "13200003333", "merchant-private@example.invalid"} {
		if strings.Contains(masked, secret) {
			t.Errorf("default admin order response leaked %q: %s", secret, masked)
		}
	}
	if !strings.Contains(masked, "138****8000") || !strings.Contains(masked, "136****6000") || !strings.Contains(masked, "合成顾客") || !strings.Contains(masked, "合成门店") {
		t.Fatalf("masked response omitted useful masked fields: %s", masked)
	}

	fullJSON, err := json.Marshal(serializeAdminOrder(order, true))
	if err != nil {
		t.Fatalf("marshal authorized admin order: %v", err)
	}
	full := string(fullJSON)
	if !strings.Contains(full, syntheticCustomerPhone) || !strings.Contains(full, syntheticUserPhone) || !strings.Contains(full, "private@example.invalid") {
		t.Fatalf("authorized response should include contact fields: %s", full)
	}
	for _, secret := range []string{"private customer note", "private store address", "13200003333"} {
		if strings.Contains(full, secret) {
			t.Errorf("authorized contact response included unrelated field %q: %s", secret, full)
		}
	}
}

func TestAdminCustomerLeadAndUserSerializersMaskSensitiveFields(t *testing.T) {
	lead := model.CustomerLead{
		BaseModel: model.BaseModel{ID: 2}, MerchantID: 3, StoreID: 4,
		CustomerName: "王小明", CustomerPhone: syntheticCustomerPhone,
		Message: "please call private number 15500001111", FollowUpNote: "private follow up note",
		Store: model.Store{BaseModel: model.BaseModel{ID: 4}, Name: "合成门店", Address: "private address", ContactPhone: "13100002222"},
	}
	encoded, err := json.Marshal(serializeAdminCustomerLead(lead, false))
	if err != nil {
		t.Fatal(err)
	}
	masked := string(encoded)
	for _, secret := range []string{syntheticCustomerPhone, "please call private number", "private follow up note", "private address", "13100002222"} {
		if strings.Contains(masked, secret) {
			t.Errorf("default lead response leaked %q: %s", secret, masked)
		}
	}
	if !strings.Contains(masked, "王*") || !strings.Contains(masked, "138****8000") {
		t.Errorf("default lead response should mask name and phone: %s", masked)
	}

	user := model.User{BaseModel: model.BaseModel{ID: 8}, Phone: syntheticUserPhone, Email: "private@example.invalid", BoundDevices: "private-device"}
	userJSON, err := json.Marshal(serializeAdminUser(user, false))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{syntheticUserPhone, "private@example.invalid", "private-device"} {
		if strings.Contains(string(userJSON), secret) {
			t.Errorf("default admin user response leaked %q: %s", secret, userJSON)
		}
	}
}

func TestAdminOrderContactViewAndExportAreAuditedAndFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, ctl := newSyntheticAdminOrderController(t, true)
	order := model.Order{
		OrderNo: "SYNTHETIC-ORDER-PRIVACY-002", OrderType: "store_order",
		CustomerPhone: syntheticCustomerPhone, CustomerNote: "private synthetic note",
		Amount: 2500, TotalAmount: 2500, Status: "completed", PaymentChannel: "alipay",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create synthetic order: %v", err)
	}

	masked := serveSyntheticAdminOrderRequest(t, ctl, "/admin/orders", "admin", "finance")
	if masked.Code != http.StatusOK || strings.Contains(masked.Body.String(), syntheticCustomerPhone) || !strings.Contains(masked.Body.String(), "138****8000") {
		t.Fatalf("default order list should return masked contact; status=%d body=%s", masked.Code, masked.Body.String())
	}
	denied := serveSyntheticAdminOrderRequest(t, ctl, "/admin/orders?include_contact=true", "admin", "finance")
	if denied.Code != http.StatusForbidden || strings.Contains(denied.Body.String(), syntheticCustomerPhone) {
		t.Fatalf("unauthorized full-contact request must be denied without PII: status=%d body=%s", denied.Code, denied.Body.String())
	}
	full := serveSyntheticAdminOrderRequest(t, ctl, "/admin/orders?include_contact=true", "admin", "finance,customer_contact_view")
	if full.Code != http.StatusOK || !strings.Contains(full.Body.String(), syntheticCustomerPhone) {
		t.Fatalf("authorized order contact request should return full number: status=%d body=%s", full.Code, full.Body.String())
	}
	assertSyntheticAuditCount(t, db, "admin_customer_contact_view", 1)

	maskedExport := serveSyntheticAdminOrderRequest(t, ctl, "/admin/orders/export", "admin", "finance,customer_order_export")
	if maskedExport.Code != http.StatusOK {
		t.Fatalf("masked order export status=%d body=%s", maskedExport.Code, maskedExport.Body.String())
	}
	maskedSheet := readSyntheticWorkbookSheet(t, maskedExport.Body.Bytes())
	if !strings.Contains(maskedSheet, "138****8000") || strings.Contains(maskedSheet, syntheticCustomerPhone) {
		t.Fatalf("default XLSX export should contain only masked contact: %s", maskedSheet)
	}

	deniedExport := serveSyntheticAdminOrderRequest(t, ctl, "/admin/orders/export?include_contact=true", "admin", "finance,customer_order_export")
	if deniedExport.Code != http.StatusForbidden || strings.Contains(deniedExport.Body.String(), syntheticCustomerPhone) {
		t.Fatalf("export without contact-view permission must be denied: status=%d body=%s", deniedExport.Code, deniedExport.Body.String())
	}
	fullExport := serveSyntheticAdminOrderRequest(t, ctl, "/admin/orders/export?include_contact=true", "admin", "finance,customer_order_export,customer_contact_view")
	if fullExport.Code != http.StatusOK {
		t.Fatalf("authorized full-contact export status=%d body=%s", fullExport.Code, fullExport.Body.String())
	}
	fullSheet := readSyntheticWorkbookSheet(t, fullExport.Body.Bytes())
	if !strings.Contains(fullSheet, syntheticCustomerPhone) {
		t.Fatalf("authorized full-contact export should include phone: %s", fullSheet)
	}
	assertSyntheticAuditCount(t, db, "admin_customer_order_export", 2)

	nilAuditController := NewOrderController(service.NewOrderService(db), nil, nil)
	failedView := serveSyntheticAdminOrderRequest(t, nilAuditController, "/admin/orders?include_contact=true", "super_admin", "")
	if failedView.Code != http.StatusInternalServerError || strings.Contains(failedView.Body.String(), syntheticCustomerPhone) {
		t.Fatalf("audit failure must fail closed for contact view: status=%d body=%s", failedView.Code, failedView.Body.String())
	}
	failedExport := serveSyntheticAdminOrderRequest(t, nilAuditController, "/admin/orders/export", "super_admin", "")
	if failedExport.Code != http.StatusInternalServerError || strings.Contains(failedExport.Body.String(), syntheticCustomerPhone) {
		t.Fatalf("audit failure must fail closed for export: status=%d body=%s", failedExport.Code, failedExport.Body.String())
	}
	if err := db.Migrator().DropTable(&model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/admin/orders?include_contact=true", "/admin/orders/export?include_contact=true"} {
		response := serveSyntheticAdminOrderRequest(t, ctl, path, "super_admin", "")
		if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), syntheticCustomerPhone) || strings.Contains(response.Header().Get("Content-Type"), "spreadsheet") {
			t.Fatalf("audit insert failure did not fail closed: status=%d body=%s", response.Code, response.Body.String())
		}
	}
}

func newSyntheticAdminOrderController(t *testing.T, withAudit bool) (*gorm.DB, *OrderController) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQL database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.User{}, &model.Merchant{}, &model.Store{}, &model.MembershipPackage{},
		&model.MerchantPlan{}, &model.MerchantSettlement{}, &model.Promotion{},
		&model.Order{}, &model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate synthetic SQLite models: %v", err)
	}
	var audit *service.AuditService
	if withAudit {
		audit = service.NewAuditService(db)
	}
	orderService := service.NewOrderService(db)
	return db, NewOrderController(orderService, nil, audit)
}

func serveSyntheticAdminOrderRequest(t *testing.T, ctl *OrderController, target, role, permissions string) *httptest.ResponseRecorder {
	t.Helper()
	engine := gin.New()
	contextMiddleware := func(c *gin.Context) {
		c.Set("role", role)
		c.Set("user_id", uint(77))
		c.Set("admin_permissions", permissions)
		c.Next()
	}
	engine.GET("/admin/orders", contextMiddleware, ctl.ListAll)
	engine.GET("/admin/orders/export", contextMiddleware, ctl.ExportAdminOrders)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	return response
}

func readSyntheticWorkbookSheet(t *testing.T, content []byte) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open generated synthetic workbook: %v", err)
	}
	for _, file := range reader.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			t.Fatalf("open generated workbook sheet: %v", err)
		}
		data, err := io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			t.Fatalf("read generated workbook sheet: %v", err)
		}
		return string(data)
	}
	t.Fatal("generated workbook has no worksheet")
	return ""
}

func assertSyntheticAuditCount(t *testing.T, db *gorm.DB, action string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.AuditLog{}).Where("action = ?", action).Count(&count).Error; err != nil {
		t.Fatalf("count synthetic audits: %v", err)
	}
	if count != want {
		t.Fatalf("audit action %q count=%d, want %d", action, count, want)
	}
}

func TestParseAdminOrderExportFiltersRejectsInvalidInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, target := range []string{
		"/admin/orders/export?date_from=not-a-date",
		"/admin/orders/export?merchant_ids=1,invalid",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = request
		if _, ok := parseAdminOrderExportFilters(ctx); ok {
			t.Errorf("parseAdminOrderExportFilters accepted invalid query %q", target)
		}
	}
}
