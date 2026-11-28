package bootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

// Demo merchant dataset for the five-stop product demo (P3收尾第3件重做，
// 2026-11-28)，按当前 HEAD 的数据模型重核后的规格。全部记录一律标注为
// 演示数据，绝不是真实经营结果。
//
// 用法（二选一）：
// 1. 全新安装自动生成：APP_ENV=development 时后端启动走 seedInitialData，
// 已先跑 seedDemoGroupBuyLinks，再跑本种子；生产环境（APP_ENV 非
// development）绝不跑。
// 2. 一键手动生成：go run./cmd/seed-demo（同样只认 APP_ENV=development，
// 否则直接拒绝；跑之前须先跑过 cmd/schema-migrate 建表）。
//
// 幂等：订单号、券号、分享码、退款单号全部用固定编号（与日期无关），其余
// 行按自然键查找已存在即跳过；重复跑不产生重复订单、明细、画像行。画像
// 行由 RebuildCustomerProfiles 从订单表重建，天然不翻倍。活动目标与复盘
// 按（商家、活动）只留一份快照——首次运行锚定最近 7 天周期，之后重跑不
// 另起周期、不追加快照。
//
// 与 2026-11-27 改稿草稿不同的地方（以现行代码为准）：
// - 商家名不是裸“山风咖啡馆”：bootstrap 启动时 seedDemoGroupBuyLinks
// 已建“山风咖啡馆”，种子复用它，不再另建一家。
// - 商品 SalesUnit/Specification/MinimumOrderQuantity 三字段仍在模型里
// （草稿称已删不成立），种子继续填，描述里同时并入单位与规格。
// - 订单号不用 DEMO-<日期>-<序号>（跨天重跑会翻倍），改用固定编号
// DEMO-SF-001…016。
// - 旧种子只有 10 单、今日只有 1 单待支付：现补到 16 单（13 单近 7 天 +
// 3 单沉睡顾客旧单），今日 3 单已付、1 单待支付，日总结、商品体检、
// 沉睡分层、复盘都有数可看。
// - 新增 v11 order_items 明细行、v13 活动目标/复盘/建议执行/抖音手工
// 核销、v16 画像重建；线索 C 的来源按草稿改为 store_qr（团购入口已删）。
// - 演示商家须能真实下单：订阅门控要求商家 active 且订阅未过期，种子
// 给演示商家开 30 天有效订阅；门店 OrderMode 固定为 submit_later
// （pay_first 时订单停在 pending、无法走标记已付流程，见 P2 小结）。
// - 演示商家只在创建时写 active + 审核字段；已存在的待审核商家绝不由
// 种子自动改 active（入驻审核是唯一通过路径，见 P3-1）。

const (
	demoCafeMerchantName = "【演示数据】山风咖啡馆"
	demoCafeStoreName    = "山风咖啡·北街店"
	demoShareCode        = "DEMO-SHANFENG-001"
	// demoDatasetOrderCount is the fixed number of demo orders (13 recent +
	// 3 dormant-customer orders).
	demoDatasetOrderCount = 16
)

var demoBeijingLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type demoProduct struct {
	name     string
	category string
	price    int64 // cents
	unit     string
	spec     string
	desc     string
}

var demoCafeProducts = []demoProduct{
	{"美式咖啡", "咖啡", 1800, "杯", "热/冰可选", "经典美式，演示商品。"},
	{"拿铁", "咖啡", 2200, "杯", "热/冰可选", "牛奶拿铁，演示商品。"},
	{"卡布奇诺", "咖啡", 2200, "杯", "热", "奶泡绵密，演示商品。"},
	{"摩卡", "咖啡", 2500, "杯", "热/冰可选", "巧克力摩卡，演示商品。"},
	{"燕麦拿铁", "咖啡", 2600, "杯", "热/冰可选", "燕麦奶版本，演示商品。"},
	{"冷萃咖啡", "咖啡", 2800, "杯", "冰", "低温慢萃，演示商品。"},
	{"手冲耶加雪菲", "手冲/特调", 3200, "杯", "热", "单品手冲，演示商品。"},
	{"柑橘冷萃", "手冲/特调", 3000, "杯", "冰", "柑橘风味冷萃，演示商品。"},
	{"杨梅气泡美式", "手冲/特调", 2600, "杯", "冰", "季节特调，演示商品。"},
	{"桂花拿铁", "手冲/特调", 2400, "杯", "热/冰可选", "桂花风味拿铁，演示商品。"},
	{"可颂", "轻食", 1200, "份", "", "黄油可颂，演示商品。"},
	{"贝果配奶油芝士", "轻食", 1600, "份", "", "轻食贝果，演示商品。"},
	{"三明治", "轻食", 2000, "份", "", "全麦三明治，演示商品。"},
	{"挂耳咖啡包（5 枚）", "周边", 3800, "包", "", "挂耳组合，演示商品。"},
	{"咖啡豆 200g", "周边", 6800, "包", "", "自家烘焙豆，演示商品。"},
}

