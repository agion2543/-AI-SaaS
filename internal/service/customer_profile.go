package service

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/model"
)

// Customer-profile materialization (v16).
//
// The old ListCustomerProfiles / ListMerchantCustomerProfiles aggregated the
// orders table on every read (GROUP BY + GROUP_CONCAT, then Go-side filter /
// sort / pagination). The facts now live in customer_profiles and are kept
// current by an O(1) delta at the moment an order becomes a valid trade, so
// reads never scan orders. Only facts are stored — no AITag, no refund
// amounts; buildCustomerInsight still derives the label at read time.
//
// Callers must invoke the upsert exactly on the PaidAt nil -> set transition
// (HandlePaymentCallback normal + duplicate/late branches,
// ConfirmMerchantOrderPayment). The upsert itself is idempotent for the
// latest order (LastOrderID match) but has no per-order ledger, so a second
// call for a non-latest order would double-count; the PaidAt guards at the
// call sites are what make that unreachable. Every helper takes the tx
// handle, never s.db: under the single-connection SQLite test fixtures a
// second handle inside a transaction deadlocks (same reason as
// refundReviewThresholdCentsTx).

// upsertCustomerProfileForPaidOrderTx folds one newly-paid store order into
// its (merchant_id, customer_phone) profile row. Non store orders, empty
// phones, unpaid orders and non-positive amounts are not profile events and
// are silently ignored, matching the old GROUP BY filter
// (order_type='store_order' AND customer_phone<>'' AND paid_at IS NOT NULL
// AND total_amount > 0). Amounts are integer fen; refunds never touch the
// totals, exactly as the old gross-amount profiles did.
func upsertCustomerProfileForPaidOrderTx(tx *gorm.DB, order model.Order) error {
	if tx == nil {
		return errors.New("customer profile upsert requires a database handle")
	}
	if order.OrderType != "store_order" || order.MerchantID == nil || order.PaidAt == nil || order.TotalAmount <= 0 {
		return nil
	}
	phone := strings.TrimSpace(order.CustomerPhone)
	if phone == "" {
		return nil
	}
	storeName := customerProfileStoreNameTx(tx, order)

	var profile model.CustomerProfile
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("merchant_id = ? AND customer_phone = ?", *order.MerchantID, phone).
		First(&profile).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		createdAt := order.CreatedAt
		profile = model.CustomerProfile{
			MerchantID:          *order.MerchantID,
			CustomerPhone:       phone,
			LastStoreID:         order.StoreID,
			LastStoreName:       storeName,
			OrderCount:          1,
			TotalAmountCents:    order.TotalAmount,
			DiscountAmountCents: order.DiscountAmount,
			FirstPaidAt:         order.PaidAt,
			LastPaidAt:          order.PaidAt,
			LastOrderAt:         &createdAt,
			LastOrderID:         order.ID,
			LastStatus:          order.Status,
		}
		return tx.Create(&profile).Error
	}
	// The order was already folded in (its id is the recorded latest one):
	// a repeated payment callback for the same order must not count twice.
	if profile.LastOrderID == order.ID {
		return nil
	}
	profile.OrderCount++
	profile.TotalAmountCents += order.TotalAmount
	profile.DiscountAmountCents += order.DiscountAmount
	if profile.FirstPaidAt == nil || order.PaidAt.Before(*profile.FirstPaidAt) {
		profile.FirstPaidAt = order.PaidAt
	}
	if profile.LastPaidAt == nil || order.PaidAt.After(*profile.LastPaidAt) {
		profile.LastPaidAt = order.PaidAt
	}
	// last_* follows the latest order by (created_at, id): an older order
	// paying late still counts toward the totals, but must not move the
	// latest-order facts backwards.
	if customerProfileOrderIsLater(order, profile) {
		createdAt := order.CreatedAt
		profile.LastOrderAt = &createdAt
		profile.LastOrderID = order.ID
		profile.LastStatus = order.Status
		profile.LastStoreID = order.StoreID
		profile.LastStoreName = storeName
	}
	return tx.Save(&profile).Error
}

// updateCustomerProfileLastStatusTx refreshes last_status after a merchant
// accepts / completes / closes an order, but only when that order is the
// customer's latest one (LastOrderID match). A status change on an older
// order must not rewrite what the latest order says about the customer.
func updateCustomerProfileLastStatusTx(tx *gorm.DB, order model.Order) error {
	if tx == nil {
		return errors.New("customer profile status update requires a database handle")
	}
	if order.OrderType != "store_order" || order.MerchantID == nil {
		return nil
	}
	phone := strings.TrimSpace(order.CustomerPhone)
	if phone == "" {
		return nil
	}
	var profile model.CustomerProfile
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("merchant_id = ? AND customer_phone = ?", *order.MerchantID, phone).
		First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if profile.LastOrderID != order.ID || profile.LastStatus == order.Status {
		return nil
	}
	profile.LastStatus = order.Status
	return tx.Save(&profile).Error
}

