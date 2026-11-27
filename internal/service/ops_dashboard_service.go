package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

// P3 商家 AI 经营看板（不做聊天入口）：每日经营总结、活动复盘对照、
// 商品体检、沉睡顾客分层唤醒草稿、内容脚本、建议执行闭环。口径红线：
// - 交易数字一律按冻结的交易口径统计（paid_at 不为空且金额大于 0），
// 按收款日期（paid_at，北京时间）归属到自然日；
// - 每个数字都带证据来源：系统记录 / 商家自填 / 外部未接入 / 模型估算 /
// 模板生成；外部未接入的数据绝不写成 0（直接说明未接入）；
// - 收入不是利润：日报只给交易额与退款净额，不算“利润”；
// - 唤醒草稿与内容脚本是给商家自己发布的宣传素材，绝不写顾客评价，
// 绝不把礼品与写评价挂钩；模板内容一律标注“模板兜底/模板生成”；
// - 建议执行闭环：数据 → 建议 → 执行 → 结果，状态流转复用 P2 的
// ai_suggestion_executions 表（待处理 → 已采纳/已忽略 → 已执行）。

// 证据来源标签（与前端 opsDashboard.js 的文案保持一致）。
const (
	// EvidenceSystemRecorded 系统记录：平台订单/支付实际发生的数据。
	EvidenceSystemRecorded = "系统记录"
	// EvidenceMerchantInput 商家自填：商家手工录入的数据（如抖音核销）。
	EvidenceMerchantInput = "商家自填"
	// EvidenceExternalNotConnected 外部未接入：平台无该渠道 API 的数据。
	EvidenceExternalNotConnected = "外部未接入"
	// EvidenceModelEstimate 模型估算：由模型或规则估算的值，不是实测。
	EvidenceModelEstimate = "模型估算"
	// EvidenceTemplate 模板生成：按固定模板生成的内容，未经过模型。
	EvidenceTemplate = "模板生成"
)

// --- 每日经营总结 ---

// OpsDailyChannelRow 是日报里一个渠道的交易拆分行。
type OpsDailyChannelRow struct {
	Channel     string `json:"channel"`
	ChannelName string `json:"channel_name"`
	OrderCount  int64  `json:"order_count"`
	AmountCents int64  `json:"amount_cents"`
	Source      string `json:"source"`
}

// OpsDailySummary 是某商家某自然日（北京时间，按收款日归属）的经营总结。
type OpsDailySummary struct {
	MerchantID   uint                 `json:"merchant_id"`
	Date         string               `json:"date"`
	TradeAmount  int64                `json:"trade_amount_cents"`
	OrderCount   int64                `json:"order_count"`
	RefundAmount int64                `json:"refund_amount_cents"`
	NetAmount    int64                `json:"net_amount_cents"`
	NewCustomers int64                `json:"new_customers"`
	ByChannel    []OpsDailyChannelRow `json:"by_channel"`
	// 抖音手工核销（商家自填）：核销数与核销金额，不并入平台交易额。
	ManualRedemptionCount       int64  `json:"manual_redemption_count"`
	ManualRedemptionAmountCents int64  `json:"manual_redemption_amount_cents"`
	ManualRedemptionSource      string `json:"manual_redemption_source"`
	// 抖音链接点击只有跳转统计、不是到店也不是核销：本接口不返回点击数，
	// 前端如需展示点击请走团购链接接口并标注“仅跳转”。
	DouyinNote  string `json:"douyin_note"`
	TradeSource string `json:"trade_source"`
}