type demoOrderItem struct {
	name string
	qty  int
}

type demoOrder struct {
	seq       int
	daysAgo   int
	hour      int
	minute    int
	phone     string
	channel   string
	items     []demoOrderItem
	status    string // store-order lifecycle: pending/accepted/completed
	shareCode string
	discount  int64 // cents, coupon discount applied to this order
	refunded  bool
}

// demoOrders is the frozen demo order list. Amounts are integer fen.
// Recent orders (seq 1-13, last 7 days) feed the daily summary, the
// 30-day product-health window and the 7-day share-campaign postmortem;
// dormant orders (seq 14-16) put one customer in each dormant tier
// (31-60 / 61-90 / 91+ days) and sell only the two peripheral products,
// so those two stay unsold inside the 30-day window.
var demoOrders = []demoOrder{
	{seq: 1, daysAgo: 6, hour: 10, minute: 12, phone: "13800000101", channel: model.OrderChannelStoreQR, status: "completed", items: []demoOrderItem{{"拿铁", 1}, {"可颂", 1}}},
	{seq: 2, daysAgo: 6, hour: 15, minute: 40, phone: "13800000102", channel: model.OrderChannelStoreQR, status: "completed", items: []demoOrderItem{{"冷萃咖啡", 2}}},
	{seq: 3, daysAgo: 5, hour: 9, minute: 5, phone: "13800000103", channel: model.OrderChannelStoreQR, status: "accepted", items: []demoOrderItem{{"美式咖啡", 1}, {"三明治", 1}}},
	{seq: 4, daysAgo: 4, hour: 13, minute: 20, phone: "13800000104", channel: model.OrderChannelTable, status: "completed", items: []demoOrderItem{{"手冲耶加雪菲", 1}, {"贝果配奶油芝士", 1}}},
	{seq: 5, daysAgo: 3, hour: 11, minute: 11, phone: "13800000105", channel: model.OrderChannelStoreQR, status: "completed", items: []demoOrderItem{{"燕麦拿铁", 2}, {"可颂", 2}}},
	{seq: 6, daysAgo: 2, hour: 16, minute: 2, phone: "13800000106", channel: model.OrderChannelStoreQR, status: "accepted", items: []demoOrderItem{{"摩卡", 1}, {"桂花拿铁", 1}, {"可颂", 1}}},
	{seq: 7, daysAgo: 1, hour: 10, minute: 30, phone: "13800000107", channel: model.OrderChannelShare, status: "completed", shareCode: demoShareCode, items: []demoOrderItem{{"卡布奇诺", 2}}},
	{seq: 8, daysAgo: 1, hour: 14, minute: 45, phone: "13800000108", channel: model.OrderChannelShare, status: "completed", shareCode: demoShareCode, discount: 500, items: []demoOrderItem{{"柑橘冷萃", 1}, {"三明治", 1}}},
	{seq: 9, daysAgo: 0, hour: 9, minute: 15, phone: "13800000109", channel: model.OrderChannelStoreQR, status: "pending", items: []demoOrderItem{{"杨梅气泡美式", 1}}},
	{seq: 10, daysAgo: 2, hour: 12, minute: 30, phone: "13800000110", channel: model.OrderChannelTable, status: "accepted", items: []demoOrderItem{{"摩卡", 1}, {"卡布奇诺", 1}}},
	{seq: 11, daysAgo: 0, hour: 11, minute: 20, phone: "13800000111", channel: model.OrderChannelTable, status: "completed", items: []demoOrderItem{{"美式咖啡", 2}, {"拿铁", 1}}},
	{seq: 12, daysAgo: 0, hour: 13, minute: 5, phone: "13800000112", channel: model.OrderChannelStoreQR, status: "completed", items: []demoOrderItem{{"冷萃咖啡", 1}, {"可颂", 1}}},
	{seq: 13, daysAgo: 0, hour: 15, minute: 40, phone: "13800000113", channel: model.OrderChannelStoreQR, status: "accepted", refunded: true, items: []demoOrderItem{{"拿铁", 1}, {"贝果配奶油芝士", 1}}},
	{seq: 14, daysAgo: 45, hour: 11, minute: 0, phone: "13800000301", channel: model.OrderChannelStoreQR, status: "completed", items: []demoOrderItem{{"挂耳咖啡包（5 枚）", 1}}},
	{seq: 15, daysAgo: 75, hour: 11, minute: 0, phone: "13800000302", channel: model.OrderChannelStoreQR, status: "completed", items: []demoOrderItem{{"咖啡豆 200g", 1}}},
	{seq: 16, daysAgo: 120, hour: 11, minute: 0, phone: "13800000303", channel: model.OrderChannelStoreQR, status: "completed", items: []demoOrderItem{{"美式咖啡", 1}, {"挂耳咖啡包（5 枚）", 1}}},
}