func customerProfileOrderIsLater(order model.Order, profile model.CustomerProfile) bool {
	if profile.LastOrderAt == nil {
		return true
	}
	if order.CreatedAt.After(*profile.LastOrderAt) {
		return true
	}
	return order.CreatedAt.Equal(*profile.LastOrderAt) && order.ID > profile.LastOrderID
}

func customerProfileStoreNameTx(tx *gorm.DB, order model.Order) string {
	if order.Store != nil && order.Store.Name != "" {
		if order.StoreID == nil || order.Store.ID == *order.StoreID {
			return order.Store.Name
		}
	}
	if order.StoreID == nil {
		return ""
	}
	var store model.Store
	if err := tx.Select("id", "name").First(&store, *order.StoreID).Error; err != nil {
		return ""
	}
	return store.Name
}

// --- read side ---

// customerProfileListRow is one materialized profile row joined with its
// merchant name, in the same shape CustomerProfileRow carries to callers.
type customerProfileListRow struct {
	CustomerPhone  string    `gorm:"column:customer_phone"`
	MerchantID     uint      `gorm:"column:merchant_id"`
	MerchantName   string    `gorm:"column:merchant_name"`
	StoreName      string    `gorm:"column:store_name"`
	OrderCount     int64     `gorm:"column:order_count"`
	TotalAmount    int64     `gorm:"column:total_amount"`
	DiscountAmount int64     `gorm:"column:discount_amount"`
	LastOrderAt    time.Time `gorm:"column:last_order_at"`
	LastStatus     string    `gorm:"column:last_status"`
}

func customerProfileBaseQuery(db *gorm.DB, keyword string) *gorm.DB {
	query := db.Table("customer_profiles").
		Joins("LEFT JOIN merchants ON merchants.id = customer_profiles.merchant_id")
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("customer_profiles.customer_phone LIKE ? OR merchants.name LIKE ? OR customer_profiles.last_store_name LIKE ?", like, like, like)
	}
	return query
}

func customerProfileListSelect(query *gorm.DB) *gorm.DB {
	// No GROUP BY: the merchants join is 1:1 on the primary key, so each
	// profile row joins at most one merchant name.
	return query.Select(`customer_profiles.customer_phone AS customer_phone,
		customer_profiles.merchant_id AS merchant_id,
		COALESCE(merchants.name, '') AS merchant_name,
		customer_profiles.last_store_name AS store_name,
		customer_profiles.order_count AS order_count,
		customer_profiles.total_amount_cents AS total_amount,
		customer_profiles.discount_amount_cents AS discount_amount,
		customer_profiles.last_order_at AS last_order_at,
		customer_profiles.last_status AS last_status`)
}

// customerProfileSortClause maps the caller-facing sort key to the
// materialized columns. Unknown keys fall back to last_order_at, as the old
// Go sort did. The trailing last_order_at / customer_phone keys keep the
// order deterministic when the primary key ties.
func customerProfileSortClause(sortBy, sortOrder string) string {
	column := "customer_profiles.last_order_at"
	switch strings.TrimSpace(sortBy) {
	case "order_count":
		column = "customer_profiles.order_count"
	case "total_amount":
		column = "customer_profiles.total_amount_cents"
	case "discount_amount":
		column = "customer_profiles.discount_amount_cents"
	case "customer_phone":
		column = "customer_profiles.customer_phone"
	}
	direction := "DESC"
	if strings.ToLower(strings.TrimSpace(sortOrder)) == "asc" {
		direction = "ASC"
	}
	if column == "customer_profiles.last_order_at" {
		return column + " " + direction + ", customer_profiles.customer_phone ASC"
	}
	// The old Go sort broke primary-key ties with last_order_at in the
	// same direction as the primary key; keep that so paginated results
	// match the legacy order row for row.
	return column + " " + direction + ", customer_profiles.last_order_at " + direction + ", customer_profiles.customer_phone ASC"
}

func customerProfileRowFromListRow(row customerProfileListRow) CustomerProfileRow {
	out := CustomerProfileRow{
		CustomerPhone:  row.CustomerPhone,
		MerchantID:     row.MerchantID,
		MerchantName:   row.MerchantName,
		StoreName:      row.StoreName,
		OrderCount:     row.OrderCount,
		TotalAmount:    row.TotalAmount,
		DiscountAmount: row.DiscountAmount,
		LastOrderAt:    row.LastOrderAt,
		LastStatus:     row.LastStatus,
	}
	out.AITag, out.AISuggestion = buildCustomerInsight(out)
	return out
}