// OpsDailySummary returns one merchant's daily ops summary for a Beijing
// calendar date (default today). Trades are attributed by paid_at; refunds
// by created_at on the same date; new customers are phones whose first valid
// trade fell on that date. Douyin redemptions are merchant-entered manual
// numbers, reported separately and never merged into platform trade amount.
func (s *MerchantService) OpsDailySummary(merchantID uint, dateValue string) (*OpsDailySummary, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("经营数据暂不可用")
	}
	if merchantID == 0 {
		return nil, errors.New("merchant id is required")
	}
	day, err := parseBeijingDate(strings.TrimSpace(dateValue))
	if err != nil {
		if strings.TrimSpace(dateValue) == "" {
			now := time.Now().In(beijingTimeZone)
			day = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, beijingTimeZone)
		} else {
			return nil, err
		}
	}
	dayStart := day
	dayEnd := day.AddDate(0, 0, 1)

	tradeQuery := func() *gorm.DB {
		return s.db.Model(&model.Order{}).
			Where("merchant_id =? AND order_type =? AND "+ValidTradeAmountCondition, merchantID, "store_order").
			Where("paid_at >=? AND paid_at <?", dayStart, dayEnd)
	}
	var orderCount int64
	if err := tradeQuery().Count(&orderCount).Error; err != nil {
		return nil, err
	}
	var tradeAmount int64
	if err := tradeQuery().Select("COALESCE(SUM(total_amount),0)").Scan(&tradeAmount).Error; err != nil {
		return nil, err
	}
	var refundAmount int64
	if err := s.db.Model(&model.RefundRecord{}).
		Where("merchant_id =? AND status =? AND created_at >=? AND created_at <?", merchantID, "success", dayStart, dayEnd).
		Select("COALESCE(SUM(amount),0)").Scan(&refundAmount).Error; err != nil {
		return nil, err
	}
	var newCustomers int64
	if err := s.db.Model(&model.Order{}).
		Select("COUNT(DISTINCT customer_phone)").
		Where("merchant_id =? AND order_type =? AND "+ValidTradeAmountCondition, merchantID, "store_order").
		Where("paid_at >=? AND paid_at <?", dayStart, dayEnd).
		Where("customer_phone <> ''").
		Where("customer_phone NOT IN (?)", s.db.Model(&model.Order{}).
			Select("DISTINCT customer_phone").
			Where("merchant_id =? AND order_type =? AND "+ValidTradeAmountCondition, merchantID, "store_order").
			Where("paid_at <?", dayStart).
			Where("customer_phone <> ''")).
		Scan(&newCustomers).Error; err != nil {
		return nil, err
	}
	type channelAgg struct {
		Channel string `gorm:"column:channel"`
		Orders  int64  `gorm:"column:orders"`
		Amount  int64  `gorm:"column:amount"`
	}
	var channelRows []channelAgg
	if err := tradeQuery().
		Select("channel AS channel, COUNT(*) AS orders, COALESCE(SUM(total_amount),0) AS amount").
		Group("channel").Order("amount DESC").Scan(&channelRows).Error; err != nil {
		return nil, err
	}
	byChannel := make([]OpsDailyChannelRow, 0, len(channelRows))
	for _, row := range channelRows {
		name := opsChannelLabel(row.Channel)
		byChannel = append(byChannel, OpsDailyChannelRow{
			Channel: row.Channel, ChannelName: name,
			OrderCount: row.Orders, AmountCents: row.Amount, Source: EvidenceSystemRecorded,
		})
	}
	var manual struct {
		Count  int64 `gorm:"column:count"`
		Amount int64 `gorm:"column:amount"`
	}
	if err := s.db.Model(&model.GroupBuyManualRedemption{}).
		Select("COALESCE(SUM(redemption_count),0) AS count, COALESCE(SUM(redemption_amount_cents),0) AS amount").
		Where("merchant_id =? AND redemption_date =?", merchantID, day.Format("2006-01-02")).
		Scan(&manual).Error; err != nil {
		return nil, err
	}
	return &OpsDailySummary{
		MerchantID: merchantID, Date: day.Format("2006-01-02"),
		TradeAmount: tradeAmount, OrderCount: orderCount,
		RefundAmount: refundAmount, NetAmount: tradeAmount - refundAmount,
		NewCustomers: newCustomers, ByChannel: byChannel,
		ManualRedemptionCount: manual.Count, ManualRedemptionAmountCents: manual.Amount,
		ManualRedemptionSource: EvidenceMerchantInput,
		DouyinNote:             "抖音团购无平台核销 API：上方核销数是商家在来客核销后手工录入的量，仅供与链接点击数对照参考，不可说成平台验证的核销；链接点击只是跳转，不是到店。",
		TradeSource:            EvidenceSystemRecorded,
	}, nil
}

// opsChannelLabel maps a stored order channel to a display name. Unknown or
// empty channels are shown as 未记录渠道, never silently folded into 扫码.
func opsChannelLabel(channel string) string {
	switch strings.TrimSpace(channel) {
	case model.OrderChannelStoreQR:
		return "扫码点餐"
	case model.OrderChannelTable:
		return "桌台结算"
	case model.OrderChannelShare:
		return "分享下单"
	case "":
		return "渠道未记录"
	default:
		return "其他渠道"
	}
}