// demoDatasetTables lists every table the dataset writes. seedInitialData
// runs the dataset against a development database that may be only
// partially migrated (see initial_data_seed_test.go); a missing table
// means "not a full install", so the auto path skips quietly instead of
// failing startup. The strict cmd/seed-demo entry checks the same list
// and fails loudly (see SeedDemoMerchantDataset).
func demoDatasetTablesReady(db *gorm.DB) bool {
	for _, table := range []interface{}{
		&model.Merchant{}, &model.Store{}, &model.StoreProduct{},
		&model.Order{}, &model.OrderItem{}, &model.RefundRecord{},
		&model.CustomerLead{}, &model.ShareCampaign{}, &model.ReferralCoupon{},
		&model.Promotion{}, &model.CampaignTarget{}, &model.CampaignPostmortem{},
		&model.AISuggestionExecution{}, &model.GroupBuyManualRedemption{},
		&model.CustomerProfile{}, &model.MerchantGroupBuyLink{}, &model.MerchantGroupBuyLinkAudit{},
	} {
		if !db.Migrator().HasTable(table) {
			return false
		}
	}
	return true
}

// SeedDemoMerchantDataset is the strict one-click entry (cmd/seed-demo):
// it refuses to run when the schema is incomplete instead of skipping.
func SeedDemoMerchantDataset(db *gorm.DB) error {
	if db == nil {
		return errors.New("demo seed requires a database handle")
	}
	if !demoDatasetTablesReady(db) {
		return errors.New("demo seed requires the full migrated schema (run cmd/schema-migrate first)")
	}
	return seedDemoMerchantDataset(db)
}

// demoDayTime returns the clock time hour:minute on the day daysAgo days
// before base, so demo data always looks recent relative to the demo day.
func demoDayTime(base time.Time, daysAgo, hour, minute int) time.Time {
	day := time.Date(base.Year(), base.Month(), base.Day(), hour, minute, 0, 0, base.Location())
	return day.AddDate(0, 0, -daysAgo)
}