func customerProfileRowsFromListRows(scanned []customerProfileListRow) []CustomerProfileRow {
	rows := make([]CustomerProfileRow, 0, len(scanned))
	for _, item := range scanned {
		rows = append(rows, customerProfileRowFromListRow(item))
	}
	return rows
}

// --- rebuild (reconciliation / backfill for tests and later ops) ---

// RebuildCustomerProfiles recomputes the materialized profiles from the
// orders table, for one merchant (merchantID > 0) or all merchants (0), and
// overwrites whatever is stored. It is the service-side twin of the v16
// migration backfill, used by tests and later reconciliation. It
// deliberately avoids GROUP_CONCAT (MySQL-only): the per-group aggregates
// are a plain GROUP BY over integers, and the latest / earliest / last-paid
// facts come from plain ordered row lookups, so it runs on SQLite as well as
// MySQL. Returns the number of profiles rebuilt.
func (s *AdminService) RebuildCustomerProfiles(merchantID uint) (int64, error) {
	return rebuildCustomerProfiles(s.db, merchantID)
}

func rebuildCustomerProfiles(db *gorm.DB, merchantID uint) (int64, error) {
	var rebuilt int64
	err := db.Transaction(func(tx *gorm.DB) error {
		type profileAgg struct {
			MerchantID     uint   `gorm:"column:merchant_id"`
			Phone          string `gorm:"column:phone"`
			OrderCount     int64  `gorm:"column:order_count"`
			TotalAmount    int64  `gorm:"column:total_amount"`
			DiscountAmount int64  `gorm:"column:discount_amount"`
		}
		var aggs []profileAgg
		aggQuery := tx.Table("orders").
			Select(`merchant_id,
				customer_phone AS phone,
				COUNT(*) AS order_count,
				COALESCE(SUM(total_amount),0) AS total_amount,
				COALESCE(SUM(discount_amount),0) AS discount_amount`).
			Where("order_type = ? AND customer_phone <> '' AND merchant_id IS NOT NULL", "store_order").
			Where(ValidTradeAmountConditionFor("orders")).
			Group("merchant_id, customer_phone")
		if merchantID > 0 {
			aggQuery = aggQuery.Where("merchant_id = ?", merchantID)
		}
		if err := aggQuery.Scan(&aggs).Error; err != nil {
			return err
		}

		deleteQuery := tx.Session(&gorm.Session{AllowGlobalUpdate: true})
		if merchantID > 0 {
			deleteQuery = tx.Where("merchant_id = ?", merchantID)
		}
		if err := deleteQuery.Delete(&model.CustomerProfile{}).Error; err != nil {
			return err
		}

		for _, agg := range aggs {
			paidScope := tx.Where("merchant_id = ? AND customer_phone = ? AND order_type = ?", agg.MerchantID, agg.Phone, "store_order").
				Where(ValidTradeAmountCondition)
			var latest model.Order
			if err := paidScope.Session(&gorm.Session{}).Preload("Store").Order("created_at DESC, id DESC").First(&latest).Error; err != nil {
				return err
			}
			var earliestPaid model.Order
			if err := paidScope.Session(&gorm.Session{}).Order("paid_at ASC, id ASC").First(&earliestPaid).Error; err != nil {
				return err
			}
			var lastPaid model.Order
			if err := paidScope.Session(&gorm.Session{}).Order("paid_at DESC, id DESC").First(&lastPaid).Error; err != nil {
				return err
			}
			createdAt := latest.CreatedAt
			profile := model.CustomerProfile{
				MerchantID:          agg.MerchantID,
				CustomerPhone:       agg.Phone,
				LastStoreID:         latest.StoreID,
				LastStoreName:       customerProfileStoreNameTx(tx, latest),
				OrderCount:          agg.OrderCount,
				TotalAmountCents:    agg.TotalAmount,
				DiscountAmountCents: agg.DiscountAmount,
				FirstPaidAt:         earliestPaid.PaidAt,
				LastPaidAt:          lastPaid.PaidAt,
				LastOrderAt:         &createdAt,
				LastOrderID:         latest.ID,
				LastStatus:          latest.Status,
			}
			if err := tx.Create(&profile).Error; err != nil {
				return err
			}
			rebuilt++
		}
		return nil
	})
	return rebuilt, err
}