// --- 商品体检 ---

// OpsProductHealthRow 是一件商品在窗口期内的体检结果。
type OpsProductHealthRow struct {
	ProductID      uint   `json:"product_id"`
	ProductName    string `json:"product_name"`
	Quantity       int64  `json:"quantity"`
	AmountCents    int64  `json:"amount_cents"`
	OrderCount     int64  `json:"order_count"`
	LastSoldAt     string `json:"last_sold_at"`
	Status         string `json:"status"` // bestseller | normal | slow | unsold
	StatusName     string `json:"status_name"`
	Suggestion     string `json:"suggestion"`
	SuggestionFrom string `json:"suggestion_source"`
}

// OpsProductHealth 是商品体检结果：窗口期内按 order_items 明细汇总。
type OpsProductHealth struct {
	MerchantID  uint                  `json:"merchant_id"`
	Days        int                   `json:"days"`
	WindowStart string                `json:"window_start"`
	WindowEnd   string                `json:"window_end"`
	Rows        []OpsProductHealthRow `json:"rows"`
	Source      string                `json:"source"`
}

// 商品体检分级规则（写在代码里，前端只展示不另发明规则）：
// - 畅销：窗口内有销量，且交易额排进有销量商品的前 25%（至少 1 件）；
// - 滞销：有销量，但窗口内只卖出 2 件及以下，或最后一次售出距窗口结束已超过 14 天；
// - 正常：其余有销量的商品；
// - 未动销：在售商品，窗口内 0 销量。
func (s *MerchantService) OpsProductHealth(merchantID uint, days int) (*OpsProductHealth, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("经营数据暂不可用")
	}
	if merchantID == 0 {
		return nil, errors.New("merchant id is required")
	}
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}
	now := time.Now().In(beijingTimeZone)
	windowEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, beijingTimeZone).AddDate(0, 0, 1)
	windowStart := windowEnd.AddDate(0, 0, -days)

	// 明细行取回来在 Go 里聚合：MAX(paid_at) 这类表达式列在 SQLite 下驱动
	// 返回字符串、扫不进 time.Time；直接取 paid_at 列则驱动按列类型返回
	// time.Time，MySQL/SQLite 两边都稳。
	type itemRow struct {
		ProductID uint      `gorm:"column:product_id"`
		Name      string    `gorm:"column:name"`
		Quantity  int64     `gorm:"column:quantity"`
		Amount    int64     `gorm:"column:amount"`
		OrderID   uint      `gorm:"column:order_id"`
		PaidAt    time.Time `gorm:"column:paid_at"`
	}
	var itemRows []itemRow
	if err := s.db.Table("order_items").
		Select("order_items.product_id AS product_id, order_items.product_name_snapshot AS name, order_items.quantity AS quantity, order_items.amount AS amount, order_items.order_id AS order_id, orders.paid_at AS paid_at").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.merchant_id = ? AND orders.order_type = ? AND "+ValidTradeAmountConditionFor("orders"), merchantID, "store_order").
		Where("orders.paid_at >= ? AND orders.paid_at < ?", windowStart, windowEnd).
		Scan(&itemRows).Error; err != nil {
		return nil, err
	}
	type itemAgg struct {
		ProductID uint
		Name      string
		Quantity  int64
		Amount    int64
		OrderIDs  map[uint]bool
		LastSold  time.Time
	}
	aggByProduct := make(map[uint]*itemAgg)
	for _, row := range itemRows {
		agg, ok := aggByProduct[row.ProductID]
		if !ok {
			agg = &itemAgg{ProductID: row.ProductID, Name: row.Name, OrderIDs: map[uint]bool{}}
			aggByProduct[row.ProductID] = agg
		}
		agg.Quantity += row.Quantity
		agg.Amount += row.Amount
		agg.OrderIDs[row.OrderID] = true
		if row.PaidAt.After(agg.LastSold) {
			agg.LastSold = row.PaidAt
		}
	}
	sold := make([]itemAgg, 0, len(aggByProduct))
	for _, agg := range aggByProduct {
		sold = append(sold, *agg)
	}
	sort.Slice(sold, func(i, j int) bool {
		if sold[i].Amount != sold[j].Amount {
			return sold[i].Amount > sold[j].Amount
		}
		return sold[i].Name < sold[j].Name
	})
	rows := make([]OpsProductHealthRow, 0, len(sold)+4)
	soldIDs := make(map[uint]bool, len(sold))
	// 畅销阈值：有销量商品按交易额排序，前 25%（向上取整，至少 1 件）。
	bestsellerCount := (len(sold) + 3) / 4
	if len(sold) > 0 && bestsellerCount < 1 {
		bestsellerCount = 1
	}
	for i, item := range sold {
		soldIDs[item.ProductID] = true
		status, name, suggestion := "normal", "正常", "销量平稳，保持供应与出品稳定，留意库存不要断货。"
		switch {
		case i < bestsellerCount:
			status, name, suggestion = "bestseller", "畅销", "重点主推：放在菜单显眼位置，可搭配套餐或加价购带动关联商品。"
		case item.Quantity <= 2 || item.LastSold.Before(windowEnd.AddDate(0, 0, -14)):
			status, name, suggestion = "slow", "滞销", "建议做一次限时促销（满减或第二件半价）再观察一轮，仍无起色考虑下架或换季再上。"
		}
		rows = append(rows, OpsProductHealthRow{
			ProductID: item.ProductID, ProductName: item.Name,
			Quantity: item.Quantity, AmountCents: item.Amount, OrderCount: int64(len(item.OrderIDs)),
			LastSoldAt: item.LastSold.Format("2006-01-02"),
			Status:     status, StatusName: name, Suggestion: suggestion, SuggestionFrom: EvidenceTemplate,
		})
	}
	// 未动销：在售商品中窗口内无 order_items 行的。
	var stores []model.Store
	if err := s.db.Where("merchant_id =?", merchantID).Find(&stores).Error; err != nil {
		return nil, err
	}
	if len(stores) > 0 {
		storeIDs := make([]uint, 0, len(stores))
		for _, st := range stores {
			storeIDs = append(storeIDs, st.ID)
		}
		var catalog []model.StoreProduct
		if err := s.db.Where("store_id IN? AND status =?", storeIDs, "active").Find(&catalog).Error; err != nil {
			return nil, err
		}
		for _, p := range catalog {
			if soldIDs[p.ID] {
				continue
			}
			rows = append(rows, OpsProductHealthRow{
				ProductID: p.ID, ProductName: p.Name,
				Quantity: 0, AmountCents: 0, OrderCount: 0, LastSoldAt: "",
				Status: "unsold", StatusName: "未动销",
				Suggestion:     "窗口期内 0 销量：检查是否在菜单上不易看到，或考虑下架、换季重新上架，不要长期占着菜单位置。",
				SuggestionFrom: EvidenceTemplate,
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].AmountCents != rows[j].AmountCents {
			return rows[i].AmountCents > rows[j].AmountCents
		}
		return rows[i].ProductName < rows[j].ProductName
	})
	return &OpsProductHealth{
		MerchantID: merchantID, Days: days,
		WindowStart: windowStart.Format("2006-01-02"), WindowEnd: windowEnd.AddDate(0, 0, -1).Format("2006-01-02"),
		Rows: rows, Source: EvidenceSystemRecorded,
	}, nil
}