func findOrCreateDemoCafe(db *gorm.DB, now time.Time) (model.Merchant, model.Store, error) {
	// Reuse the merchant/store created by seedDemoGroupBuyLinks (same
	// names); only create them here when that seed did not run first
	// (standalone cmd/seed-demo on a database without the links).
	merchant := model.Merchant{}
	if err := db.Where("name =?", demoCafeMerchantName).First(&merchant).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return merchant, model.Store{}, err
		}
		expire := now.AddDate(0, 0, 30)
		merchant = model.Merchant{
			Name: demoCafeMerchantName, Status: "active", ContactPhone: "00000000000",
			ReviewReason: "演示数据：开发环境演示商家，入驻审核已通过", ReviewedBy: 1, ReviewedAt: &now,
			SubscriptionPlan: "month", SubscriptionStatus: "active",
			SubscriptionExpireAt: &expire, SubscriptionExpiredAt: &expire,
		}
		if err := db.Create(&merchant).Error; err != nil {
			return merchant, model.Store{}, err
		}
	}
	store := model.Store{}
	if err := db.Where("merchant_id =? AND name =?", merchant.ID, demoCafeStoreName).First(&store).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return merchant, store, err
		}
		store = model.Store{MerchantID: merchant.ID, Name: demoCafeStoreName, Status: "active", IsOpen: true, OrderMode: "submit_later"}
		if err := db.Create(&store).Error; err != nil {
			return merchant, store, err
		}
	}
	// Demo-store-only normalization (never touches another merchant's
	// store): submit_later so a live order can go through mark-paid, and
	// the store stays open. The merchant's review status is NOT touched
	// here — a pending merchant stays pending by design.
	if store.OrderMode != "submit_later" || store.Status != "active" || !store.IsOpen {
		if err := db.Model(&store).Updates(map[string]interface{}{
			"order_mode": "submit_later", "status": "active", "is_open": true,
		}).Error; err != nil {
			return merchant, store, err
		}
		store.OrderMode = "submit_later"
		store.Status = "active"
		store.IsOpen = true
	}
	// Subscription gate (H1): a merchant without a live subscription
	// cannot take new store orders at all, so the demo merchant needs one.
	// Only extended for an already-active merchant; a pending merchant is
	// left alone (review flow owns its status).
	if merchant.Status == "active" && (merchant.SubscriptionExpireAt == nil || merchant.SubscriptionExpireAt.Before(now.AddDate(0, 0, 7))) {
		expire := now.AddDate(0, 0, 30)
		if err := db.Model(&merchant).Updates(map[string]interface{}{
			"subscription_status": "active", "subscription_plan": "month",
			"subscription_expire_at": expire, "subscription_expired_at": expire,
		}).Error; err != nil {
			return merchant, store, err
		}
		merchant.SubscriptionStatus = "active"
		merchant.SubscriptionPlan = "month"
		merchant.SubscriptionExpireAt = &expire
		merchant.SubscriptionExpiredAt = &expire
	}
	return merchant, store, nil
}