// --- 沉睡顾客分层唤醒 ---

// OpsDormantTier 是一层沉睡顾客：人数、抽样手机号（已打码）、唤醒草稿。
type OpsDormantTier struct {
	Tier         string   `json:"tier"` // light | medium | deep
	TierName     string   `json:"tier_name"`
	MinDays      int      `json:"min_days"`
	MaxDays      int      `json:"max_days"`
	Count        int64    `json:"count"`
	SamplePhones []string `json:"sample_phones"`
	Draft        string   `json:"draft"`
	DraftSource  string   `json:"draft_source"`
}

// OpsDormantCustomers 是沉睡顾客分层结果。
type OpsDormantCustomers struct {
	MerchantID uint             `json:"merchant_id"`
	AsOfDate   string           `json:"as_of_date"`
	Tiers      []OpsDormantTier `json:"tiers"`
	Source     string           `json:"source"`
	Note       string           `json:"note"`
}

// 沉睡分层（按最后一次有效交易的距今天数，北京时间自然日）：
// 轻度 31–60 天，中度 61–90 天，深度 91 天以上。30 天内消费过的不算沉睡。
// 手机号只展示打码抽样（前 3 后 4），不返回完整号码。
func (s *MerchantService) OpsDormantCustomers(merchantID uint) (*OpsDormantCustomers, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("经营数据暂不可用")
	}
	if merchantID == 0 {
		return nil, errors.New("merchant id is required")
	}
	now := time.Now().In(beijingTimeZone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, beijingTimeZone)
	// 直接取 phone + paid_at 两列、在 Go 里按手机号取最近一次：同上，
	// MAX(paid_at) 表达式列在 SQLite 下扫不进 time.Time。
	type tradeRow struct {
		Phone  string    `gorm:"column:phone"`
		PaidAt time.Time `gorm:"column:paid_at"`
	}
	var tradeRows []tradeRow
	if err := s.db.Model(&model.Order{}).
		Select("customer_phone AS phone, paid_at AS paid_at").
		Where("merchant_id = ? AND order_type = ? AND "+ValidTradeAmountCondition, merchantID, "store_order").
		Where("customer_phone <> ''").
		Scan(&tradeRows).Error; err != nil {
		return nil, err
	}
	lastByPhone := make(map[string]time.Time, len(tradeRows))
	for _, row := range tradeRows {
		if last, ok := lastByPhone[row.Phone]; !ok || row.PaidAt.After(last) {
			lastByPhone[row.Phone] = row.PaidAt
		}
	}
	type lastTrade struct {
		Phone    string
		LastPaid time.Time
	}
	trades := make([]lastTrade, 0, len(lastByPhone))
	for phone, last := range lastByPhone {
		trades = append(trades, lastTrade{Phone: phone, LastPaid: last})
	}
	tiers := []struct {
		key, name string
		min, max  int
		draft     string
	}{
		{"light", "轻度沉睡（31–60 天未到店）", 31, 60,
			"好久不见！最近店里上了新品，凭这条消息到店可享，欢迎回来尝尝。"},
		{"medium", "中度沉睡（61–90 天未到店）", 61, 90,
			"好久没见你啦，最近店里换了新菜单，凭这条消息到店可享，等你回来。"},
		{"deep", "深度沉睡（91 天以上未到店）", 91, 0,
			"很久没见了，不知道你最近怎么样。店里最近，凭这条消息到店可享，有空回来坐坐。"},
	}
	result := &OpsDormantCustomers{
		MerchantID: merchantID, AsOfDate: today.Format("2006-01-02"),
		Source: EvidenceSystemRecorded,
		Note:   "分层依据是顾客最后一次有效交易的距今天数；手机号仅展示打码抽样。唤醒草稿是模板文案、供商家自己发给顾客，内权益由商家填写，绝不承诺平台没有的优惠，也绝不写顾客评价。",
	}
	for _, tier := range tiers {
		var count int64
		phones := make([]string, 0, 10)
		for _, tr := range trades {
			lp := tr.LastPaid.In(beijingTimeZone)
			lastDate := time.Date(lp.Year(), lp.Month(), lp.Day(), 0, 0, 0, 0, beijingTimeZone)
			days := int(today.Sub(lastDate).Hours() / 24)
			if days < tier.min {
				continue
			}
			if tier.max > 0 && days > tier.max {
				continue
			}
			count++
			if len(phones) < 10 {
				phones = append(phones, maskPhone(tr.Phone))
			}
		}
		result.Tiers = append(result.Tiers, OpsDormantTier{
			Tier: tier.key, TierName: tier.name, MinDays: tier.min, MaxDays: tier.max,
			Count: count, SamplePhones: phones, Draft: tier.draft, DraftSource: EvidenceTemplate,
		})
	}
	return result, nil
}