// seedDemoMerchantDataset fills the demo cafe with products, orders (+v11
// detail rows), coupons, one share campaign, leads, a promotion, a campaign
// target + postmortem, one manual Douyin redemption, suggestion executions
// and rebuilt customer profiles, so the demo dashboards are not empty.
// It never touches the group-buy links themselves.
func seedDemoMerchantDataset(db *gorm.DB) error {
	if !demoDatasetTablesReady(db) {
		return nil
	}
	// The links seed also creates the demo merchant/store; running it here
	// makes the dataset self-contained for the standalone cmd entry.
	if err := seedDemoGroupBuyLinks(db); err != nil {
		return err
	}
	now := time.Now().In(demoBeijingLocation)
	merchant, store, err := findOrCreateDemoCafe(db, now)
	if err != nil {
		return err
	}

	// --- Products (idempotent by store + name; existing rows untouched) ---
	productByName := make(map[string]model.StoreProduct, len(demoCafeProducts))
	for i, p := range demoCafeProducts {
		desc := p.desc
		if p.unit != "" {
			desc += "单位：" + p.unit + "；"
		}
		if p.spec != "" {
			desc += p.spec
		}
		product := model.StoreProduct{
			StoreID: store.ID, Name: p.name, Price: p.price, Category: p.category,
			Status: "active", Sort: (i + 1) * 10, SalesUnit: p.unit,
			Specification: p.spec, Description: desc, MinimumOrderQuantity: 1,
		}
		if err := db.Where("store_id =? AND name =?", store.ID, p.name).First(&product).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Create(&product).Error; err != nil {
				return err
			}
		}
		productByName[p.name] = product
	}
	priceByName := make(map[string]int64, len(demoCafeProducts))
	for _, p := range demoCafeProducts {
		priceByName[p.name] = p.price
	}

	// --- Orders (idempotent by fixed order_no) + v11 detail rows ---
	ordersBySeq := make(map[int]model.Order, len(demoOrders))
	for _, spec := range demoOrders {
		created := demoDayTime(now, spec.daysAgo, spec.hour, spec.minute)
		orderNo := fmt.Sprintf("DEMO-SF-%03d", spec.seq)
		var total int64
		snapshots := make([]map[string]interface{}, 0, len(spec.items))
		for _, item := range spec.items {
			price := priceByName[item.name]
			line := price * int64(item.qty)
			total += line
			snapshot := map[string]interface{}{
				"name": item.name, "price": price, "quantity": item.qty, "line_amount": line,
			}
			if product, ok := productByName[item.name]; ok {
				snapshot["product_id"] = product.ID
			}
			snapshots = append(snapshots, snapshot)
		}
		itemsJSON, err := json.Marshal(snapshots)
		if err != nil {
			return err
		}
		order := model.Order{
			OrderNo: orderNo, OrderType: "store_order",
			MerchantID: &merchant.ID, StoreID: &store.ID,
			CustomerPhone: spec.phone, Amount: total, TotalAmount: total - spec.discount,
			DiscountAmount: spec.discount, Items: string(itemsJSON),
			Status: spec.status, Channel: spec.channel, ShareCode: spec.shareCode,
			MerchantNote: "演示订单：演示数据，非真实交易",
			BaseModel:    model.BaseModel{CreatedAt: created, UpdatedAt: created},
		}
		if spec.status != "pending" {
			paidAt := created.Add(2 * time.Minute)
			order.PaidAt = &paidAt
			order.PaymentChannel = "alipay"
			order.TransactionNo = fmt.Sprintf("DEMO-TXN-%03d", spec.seq)
		}
		if spec.refunded {
			order.RefundStatus = "full"
			order.RefundedAmount = order.TotalAmount
		}
		if err := db.Where("order_no =?", orderNo).First(&order).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Create(&order).Error; err != nil {
				return err
			}
		}
		ordersBySeq[spec.seq] = order

		// v11 detail rows (idempotent by order + product).
		for _, item := range spec.items {
			product, ok := productByName[item.name]
			if !ok {
				continue
			}
			detail := model.OrderItem{
				OrderID: order.ID, ProductID: product.ID,
				ProductNameSnapshot: item.name, Quantity: item.qty,
				UnitPrice: product.Price, Amount: product.Price * int64(item.qty),
			}
			if err := db.Where("order_id =? AND product_id =?", order.ID, product.ID).First(&detail).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if err := db.Create(&detail).Error; err != nil {
					return err
				}
			}
		}

		if spec.refunded {
			refund := model.RefundRecord{
				OrderID: order.ID, MerchantID: &merchant.ID, StoreID: &store.ID,
				RefundNo: "DEMO-REFUND-001", TransactionNo: order.TransactionNo,
				Amount: order.TotalAmount, Reason: "演示数据：顾客取消订单，全额退款",
				OperatorRole: "merchant", Status: "success",
				BaseModel: model.BaseModel{CreatedAt: created.Add(30 * time.Minute), UpdatedAt: created.Add(30 * time.Minute)},
			}
			if err := db.Where("refund_no =?", refund.RefundNo).First(&refund).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if err := db.Create(&refund).Error; err != nil {
					return err
				}
			}
		}
	}

	// --- Share campaign (idempotent by share_code) ---
	referrerOrder := ordersBySeq[7]
	convertedOrder := ordersBySeq[8]
	campaign := model.ShareCampaign{
		MerchantID: merchant.ID, StoreID: store.ID, OrderID: &referrerOrder.ID,
		ShareCode: demoShareCode, CustomerPhone: referrerOrder.CustomerPhone,
		PosterTitle: "分享得 5 元券",
		PosterCopy:  "演示数据：我刚在山风咖啡·北街店下单，体验不错。扫码领取好友专属优惠，下次一起试试。",
		ScanCount:   12, LeadCount: 2, ConversionCount: 1,
		ConversionAmount: convertedOrder.TotalAmount, Status: "active",
	}
	if convertedOrder.PaidAt != nil {
		campaign.LastScannedAt = convertedOrder.PaidAt
	}
	if err := db.Where("share_code =?", demoShareCode).First(&campaign).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := db.Create(&campaign).Error; err != nil {
			return err
		}
	}

	// --- Referral coupons (idempotent by coupon_no) ---
	validFrom := demoDayTime(now, 2, 0, 0)
	validTo := demoDayTime(now, -365, 23, 59)
	expiredFrom := demoDayTime(now, 40, 0, 0)
	expiredTo := demoDayTime(now, 1, 23, 59)
	coupons := []model.ReferralCoupon{
		{
			CouponNo: "DEMO-CPN-001", OwnerPhone: "13800000205", OwnerType: "new_customer",
			Title: "满 30 减 5 好友券", Amount: 500, Threshold: 3000,
			Status: "unused", ValidFrom: &validFrom, ValidTo: &validTo,
			Remark: "演示数据：分享活动好友券",
		},
		{
			CouponNo: "DEMO-CPN-002", OwnerPhone: "13800000204", OwnerType: "new_customer",
			Title: "满 68 减 10 好友券", Amount: 1000, Threshold: 6800,
			Status: "unused", ValidFrom: &validFrom, ValidTo: &validTo,
			Remark: "演示数据：分享活动好友券",
		},
		{
			CouponNo: "DEMO-CPN-003", OwnerPhone: convertedOrder.CustomerPhone, OwnerType: "new_customer",
			Title: "满 30 减 5 好友券", Amount: 500, Threshold: 3000,
			Status: "used", ValidFrom: &validFrom, ValidTo: &validTo,
			OrderID: &convertedOrder.ID, UsedAt: convertedOrder.PaidAt,
			Remark: "演示数据：已在演示订单中核销",
		},
		{
			CouponNo: "DEMO-CPN-004", OwnerPhone: referrerOrder.CustomerPhone, OwnerType: "referrer",
			Title: "满 68 减 10 推荐人券", Amount: 1000, Threshold: 6800,
			Status: "expired", ValidFrom: &expiredFrom, ValidTo: &expiredTo,
			Remark: "演示数据：已过期演示券",
		},
	}
	var usedCoupon model.ReferralCoupon
	for _, coupon := range coupons {
		coupon.MerchantID = merchant.ID
		coupon.StoreID = store.ID
		coupon.ShareCampaignID = &campaign.ID
		if err := db.Where("coupon_no =?", coupon.CouponNo).First(&coupon).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Create(&coupon).Error; err != nil {
				return err
			}
		}
		if coupon.CouponNo == "DEMO-CPN-003" {
			usedCoupon = coupon
		}
	}
	// Link the converted order back to the coupon it consumed, mirroring how
	// the real order flow records coupon usage.
	if usedCoupon.ID != 0 && convertedOrder.CouponID == nil {
		if err := db.Model(&model.Order{}).Where("id =?", convertedOrder.ID).
			Updates(map[string]interface{}{"referral_coupon_id": usedCoupon.ID, "coupon_no": usedCoupon.CouponNo}).Error; err != nil {
			return err
		}
	}

	// --- Promotion (idempotent by merchant + title) ---
	promoFrom := demoDayTime(now, 30, 0, 0)
	promoTo := demoDayTime(now, -30, 23, 59)
	promotion := model.Promotion{
		MerchantID: merchant.ID, StoreID: &store.ID,
		Title: "演示：满 50 减 5", Description: "演示数据：满 50 元减 5 元，演示活动，非真实优惠。",
		Type: "amount", Threshold: 5000, Discount: 500, Status: "published",
		ValidFrom: &promoFrom, ValidTo: &promoTo,
	}
	if err := db.Where("merchant_id =? AND title =?", merchant.ID, promotion.Title).First(&promotion).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := db.Create(&promotion).Error; err != nil {
			return err
		}
	}

	// --- Customer leads (idempotent by merchant + phone) ---
	// Lead statuses use the repository's real enum (new/contacted/converted).
	// Lead C's source is store_qr (the 2026-11-27 draft fix): the group-buy
	// entry channel no longer exists, and the message now says 扫桌码.
	type demoLead struct {
		name    string
		phone   string
		source  string
		status  string
		message string
		daysAgo int
	}
	leads := []demoLead{
		{"演示顾客A", "13800000201", "store_qr", "new", "问了手冲豆子什么时候上新", 0},
		{"演示顾客B", "13800000202", "store_qr", "new", "想订周末的靠窗位", 1},
		{"演示顾客C", "13800000203", "store_qr", "new", "扫桌码进来的，问套餐内容", 1},
		{"演示顾客D", "13800000204", "store_qr", "contacted", "咨询团购套餐能不能叠加优惠", 2},
		{"演示顾客E", "13800000205", "share", "contacted", "朋友分享来的，问营业时间", 2},
		{"演示顾客F", convertedOrder.CustomerPhone, "share", "converted", "朋友分享后到店下了单", 1},
	}
	for _, spec := range leads {
		created := demoDayTime(now, spec.daysAgo, 12, 0)
		lead := model.CustomerLead{
			MerchantID: merchant.ID, StoreID: store.ID,
			CustomerName: spec.name, CustomerPhone: spec.phone,
			Source: spec.source, Status: spec.status, Message: spec.message,
			BaseModel: model.BaseModel{CreatedAt: created, UpdatedAt: created},
		}
		if err := db.Where("merchant_id =? AND customer_phone =?", merchant.ID, spec.phone).First(&lead).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Create(&lead).Error; err != nil {
				return err
			}
		}
	}

	// --- Campaign target + postmortem (one snapshot per campaign) ---
	// The period is the last 7 days on first run and then frozen: a later
	// re-run reuses the existing target/postmortem instead of opening a
	// second period, so the dataset never doubles.
	periodStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, demoBeijingLocation).AddDate(0, 0, -6)
	periodEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, demoBeijingLocation)
	var existingTarget model.CampaignTarget
	targetPeriodStart, targetPeriodEnd := periodStart, periodEnd
	if err := db.Where("merchant_id =? AND campaign_type =? AND campaign_id =?",
		merchant.ID, model.CampaignTypeShareCampaign, campaign.ID).
		Order("period_start DESC").First(&existingTarget).Error; err == nil {
		parsed, perr := time.ParseInLocation("2006-01-02", string(existingTarget.PeriodStart), demoBeijingLocation)
		if perr == nil {
			targetPeriodStart = parsed
		}
		parsedEnd, perr := time.ParseInLocation("2006-01-02", string(existingTarget.PeriodEnd), demoBeijingLocation)
		if perr == nil {
			targetPeriodEnd = parsedEnd
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	} else {
		target := model.CampaignTarget{
			MerchantID: merchant.ID, CampaignType: model.CampaignTypeShareCampaign, CampaignID: campaign.ID,
			TargetOrders: 15, TargetAmountCents: 80000, TargetNewCustomers: 15,
			PeriodStart: model.PlatformExpenseDate(targetPeriodStart.Format("2006-01-02")),
			PeriodEnd:   model.PlatformExpenseDate(targetPeriodEnd.Format("2006-01-02")),
			CreatedBy:   1,
		}
		if err := db.Create(&target).Error; err != nil {
			return err
		}
		existingTarget = target
	}
	var existingPostmortem model.CampaignPostmortem
	if err := db.Where("merchant_id =? AND campaign_type =? AND campaign_id =?",
		merchant.ID, model.CampaignTypeShareCampaign, campaign.ID).
		Order("period_start DESC").First(&existingPostmortem).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		actualOrders, actualAmount, actualNewCustomers, err := demoCampaignActuals(db, merchant.ID, targetPeriodStart, targetPeriodEnd)
		if err != nil {
			return err
		}
		postmortem := model.CampaignPostmortem{
			MerchantID: merchant.ID, CampaignType: model.CampaignTypeShareCampaign, CampaignID: campaign.ID,
			PeriodStart:  model.PlatformExpenseDate(targetPeriodStart.Format("2006-01-02")),
			PeriodEnd:    model.PlatformExpenseDate(targetPeriodEnd.Format("2006-01-02")),
			TargetOrders: existingTarget.TargetOrders, TargetAmountCents: existingTarget.TargetAmountCents,
			ActualOrders: actualOrders, ActualAmountCents: actualAmount, ActualNewCustomers: actualNewCustomers,
			Conclusion:  "演示数据：分享活动首轮复盘，实际交易额未达目标，下一轮先缩小权益范围再验证。以上均为演示数据，非真实经营结果。",
			GeneratedBy: model.SuggestionGeneratedBySystem, GeneratedAt: now,
		}
		if err := db.Create(&postmortem).Error; err != nil {
			return err
		}
	}

	// --- Douyin manual redemption (idempotent by merchant + link + date) ---
	// The date is the paid date of today's demo order, so a re-run on a
	// later day looks the same row up instead of adding a second one. The
	// link comes from the group-buy seed; without it there is nothing to
	// attach the redemption to, so this step is skipped, not failed.
	var douyinLink model.MerchantGroupBuyLink
	if err := db.Where("merchant_id =? AND channel =?", merchant.ID, "douyin").First(&douyinLink).Error; err == nil {
		redemptionDate := ordersBySeq[12].PaidAt
		if redemptionDate != nil {
			dateStr := redemptionDate.In(demoBeijingLocation).Format("2006-01-02")
			redemption := model.GroupBuyManualRedemption{
				MerchantID: merchant.ID, StoreID: &store.ID, LinkID: douyinLink.ID,
				RedemptionDate:  model.PlatformExpenseDate(dateStr),
				RedemptionCount: 3, RedemptionAmountCents: 15000,
				Note: "演示数据：抖音来客核销后商家手工录入，非平台验证核销", RecordedBy: 1,
			}
			if err := db.Where("merchant_id =? AND link_id =? AND redemption_date =?",
				merchant.ID, douyinLink.ID, dateStr).First(&redemption).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if err := db.Create(&redemption).Error; err != nil {
					return err
				}
			}
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// --- Suggestion executions (idempotent by merchant + title) ---
	suggestions := []model.AISuggestionExecution{
		{
			MerchantID: merchant.ID, StoreID: &store.ID, Source: model.SuggestionGeneratedBySystem,
			Title:           "处理滞销商品：桂花拿铁",
			ContentSnapshot: "演示数据：近 30 天销量 1 件。建议做一次限时促销再观察一轮，仍无起色考虑下架或换季再上。（依据：系统记录的 order_items 明细）",
			Status:          model.SuggestionExecutionPending,
		},
		{
			MerchantID: merchant.ID, StoreID: &store.ID, Source: model.SuggestionGeneratedBySystem,
			Title:           "唤醒轻度沉睡顾客（演示）",
			ContentSnapshot: "演示数据：轻度沉睡（31–60 天未到店）顾客 1 人。草稿为模板，权益由商家填写。（依据：系统记录的顾客最后交易时间）",
			Status:          model.SuggestionExecutionExecuted, ExecutedAt: &now,
			EffectNote: "演示数据：已执行唤醒消息，效果待复盘（演示记录，非真实效果）。",
		},
	}
	for _, suggestion := range suggestions {
		entry := suggestion
		if err := db.Where("merchant_id =? AND title =?", merchant.ID, entry.Title).First(&entry).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Create(&entry).Error; err != nil {
				return err
			}
		}
	}

	// --- Customer profiles (v16): rebuild from orders, never duplicated ---
	// NewAdminService only needs the db handle on this path (cfg is unused
	// by the rebuild); bootstrap has no config here, so nil is passed.
	if _, err := service.NewAdminService(nil, db).RebuildCustomerProfiles(merchant.ID); err != nil {
		return err
	}
	return nil
}