// maskPhone masks a phone number to first-3 + **** + last-4. Short numbers
// are masked in the middle as well; never return a full phone here.
func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) <= 7 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// --- 内容脚本 ---

// OpsContentScript 是一条短视频/文案脚本建议。
type OpsContentScript struct {
	Scenario    string   `json:"scenario"`
	Title       string   `json:"title"`
	Hook        string   `json:"hook"`
	Lines       []string `json:"lines"`
	Caption     string   `json:"caption"`
	GeneratedBy string   `json:"generated_by"` // ai | template
	Fallback    bool     `json:"fallback"`
	Source      string   `json:"source"`
}

// 内容脚本模板（无模型时直接出模板并标注“模板兜底”，与 MerchantAIView
// 的 fallback 口径一致）。脚本只写商家自己账号的宣传内容，绝不写顾客
// 评价、不编造销量与价格，价格与权益出现时一律留给商家填写。
var opsContentScriptTemplates = map[string]OpsContentScript{
	"new_dish": {
		Scenario: "new_dish", Title: "新品上市",
		Hook: "这道菜我们试了 7 次，终于敢拿出来了。",
		Lines: []string{
			"镜头 1（3 秒）：后厨特写，新品出锅冒热气的画面。",
			"镜头 2（5 秒）：店主口播，这道菜的食材和做法，讲一个真实的细节。",
			"镜头 3（4 秒）：第一位顾客试吃反应，只拍真实表情，不摆拍。",
			"镜头 4（3 秒）：门头 + 今日供应数量，收尾留一句“今天只有”。",
		},
		Caption: "新品上市，今日限量，卖完就没了。",
	},
	"promotion": {
		Scenario: "promotion", Title: "促销活动",
		Hook: "这个活动我们只做三天，过了就没了。",
		Lines: []string{
			"镜头 1（3 秒）：店门口实拍，直接报活动名和时间。",
			"镜头 2（5 秒）：店主口播活动规则，只讲一条最实在的优惠。",
			"镜头 3（4 秒）：往期顾客到店画面，证明活动真实在办。",
			"镜头 4（3 秒）：门头 + 活动结束日期，收尾留一句“截图这条视频到店可用”。",
		},
		Caption: "活动时间，规则，截图可用。",
	},
	"store_story": {
		Scenario: "store_story", Title: "门店故事",
		Hook: "这家店开了这么久，很多人不知道它是怎么来的。",
		Lines: []string{
			"镜头 1（4 秒）：清晨开店实拍，开门、备菜、擦桌子的日常。",
			"镜头 2（6 秒）：店主口播开店的初衷，讲一件真实发生过的小事。",
			"镜头 3（4 秒）：老顾客打招呼的画面，真实、不摆拍。",
			"镜头 4（3 秒）：门头夜景收尾，留一句“明天早上见”。",
		},
		Caption: "开店第天，谢谢每一个回来的人。",
	},
	"festival": {
		Scenario: "festival", Title: "节日问候",
		Hook: "节日到了，先祝大家节日快乐。",
		Lines: []string{
			"镜头 1（3 秒）：店内节日装饰实拍。",
			"镜头 2（5 秒）：店主口播节日安排，营业时间有没有变。",
			"镜头 3（4 秒）：节日限定菜品或小礼品实拍，讲清楚怎么领。",
			"镜头 4（3 秒）：全体员工一句节日快乐收尾。",
		},
		Caption: "节日快乐！节日安排：。",
	},
}