// demoCampaignActuals mirrors service.computeCampaignActuals: valid trades
// (paid_at set, total_amount > 0) paid inside [start, end], and the new
// customers among them (no valid trade before the period).
func demoCampaignActuals(db *gorm.DB, merchantID uint, start, end time.Time) (orders int64, amount int64, newCustomers int64, err error) {
	periodEndExclusive := end.AddDate(0, 0, 1)
	type tradeAgg struct {
		Orders int64 `gorm:"column:orders"`
		Amount int64 `gorm:"column:amount"`
	}
	var agg tradeAgg
	tradeErr := db.Model(&model.Order{}).
		Select("COUNT(*) AS orders, COALESCE(SUM(total_amount),0) AS amount").
		Where("merchant_id =? AND order_type =? AND "+service.ValidTradeAmountCondition, merchantID, "store_order").
		Where("paid_at >=? AND paid_at <?", start, periodEndExclusive).
		Scan(&agg).Error
	if tradeErr != nil {
		return 0, 0, 0, tradeErr
	}
	var customers int64
	customerErr := db.Model(&model.Order{}).
		Select("COUNT(DISTINCT customer_phone)").
		Where("merchant_id =? AND order_type =? AND "+service.ValidTradeAmountCondition, merchantID, "store_order").
		Where("paid_at >=? AND paid_at <?", start, periodEndExclusive).
		Where("customer_phone <> ''").
		Where("customer_phone NOT IN (?)", db.Model(&model.Order{}).
			Select("DISTINCT customer_phone").
			Where("merchant_id =? AND order_type =? AND "+service.ValidTradeAmountCondition, merchantID, "store_order").
			Where("paid_at <?", start).
			Where("customer_phone <> ''")).
		Scan(&customers).Error
	if customerErr != nil {
		return 0, 0, 0, customerErr
	}
	return agg.Orders, agg.Amount, customers, nil
}