// OpsContentScripts returns a content script for one scenario. It tries the
// configured AI provider first; when no model is wired (the normal state:
// NewAIService installs the disabled provider) it returns the template and
// marks fallback=true, exactly like MerchantAIView's “模板兜底” label.
func (s *MerchantService) OpsContentScripts(ctx context.Context, merchantID uint, scenario string) (*OpsContentScript, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("经营数据暂不可用")
	}
	if merchantID == 0 {
		return nil, errors.New("merchant id is required")
	}
	scenario = strings.TrimSpace(scenario)
	if scenario == "" {
		scenario = "new_dish"
	}
	template, ok := opsContentScriptTemplates[scenario]
	if !ok {
		return nil, errors.New("内容脚本场景无效")
	}
	out := template
	out.GeneratedBy = "template"
	out.Fallback = true
	out.Source = EvidenceTemplate

	// 尝试走模型：系统提示里写死红线（只写商家自己的宣传脚本，不写评价、
	// 不编销量价格）。未接模型时 Generate 直接返回 ErrAIUnavailable，走模板。
	if s.ai != nil {
		start := time.Now()
		result, err := s.ai.Generate(ctx, AIRequest{
			SystemPrompt: "你是本地生活商家短视频脚本助手。只写商家自己账号发布的宣传脚本：开头钩子 + 分镜 + 口播文案 + 发布文案。绝不写顾客评价，绝不编造销量、评分与价格，价格与权益一律写占位。",
			UserPrompt:   "场景：" + template.Title + "。给一家本地餐饮小店写一条 15 秒短视频脚本。",
		})
		_ = s.recordAIUsage(merchantID, "ops_content_script", result, err, time.Since(start))
		if err == nil && result != nil && strings.TrimSpace(result.Content) != "" {
			out.Lines = []string{strings.TrimSpace(result.Content)}
			out.Caption = "模型生成，请商家核对价格、权益与时间后再发布。"
			out.GeneratedBy = "ai"
			out.Fallback = result.Fallback
			out.Source = EvidenceModelEstimate
		}
	}
	return &out, nil
}

// --- 建议执行闭环：从数据自动生成建议 ---

// GenerateOpsSuggestionsResult 是“生成建议”一次调用的结果。
type GenerateOpsSuggestionsResult struct {
	Created int                           `json:"created"`
	Skipped int                           `json:"skipped"`
	Rows    []model.AISuggestionExecution `json:"rows"`
}

// GenerateOpsSuggestions derives actionable suggestions from the merchant's
// own data and opens the execution loop for each (source=system): slow or
// unsold products, dormant-customer tiers, and the latest campaign
// postmortem that missed its target. A suggestion is skipped when an
// identical pending one already exists, so repeated clicks never duplicate
// the todo list.
func (s *MerchantService) GenerateOpsSuggestions(merchantID uint) (*GenerateOpsSuggestionsResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("经营数据暂不可用")
	}
	if merchantID == 0 {
		return nil, errors.New("merchant id is required")
	}
	result := &GenerateOpsSuggestionsResult{Rows: []model.AISuggestionExecution{}}
	record := func(source, title, content string) {
		var existing model.AISuggestionExecution
		err := s.db.Where("merchant_id =? AND source =? AND title =? AND status =?",
			merchantID, source, title, model.SuggestionExecutionPending).First(&existing).Error
		if err == nil {
			result.Skipped++
			return
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			result.Skipped++
			return
		}
		entry := model.AISuggestionExecution{
			MerchantID: merchantID, Source: source, Title: title,
			ContentSnapshot: content, Status: model.SuggestionExecutionPending,
		}
		if err := s.db.Create(&entry).Error; err != nil {
			result.Skipped++
			return
		}
		result.Created++
		result.Rows = append(result.Rows, entry)
	}

	// 1. 商品体检：滞销与未动销，各取前 3 件。
	if health, err := s.OpsProductHealth(merchantID, 30); err == nil {
		n := 0
		for _, row := range health.Rows {
			if row.Status != "slow" && row.Status != "unsold" {
				continue
			}
			if n >= 3 {
				break
			}
			n++
			record(model.SuggestionGeneratedBySystem,
				fmt.Sprintf("处理%s商品：%s", row.StatusName, row.ProductName),
				fmt.Sprintf("近 30 天销量 %d 件、交易额 %s。%s（依据：系统记录的 order_items 明细）", row.Quantity, formatCentsCNY(row.AmountCents), row.Suggestion))
		}
	}
	// 2. 沉睡顾客：每层有人就给一条唤醒建议。
	if dormant, err := s.OpsDormantCustomers(merchantID); err == nil {
		for _, tier := range dormant.Tiers {
			if tier.Count <= 0 {
				continue
			}
			record(model.SuggestionGeneratedBySystem,
				fmt.Sprintf("唤醒%s顾客（%d 人）", tier.TierName, tier.Count),
				fmt.Sprintf("%s，共 %d 人。草稿：%s（依据：系统记录的顾客最后交易时间；草稿为模板，权益由商家填写）", tier.TierName, tier.Count, tier.Draft))
		}
	}
	// 3. 最新活动复盘：实际交易额低于目标 80% 时跟进。
	var latest model.CampaignPostmortem
	if err := s.db.Where("merchant_id =?", merchantID).Order("period_start DESC, id DESC").First(&latest).Error; err == nil {
		if latest.TargetAmountCents > 0 && latest.ActualAmountCents*100 < latest.TargetAmountCents*80 {
			record(model.SuggestionGeneratedBySystem,
				fmt.Sprintf("活动复盘跟进：%s 未达目标", latest.CampaignType),
				fmt.Sprintf("周期 %s 至 %s，目标交易额 %s，实际 %s（完成率 %d%%）。建议：对照复盘结论调整下一轮活动目标或权益，再跑一轮小范围验证。（依据：系统记录的复盘快照）",
					latest.PeriodStart, latest.PeriodEnd,
					formatCentsCNY(latest.TargetAmountCents), formatCentsCNY(latest.ActualAmountCents),
					latest.ActualAmountCents*100/latest.TargetAmountCents))
		}
	}
	return result, nil
}

// formatCentsCNY renders cents as ¥x.xx without importing the frontend
// formatter. Negative values keep the minus sign in front.
func formatCentsCNY(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s¥%d.%02d", sign, cents/100, cents%100)
}
