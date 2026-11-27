package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

// AI 成本控制（P1 MVP）后端实现。口径：
//   - 预算执行口径：调用记录上的 EstimatedCostCents 累计（写入时按当时生效价
//     或内置费率估算，CNY 分）。价格表事后改价不改写已记成本。
//   - 统计/核对口径：按调用时间取当时生效的价格表重算估算成本；无生效价的
//     调用回落到记录上的 EstimatedCostCents。两者在返回里分别标注来源。
//   - 估算成本永远不是供应商真实账单；账单只来自超管手工录入的 AIVendorBill。
//   - 预算、预警、超额策略、紧急关闭全部在后端强制，AI 调用入口先过
//     “紧急关闭 → 商家预算 → 策略”检查再执行。

const (
	maxAIBudgetCents       int64 = 100_000_000_000
	maxAIModelPriceCents   int64 = 100_000_000 // 分/百万 Token 上限
	maxAIVendorBillCents   int64 = 100_000_000_000
	maxAICostNoteRunes           = 500
	aiBudgetAlertListLimit       = 500
)

var (
	ErrInvalidAICostControl    = errors.New("invalid AI cost control request")
	ErrAIEmergencyDisabled     = errors.New("AI 已由超管紧急关闭，暂不可用")
	ErrAIBudgetExceeded        = errors.New("已达到 AI 预算上限，调用被拒绝")
	ErrAIBudgetManualReview    = errors.New("已达到 AI 预算上限，需超管人工审核后方可继续")
	ErrAIHighCostRestricted    = errors.New("已达到 AI 预算上限，高成本功能暂不可用")
	ErrAIModelPriceNotFound    = errors.New("model price was not found")
	ErrAIBudgetNotFound        = errors.New("merchant AI budget was not found")
	ErrAIVendorBillKeyConflict = errors.New("vendor bill idempotency key conflicts with an existing bill")
	ErrAIModelPriceKeyConflict = errors.New("model price already exists for this model, provider and effective time")
	ErrAINoDowngradeModel      = errors.New("已达到 AI 预算上限且未配置可降级模型，请超管配置模型价格表或调整预算")
)

// AIBudgetThresholds 是预警阈值（百分比），按达到顺序产生预警记录。
var AIBudgetThresholds = []int{70, 90, 100}

// highCostAIScenarios 是“限制高成本功能”策略下被拒绝的场景：
// 多文案批量生成与分享活动复盘是输出量最大的两类调用。其余场景放行。
// 这是一份写明的约定，不是按调用量动态判定的启发式规则。
var highCostAIScenarios = map[string]bool{
	"referral_multi_copy":   true,
	"share_campaign_review": true,
}

// DB exposes the service database handle for cost-control guards that share
// one implementation between the admin and merchant services.
func (s *AdminService) DB() *gorm.DB { return s.db }

// AICostGuard 是 AI 调用入口的门禁：紧急关闭 → 商家预算 → 超额策略。
// AdminService 与 MerchantService 共用同一实现，保证两端口径一致。
type AICostGuard struct {
	db *gorm.DB
}

func NewAICostGuard(db *gorm.DB) *AICostGuard { return &AICostGuard{db: db} }

// AIAccessDecision 是门禁的判定结果。Allowed=false 时调用方必须返回对应的
// 错误，不得继续执行；DowngradeModel 非空时调用方必须用该模型发起调用。
type AIAccessDecision struct {
	Allowed        bool                    `json:"allowed"`
	Action         string                  `json:"action"`
	DowngradeModel string                  `json:"downgrade_model,omitempty"`
	Reason         string                  `json:"reason"`
	BudgetStatus   *MerchantAIBudgetStatus `json:"budget_status,omitempty"`
}

// CheckMerchantAIAccess 在 AI 调用执行前做强制检查：
// 1) 全局紧急关闭 → 直接拒绝；2) 商家预算（无商家行时用全局默认行，
// 都无则不限额）；3) 达到预算后按配置策略处理。检查过程会补记本次周期内
// 已达到阈值的预警记录（幂等，不重复产生）。
func (g *AICostGuard) CheckMerchantAIAccess(merchantID uint, scenario string) (AIAccessDecision, error) {
	if g == nil || g.db == nil {
		return AIAccessDecision{}, errors.New("AI cost control database is unavailable")
	}
	disabled, _, err := g.GetAIEmergencySwitch()
	if err != nil {
		return AIAccessDecision{}, err
	}
	if disabled {
		return AIAccessDecision{Allowed: false, Action: "denied_emergency_disabled", Reason: ErrAIEmergencyDisabled.Error()}, ErrAIEmergencyDisabled
	}
	status, err := g.EvaluateMerchantBudget(merchantID)
	if err != nil {
		return AIAccessDecision{}, err
	}
	decision := AIAccessDecision{Allowed: true, Action: "allow", BudgetStatus: status}
	if status == nil || !status.OverLimit {
		return decision, nil
	}
	switch status.EffectiveAction {
	case model.AIBudgetOverLimitDowngrade:
		cheapest, err := g.cheapestActiveModel(time.Now())
		if err != nil {
			return AIAccessDecision{}, err
		}
		if cheapest == "" {
			return AIAccessDecision{Allowed: false, Action: "denied_no_downgrade_model", Reason: ErrAINoDowngradeModel.Error(), BudgetStatus: status}, ErrAINoDowngradeModel
		}
		decision.Action = "allow_downgraded"
		decision.DowngradeModel = cheapest
		decision.Reason = "已达到 AI 预算上限，已降级到低成本模型"
		return decision, nil
	case model.AIBudgetOverLimitRestrictExpensive:
		if highCostAIScenarios[strings.TrimSpace(scenario)] {
			return AIAccessDecision{Allowed: false, Action: "denied_high_cost_restricted", Reason: ErrAIHighCostRestricted.Error(), BudgetStatus: status}, ErrAIHighCostRestricted
		}
		decision.Action = "allow_restricted"
		decision.Reason = "已达到 AI 预算上限，仅放行低成本功能"
		return decision, nil
	case model.AIBudgetOverLimitManualReview:
		return AIAccessDecision{Allowed: false, Action: "denied_manual_review", Reason: ErrAIBudgetManualReview.Error(), BudgetStatus: status}, ErrAIBudgetManualReview
	default: // stop
		return AIAccessDecision{Allowed: false, Action: "denied_budget_exceeded", Reason: ErrAIBudgetExceeded.Error(), BudgetStatus: status}, ErrAIBudgetExceeded
	}
}

// MerchantAIBudgetStatus 是某商家在当前日/月周期的预算使用快照。
type MerchantAIBudgetStatus struct {
	MerchantID          uint   `json:"merchant_id"`
	HasBudget           bool   `json:"has_budget"`
	GlobalDefault       bool   `json:"global_default"`
	DailyBudgetCents    int64  `json:"daily_budget_cents"`
	DailyUsedCents      int64  `json:"daily_used_cost_cents"`
	DailyUsedTokens     int64  `json:"daily_used_tokens"`
	DailyRatioPercent   int64  `json:"daily_ratio_percent"`
	MonthlyBudgetCents  int64  `json:"monthly_budget_cents"`
	MonthlyUsedCents    int64  `json:"monthly_used_cost_cents"`
	MonthlyUsedTokens   int64  `json:"monthly_used_tokens"`
	MonthlyRatioPercent int64  `json:"monthly_ratio_percent"`
	EffectiveAction     string `json:"effective_action"`
	OverLimit           bool   `json:"over_limit"`
}

// EvaluateMerchantBudget 计算商家当前日/月预算使用进度，并为已达到的阈值
// 补记预警记录（同一商家/周期/阈值只记一条）。预算口径为调用记录上
// EstimatedCostCents 的累计（CNY 分），与统计重算口径的来源不同，见文件头注释。
func (g *AICostGuard) EvaluateMerchantBudget(merchantID uint) (*MerchantAIBudgetStatus, error) {
	if g == nil || g.db == nil {
		return nil, errors.New("AI cost control database is unavailable")
	}
	budget, globalDefault, err := g.resolveMerchantBudget(merchantID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	dailyUsedCents, dailyUsedTokens, err := g.sumMerchantUsage(merchantID, dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	monthlyUsedCents, monthlyUsedTokens, err := g.sumMerchantUsage(merchantID, monthStart, monthStart.AddDate(0, 1, 0))
	if err != nil {
		return nil, err
	}
	status := &MerchantAIBudgetStatus{
		MerchantID:        merchantID,
		HasBudget:         budget != nil,
		GlobalDefault:     globalDefault,
		DailyUsedCents:    dailyUsedCents,
		DailyUsedTokens:   dailyUsedTokens,
		MonthlyUsedCents:  monthlyUsedCents,
		MonthlyUsedTokens: monthlyUsedTokens,
	}
	if budget == nil {
		return status, nil
	}
	status.DailyBudgetCents = budget.DailyBudgetCents
	status.MonthlyBudgetCents = budget.MonthlyBudgetCents
	status.EffectiveAction = budget.OverLimitAction
	status.DailyRatioPercent = budgetRatioPercent(dailyUsedCents, budget.DailyBudgetCents)
	status.MonthlyRatioPercent = budgetRatioPercent(monthlyUsedCents, budget.MonthlyBudgetCents)
	status.OverLimit = (budget.DailyBudgetCents > 0 && dailyUsedCents >= budget.DailyBudgetCents) ||
		(budget.MonthlyBudgetCents > 0 && monthlyUsedCents >= budget.MonthlyBudgetCents)
	if err := g.ensureBudgetAlerts(merchantID, "daily", dayStart, budget.DailyBudgetCents, dailyUsedCents, dailyUsedTokens); err != nil {
		return nil, err
	}
	if err := g.ensureBudgetAlerts(merchantID, "monthly", monthStart, budget.MonthlyBudgetCents, monthlyUsedCents, monthlyUsedTokens); err != nil {
		return nil, err
	}
	return status, nil
}

func budgetRatioPercent(usedCents, budgetCents int64) int64 {
	if budgetCents <= 0 {
		return 0
	}
	return usedCents * 100 / budgetCents
}

// resolveMerchantBudget 取商家预算行；无商家行时取全局默认行（merchant_id=0）；
// 都无返回 nil（不限额）。
func (g *AICostGuard) resolveMerchantBudget(merchantID uint) (*model.MerchantAIBudget, bool, error) {
	var budget model.MerchantAIBudget
	if err := g.db.Where("merchant_id = ?", merchantID).Take(&budget).Error; err == nil {
		return &budget, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	if merchantID == 0 {
		return nil, false, nil
	}
	if err := g.db.Where("merchant_id = ?", 0).Take(&budget).Error; err == nil {
		return &budget, true, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	return nil, false, nil
}

type merchantUsageSum struct {
	CostCents int64 `gorm:"column:cost_cents"`
	Tokens    int64 `gorm:"column:tokens"`
}

func (g *AICostGuard) sumMerchantUsage(merchantID uint, start, end time.Time) (int64, int64, error) {
	var row merchantUsageSum
	if err := g.db.Model(&model.MerchantAIUsageLog{}).
		Select("COALESCE(SUM(estimated_cost_cents),0) AS cost_cents, COALESCE(SUM(total_tokens),0) AS tokens").
		Where("merchant_id = ? AND used_at >= ? AND used_at < ?", merchantID, start, end).
		Scan(&row).Error; err != nil {
		return 0, 0, err
	}
	return row.CostCents, row.Tokens, nil
}

func (g *AICostGuard) ensureBudgetAlerts(merchantID uint, periodType string, periodStart time.Time, budgetCents, usedCents, usedTokens int64) error {
	if budgetCents <= 0 {
		return nil
	}
	ratio := budgetRatioPercent(usedCents, budgetCents)
	for _, threshold := range AIBudgetThresholds {
		if ratio < int64(threshold) {
			continue
		}
		alert := model.MerchantAIBudgetAlert{
			MerchantID:         merchantID,
			PeriodType:         periodType,
			PeriodStart:        model.PlatformExpenseDate(periodStart.Format("2006-01-02")),
			Threshold:          threshold,
			UsedTokens:         usedTokens,
			EstimatedCostCents: usedCents,
		}
		// 唯一键（商家/周期/起始日/阈值）保证同一阈值只记一条；并发下冲突即已记。
		if err := g.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&alert).Error; err != nil {
			return err
		}
	}
	return nil
}

func (g *AICostGuard) cheapestActiveModel(now time.Time) (string, error) {
	var prices []model.AIModelPrice
	if err := g.db.Where("is_active = ? AND currency = ? AND effective_from <= ?", true, "CNY", now).
		Find(&prices).Error; err != nil {
		return "", err
	}
	if len(prices) == 0 {
		return "", nil
	}
	// 同一模型可能有多条生效价：只取每模型最新一条，再比总价。
	latest := make(map[string]model.AIModelPrice)
	for _, price := range prices {
		if current, ok := latest[price.Model]; !ok || price.EffectiveFrom.After(current.EffectiveFrom) {
			latest[price.Model] = price
		}
	}
	names := make([]string, 0, len(latest))
	for name := range latest {
		names = append(names, name)
	}
	sort.Strings(names)
	best := ""
	var bestTotal int64 = -1
	for _, name := range names {
		price := latest[name]
		total := price.InputPriceCentsPerMillion + price.OutputPriceCentsPerMillion + price.CachedPriceCentsPerMillion
		if bestTotal < 0 || total < bestTotal {
			best, bestTotal = name, total
		}
	}
	return best, nil
}

// --- 模型价格表 ---

// EstimateAICostCents 按价格表估算一次调用的成本（CNY 分）。取调用时间
// 当时生效的最新价（同 provider 优先，否则同模型任一 provider 的最新价）；
// 无生效价时 priced=false，调用方回落到内置费率估算。
func (g *AICostGuard) EstimateAICostCents(provider, modelName string, promptTokens, completionTokens, cachedTokens int, at time.Time) (costCents int64, currency string, priced bool, err error) {
	if g == nil || g.db == nil {
		return 0, "", false, errors.New("AI cost control database is unavailable")
	}
	price, err := g.findEffectivePrice(provider, modelName, at)
	if err != nil {
		return 0, "", false, err
	}
	if price == nil {
		return 0, "", false, nil
	}
	cost := ceilTokenCost(promptTokens, price.InputPriceCentsPerMillion) +
		ceilTokenCost(completionTokens, price.OutputPriceCentsPerMillion) +
		ceilTokenCost(cachedTokens, price.CachedPriceCentsPerMillion)
	if cost < 0 {
		cost = 0
	}
	return cost, price.Currency, true, nil
}

func (g *AICostGuard) findEffectivePrice(provider, modelName string, at time.Time) (*model.AIModelPrice, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, nil
	}
	var price model.AIModelPrice
	provider = strings.TrimSpace(provider)
	if provider != "" {
		err := g.db.Where("model = ? AND provider = ? AND is_active = ? AND effective_from <= ?", modelName, provider, true, at).
			Order("effective_from DESC").Take(&price).Error
		if err == nil {
			return &price, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	err := g.db.Where("model = ? AND is_active = ? AND effective_from <= ?", modelName, true, at).
		Order("effective_from DESC").Take(&price).Error
	if err == nil {
		return &price, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

// ListModelPrices 返回价格表（最新生效价在前；含已停用行，调用方自行区分）。
func (s *AdminService) ListModelPrices() ([]model.AIModelPrice, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("AI cost control database is unavailable")
	}
	var rows []model.AIModelPrice
	if err := s.db.Order("model ASC, effective_from DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.AIModelPrice{}
	}
	return rows, nil
}

// SaveModelPrice 新增或更新一条价格。同一（模型、供应商、生效时间）只允许
// 一条：已存在则原地更新单价与停用状态（调价请用新的生效时间新增一行，
// 以保留历史估算口径）。目前只支持 CNY 计价。
func (s *AdminService) SaveModelPrice(request dto.SaveAIModelPriceRequest, actor AuditActor) (*model.AIModelPrice, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, errors.New("AI cost control database is unavailable")
	}
	entry, err := validateAIModelPriceRequest(request, actor.ID)
	if err != nil {
		return nil, false, err
	}
	if actor.ID == 0 || strings.TrimSpace(actor.Type) != "super_admin" {
		return nil, false, fmt.Errorf("%w: only super admin can maintain model prices", ErrInvalidAICostControl)
	}
	created := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.AIModelPrice
		lookupErr := tx.Where("model = ? AND provider = ? AND effective_from = ?", entry.Model, entry.Provider, entry.EffectiveFrom).Take(&existing).Error
		if lookupErr == nil {
			existing.InputPriceCentsPerMillion = entry.InputPriceCentsPerMillion
			existing.OutputPriceCentsPerMillion = entry.OutputPriceCentsPerMillion
			existing.CachedPriceCentsPerMillion = entry.CachedPriceCentsPerMillion
			existing.Currency = entry.Currency
			existing.IsActive = entry.IsActive
			existing.UpdatedBy = actor.ID
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			*entry = existing
			return appendAuditForAICostControl(tx, actor, "ai_model_price_update", "ai_model_price", entry.ID)
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		created = true
		return appendAuditForAICostControl(tx, actor, "ai_model_price_create", "ai_model_price", entry.ID)
	})
	if err != nil {
		return nil, false, err
	}
	return entry, created, nil
}

func validateAIModelPriceRequest(request dto.SaveAIModelPriceRequest, actorID uint) (*model.AIModelPrice, error) {
	if actorID == 0 {
		return nil, fmt.Errorf("%w: actor is required", ErrInvalidAICostControl)
	}
	modelName := strings.TrimSpace(request.Model)
	if modelName == "" || utf8.RuneCountInString(modelName) > 120 {
		return nil, fmt.Errorf("%w: model must contain 1 to 120 characters", ErrInvalidAICostControl)
	}
	provider := strings.TrimSpace(request.Provider)
	if utf8.RuneCountInString(provider) > 80 {
		return nil, fmt.Errorf("%w: provider must contain at most 80 characters", ErrInvalidAICostControl)
	}
	if request.InputPriceCentsPerMillion < 0 || request.InputPriceCentsPerMillion > maxAIModelPriceCents ||
		request.OutputPriceCentsPerMillion < 0 || request.OutputPriceCentsPerMillion > maxAIModelPriceCents ||
		request.CachedPriceCentsPerMillion < 0 || request.CachedPriceCentsPerMillion > maxAIModelPriceCents {
		return nil, fmt.Errorf("%w: prices must be between 0 and %d cents per million tokens", ErrInvalidAICostControl, maxAIModelPriceCents)
	}
	if strings.TrimSpace(request.Currency) != "CNY" {
		return nil, fmt.Errorf("%w: currency must be CNY (other currencies are not supported yet)", ErrInvalidAICostControl)
	}
	effectiveFrom, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(request.EffectiveFrom), time.Local)
	if err != nil {
		return nil, fmt.Errorf("%w: effective_from must use YYYY-MM-DD HH:MM:SS", ErrInvalidAICostControl)
	}
	return &model.AIModelPrice{
		Model:                      modelName,
		Provider:                   provider,
		InputPriceCentsPerMillion:  request.InputPriceCentsPerMillion,
		OutputPriceCentsPerMillion: request.OutputPriceCentsPerMillion,
		CachedPriceCentsPerMillion: request.CachedPriceCentsPerMillion,
		Currency:                   "CNY",
		EffectiveFrom:              effectiveFrom,
		CreatedBy:                  actorID,
		UpdatedBy:                  actorID,
		IsActive:                   request.IsActive,
	}, nil
}

// --- 供应商实际账单 ---

// CreateAIVendorBill 手工录入一条供应商实际账单。同一幂等键+相同内容重放
// 不重复建账；同一幂等键不同内容直接拒绝。
func (s *AdminService) CreateAIVendorBill(request dto.CreateAIVendorBillRequest, actor AuditActor) (*model.AIVendorBill, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, errors.New("AI cost control database is unavailable")
	}
	entry, err := validateAIVendorBillRequest(request, actor.ID)
	if err != nil {
		return nil, false, err
	}
	if actor.ID == 0 || strings.TrimSpace(actor.Type) != "super_admin" {
		return nil, false, fmt.Errorf("%w: only super admin can record vendor bills", ErrInvalidAICostControl)
	}
	created := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.AIVendorBill
		lookupErr := tx.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error
		if lookupErr == nil {
			if !sameAIVendorBillPayload(existing, *entry) {
				return ErrAIVendorBillKeyConflict
			}
			*entry = existing
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		created = true
		return appendAuditForAICostControl(tx, actor, "ai_vendor_bill_create", "ai_vendor_bill", entry.ID)
	})
	if err == nil {
		return entry, created, nil
	}
	if errors.Is(err, ErrAIVendorBillKeyConflict) {
		return nil, false, err
	}
	var existing model.AIVendorBill
	if lookupErr := s.db.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error; lookupErr == nil {
		if sameAIVendorBillPayload(existing, *entry) {
			return &existing, false, nil
		}
		return nil, false, ErrAIVendorBillKeyConflict
	}
	return nil, false, err
}

func validateAIVendorBillRequest(request dto.CreateAIVendorBillRequest, actorID uint) (*model.AIVendorBill, error) {
	if actorID == 0 {
		return nil, fmt.Errorf("%w: actor is required", ErrInvalidAICostControl)
	}
	vendor := strings.TrimSpace(request.Vendor)
	if vendor == "" || utf8.RuneCountInString(vendor) > 120 {
		return nil, fmt.Errorf("%w: vendor must contain 1 to 120 characters", ErrInvalidAICostControl)
	}
	periodStart, err := time.Parse("2006-01-02", strings.TrimSpace(request.PeriodStart))
	if err != nil || periodStart.Format("2006-01-02") != strings.TrimSpace(request.PeriodStart) {
		return nil, fmt.Errorf("%w: period_start must use YYYY-MM-DD", ErrInvalidAICostControl)
	}
	periodEnd, err := time.Parse("2006-01-02", strings.TrimSpace(request.PeriodEnd))
	if err != nil || periodEnd.Format("2006-01-02") != strings.TrimSpace(request.PeriodEnd) {
		return nil, fmt.Errorf("%w: period_end must use YYYY-MM-DD", ErrInvalidAICostControl)
	}
	if periodEnd.Before(periodStart) {
		return nil, fmt.Errorf("%w: period_end precedes period_start", ErrInvalidAICostControl)
	}
	if periodEnd.Format("2006-01-02") > time.Now().In(time.Local).Format("2006-01-02") {
		return nil, fmt.Errorf("%w: period_end cannot be in the future", ErrInvalidAICostControl)
	}
	if request.AmountCents <= 0 || request.AmountCents > maxAIVendorBillCents {
		return nil, fmt.Errorf("%w: amount_cents must be between 1 and %d", ErrInvalidAICostControl, maxAIVendorBillCents)
	}
	if strings.TrimSpace(request.Currency) != "CNY" {
		return nil, fmt.Errorf("%w: currency must be CNY (other currencies are not supported yet)", ErrInvalidAICostControl)
	}
	note := strings.TrimSpace(request.Note)
	if utf8.RuneCountInString(note) > maxAICostNoteRunes {
		return nil, fmt.Errorf("%w: note must contain at most %d characters", ErrInvalidAICostControl, maxAICostNoteRunes)
	}
	for _, r := range note {
		if unicode.IsControl(r) {
			return nil, fmt.Errorf("%w: note cannot contain control characters", ErrInvalidAICostControl)
		}
	}
	key, err := uuid.Parse(strings.TrimSpace(request.IdempotencyKey))
	if err != nil || key.String() == uuid.Nil.String() {
		return nil, fmt.Errorf("%w: idempotency_key must be a non-zero UUID", ErrInvalidAICostControl)
	}
	return &model.AIVendorBill{
		Vendor:         vendor,
		PeriodStart:    model.PlatformExpenseDate(periodStart.Format("2006-01-02")),
		PeriodEnd:      model.PlatformExpenseDate(periodEnd.Format("2006-01-02")),
		AmountCents:    request.AmountCents,
		Currency:       "CNY",
		Note:           note,
		CreatedBy:      actorID,
		IdempotencyKey: key.String(),
	}, nil
}

func sameAIVendorBillPayload(existing, candidate model.AIVendorBill) bool {
	return existing.Vendor == candidate.Vendor && existing.PeriodStart == candidate.PeriodStart &&
		existing.PeriodEnd == candidate.PeriodEnd && existing.AmountCents == candidate.AmountCents &&
		existing.Currency == candidate.Currency && existing.Note == candidate.Note && existing.CreatedBy == candidate.CreatedBy
}

// ListAIVendorBills 按账单周期倒序列出供应商实际账单。
func (s *AdminService) ListAIVendorBills(vendor string, startDate, endDate time.Time) ([]model.AIVendorBill, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("AI cost control database is unavailable")
	}
	query := s.db.Model(&model.AIVendorBill{})
	if strings.TrimSpace(vendor) != "" {
		query = query.Where("vendor = ?", strings.TrimSpace(vendor))
	}
	if !startDate.IsZero() {
		query = query.Where("period_end >= ?", startDate.Format("2006-01-02"))
	}
	if !endDate.IsZero() {
		query = query.Where("period_start <= ?", endDate.Format("2006-01-02"))
	}
	var rows []model.AIVendorBill
	if err := query.Order("period_start DESC, id DESC").Limit(500).Find(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.AIVendorBill{}
	}
	return rows, nil
}

// AIBillReconciliation 是估算成本与实际账单的核对结果。两者口径不同：
// 估算成本 = 调用记录 × 价格表（模型估算）；实际账单 = 超管手工录入的供应商
// 真实金额。差额（实际 − 估算）仅供核对参考，不代表欠款或退款。
type AIBillReconciliation struct {
	PeriodStart            string               `json:"period_start"`
	PeriodEnd              string               `json:"period_end"`
	EstimatedCostCents     int64                `json:"estimated_cost_cents"`
	EstimatedCostSource    string               `json:"estimated_cost_source"`
	ActualBillTotalCents   int64                `json:"actual_bill_total_cents"`
	ActualBillCount        int64                `json:"actual_bill_count"`
	DifferenceCents        int64                `json:"difference_cents_actual_minus_estimated"`
	Bills                  []model.AIVendorBill `json:"bills"`
	UnpricedEstimatedCents int64                `json:"unpriced_fallback_estimated_cost_cents"`
	Note                   string               `json:"note"`
}

// ReconcileAIBills 按周期汇总估算成本与实际账单并算差额。
func (s *AdminService) ReconcileAIBills(startDate, endDate time.Time) (*AIBillReconciliation, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("AI cost control database is unavailable")
	}
	start, end, err := normalizeAICostPeriod(startDate, endDate)
	if err != nil {
		return nil, err
	}
	stats, err := NewAICostGuard(s.db).AIUsageStats(start, end)
	if err != nil {
		return nil, err
	}
	bills, err := s.ListAIVendorBills("", start, end.AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}
	var actualTotal int64
	for _, bill := range bills {
		actualTotal += bill.AmountCents
	}
	return &AIBillReconciliation{
		PeriodStart:            start.Format("2006-01-02"),
		PeriodEnd:              end.AddDate(0, 0, -1).Format("2006-01-02"),
		EstimatedCostCents:     stats.TotalEstimatedCostCents,
		EstimatedCostSource:    stats.EstimatedCostSource,
		ActualBillTotalCents:   actualTotal,
		ActualBillCount:        int64(len(bills)),
		DifferenceCents:        actualTotal - stats.TotalEstimatedCostCents,
		Bills:                  bills,
		UnpricedEstimatedCents: stats.UnpricedFallbackEstimatedCents,
		Note:                   "估算成本来自调用记录×价格表（模型估算），实际账单为超管手工录入的供应商真实金额；两者口径不同，差额仅供核对参考，不代表欠款或退款。",
	}, nil
}

func normalizeAICostPeriod(startDate, endDate time.Time) (time.Time, time.Time, error) {
	if startDate.IsZero() || endDate.IsZero() {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: start_date and end_date are required", ErrInvalidAICostControl)
	}
	start := time.Date(startDate.In(time.Local).Year(), startDate.In(time.Local).Month(), startDate.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
	endDay := endDate.In(time.Local)
	end := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), 0, 0, 0, 0, time.Local)
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: end_date precedes start_date", ErrInvalidAICostControl)
	}
	today := time.Now().In(time.Local)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	if end.After(today) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: end_date cannot be in the future", ErrInvalidAICostControl)
	}
	endExclusive := end.AddDate(0, 0, 1)
	if endExclusive.After(start.AddDate(0, 0, 366)) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: period cannot exceed 366 days", ErrInvalidAICostControl)
	}
	return start, endExclusive, nil
}

// --- 商家预算 ---

// SaveMerchantBudget 设置某商家（或 merchant_id=0 的全局默认）的日/月预算
// 与超额策略。金额为分，0 表示该周期不限额。
func (s *AdminService) SaveMerchantBudget(merchantID uint, request dto.SaveMerchantAIBudgetRequest, actor AuditActor) (*model.MerchantAIBudget, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("AI cost control database is unavailable")
	}
	if actor.ID == 0 || strings.TrimSpace(actor.Type) != "super_admin" {
		return nil, fmt.Errorf("%w: only super admin can set AI budgets", ErrInvalidAICostControl)
	}
	action := strings.TrimSpace(request.OverLimitAction)
	switch action {
	case model.AIBudgetOverLimitStop, model.AIBudgetOverLimitDowngrade, model.AIBudgetOverLimitRestrictExpensive, model.AIBudgetOverLimitManualReview:
	default:
		return nil, fmt.Errorf("%w: unsupported over_limit_action", ErrInvalidAICostControl)
	}
	if request.DailyBudgetCents < 0 || request.DailyBudgetCents > maxAIBudgetCents ||
		request.MonthlyBudgetCents < 0 || request.MonthlyBudgetCents > maxAIBudgetCents {
		return nil, fmt.Errorf("%w: budgets must be between 0 and %d cents (0 means unlimited)", ErrInvalidAICostControl, maxAIBudgetCents)
	}
	if merchantID != 0 {
		var merchant model.Merchant
		if err := s.db.Select("id").First(&merchant, merchantID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("%w: merchant was not found", ErrInvalidAICostControl)
			}
			return nil, err
		}
	}
	var entry model.MerchantAIBudget
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.MerchantAIBudget
		lookupErr := tx.Where("merchant_id = ?", merchantID).Take(&existing).Error
		if lookupErr == nil {
			existing.DailyBudgetCents = request.DailyBudgetCents
			existing.MonthlyBudgetCents = request.MonthlyBudgetCents
			existing.OverLimitAction = action
			existing.UpdatedBy = actor.ID
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			entry = existing
			return appendAuditForAICostControl(tx, actor, "ai_budget_update", "merchant_ai_budget", entry.ID)
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		entry = model.MerchantAIBudget{
			MerchantID:         merchantID,
			DailyBudgetCents:   request.DailyBudgetCents,
			MonthlyBudgetCents: request.MonthlyBudgetCents,
			OverLimitAction:    action,
			UpdatedBy:          actor.ID,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		return appendAuditForAICostControl(tx, actor, "ai_budget_create", "merchant_ai_budget", entry.ID)
	})
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// ListMerchantBudgets 列出全部商家预算行（含 merchant_id=0 的全局默认行）。
func (s *AdminService) ListMerchantBudgets() ([]model.MerchantAIBudget, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("AI cost control database is unavailable")
	}
	var rows []model.MerchantAIBudget
	if err := s.db.Order("merchant_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.MerchantAIBudget{}
	}
	return rows, nil
}

// MerchantBudgetStatus 返回某商家的预算行（商家行优先，否则全局默认行）与
// 当前日/月使用快照；读取时会补记已达到阈值的预警记录。
func (s *AdminService) MerchantBudgetStatus(merchantID uint) (*MerchantAIBudgetStatus, *model.MerchantAIBudget, error) {
	if s == nil || s.db == nil {
		return nil, nil, errors.New("AI cost control database is unavailable")
	}
	guard := NewAICostGuard(s.db)
	status, err := guard.EvaluateMerchantBudget(merchantID)
	if err != nil {
		return nil, nil, err
	}
	budget, _, err := guard.resolveMerchantBudget(merchantID)
	if err != nil {
		return nil, nil, err
	}
	return status, budget, nil
}

// ListBudgetAlerts 查询预警记录（可按商家/周期类型/阈值/日期范围筛选）。
func (s *AdminService) ListBudgetAlerts(merchantID uint, periodType string, threshold int, startDate, endDate time.Time) ([]model.MerchantAIBudgetAlert, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("AI cost control database is unavailable")
	}
	query := s.db.Model(&model.MerchantAIBudgetAlert{})
	if merchantID != 0 {
		query = query.Where("merchant_id = ?", merchantID)
	}
	if strings.TrimSpace(periodType) != "" {
		query = query.Where("period_type = ?", strings.TrimSpace(periodType))
	}
	if threshold != 0 {
		query = query.Where("threshold = ?", threshold)
	}
	if !startDate.IsZero() {
		query = query.Where("period_start >= ?", startDate.Format("2006-01-02"))
	}
	if !endDate.IsZero() {
		query = query.Where("period_start <= ?", endDate.Format("2006-01-02"))
	}
	var rows []model.MerchantAIBudgetAlert
	if err := query.Order("period_start DESC, id DESC").Limit(aiBudgetAlertListLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.MerchantAIBudgetAlert{}
	}
	return rows, nil
}

// --- 紧急关闭开关 ---

// GetAIEmergencySwitch 返回全局开关状态；表里无行（旧 AutoMigrate 库未种子）
// 时视为未关闭，不报错。
func (g *AICostGuard) GetAIEmergencySwitch() (bool, *model.AIEmergencySwitch, error) {
	if g == nil || g.db == nil {
		return false, nil, errors.New("AI cost control database is unavailable")
	}
	var row model.AIEmergencySwitch
	if err := g.db.First(&row, 1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, &model.AIEmergencySwitch{ID: 1, Disabled: false}, nil
		}
		return false, nil, err
	}
	return row.Disabled, &row, nil
}

// SetAIEmergencySwitch 设置全局开关并记审计。关闭后 AI 调用直接拒绝，
// 开启后恢复；变更人与原因随审计记录。
func (s *AdminService) SetAIEmergencySwitch(disabled bool, reason string, actor AuditActor) (*model.AIEmergencySwitch, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("AI cost control database is unavailable")
	}
	if actor.ID == 0 || strings.TrimSpace(actor.Type) != "super_admin" {
		return nil, fmt.Errorf("%w: only super admin can change the AI emergency switch", ErrInvalidAICostControl)
	}
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > maxAICostNoteRunes {
		return nil, fmt.Errorf("%w: reason must contain at most %d characters", ErrInvalidAICostControl, maxAICostNoteRunes)
	}
	for _, r := range reason {
		if unicode.IsControl(r) {
			return nil, fmt.Errorf("%w: reason cannot contain control characters", ErrInvalidAICostControl)
		}
	}
	var row model.AIEmergencySwitch
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.AIEmergencySwitch
		lookupErr := tx.First(&existing, 1).Error
		if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			row = model.AIEmergencySwitch{ID: 1, Disabled: disabled, Reason: reason, UpdatedBy: actor.ID}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else {
			existing.Disabled = disabled
			existing.Reason = reason
			existing.UpdatedBy = actor.ID
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			row = existing
		}
		action := "ai_emergency_switch_enable"
		if disabled {
			action = "ai_emergency_switch_disable"
		}
		return appendAuditForAICostControl(tx, actor, action, "ai_emergency_switch", 1)
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func appendAuditForAICostControl(tx *gorm.DB, actor AuditActor, action, targetType string, targetID uint) error {
	return AppendAuditTx(tx, AuditEvent{
		Actor: actor,
		Input: AuditRecordInput{Action: action, TargetType: targetType, TargetID: targetID},
	})
}

// --- 使用量统计 ---

// AIUsageGroupStat 是一组调用记录的聚合：调用次数、输入/输出/缓存/总 Token、
// 估算成本（CNY 分，价格表重算口径）。
type AIUsageGroupStat struct {
	MerchantID         uint   `json:"merchant_id,omitempty"`
	MerchantName       string `json:"merchant_name,omitempty"`
	StoreID            uint   `json:"store_id,omitempty"`
	Model              string `json:"model,omitempty"`
	Scenario           string `json:"scenario,omitempty"`
	RequestCount       int64  `json:"request_count"`
	SuccessCount       int64  `json:"success_count"`
	FailedCount        int64  `json:"failed_count"`
	PromptTokens       int64  `json:"prompt_tokens"`
	CompletionTokens   int64  `json:"completion_tokens"`
	CachedTokens       int64  `json:"cached_tokens"`
	TotalTokens        int64  `json:"total_tokens"`
	EstimatedCostCents int64  `json:"estimated_cost_cents"`
}

// AIUsageStats 是按商家/门店/模型/功能四维聚合的使用量统计，全部基于现有的
// MerchantAIUsageLog，不另起记录。估算成本按调用时间的生效价重算；
// 无生效价的调用回落到记录上的 EstimatedCostCents，金额汇总进
// UnpricedFallbackEstimatedCents 以便核对。
type AIUsageStats struct {
	PeriodStart                    string             `json:"period_start"`
	PeriodEnd                      string             `json:"period_end"`
	TotalRequests                  int64              `json:"total_requests"`
	TotalPromptTokens              int64              `json:"total_prompt_tokens"`
	TotalCompletionTokens          int64              `json:"total_completion_tokens"`
	TotalCachedTokens              int64              `json:"total_cached_tokens"`
	TotalTokens                    int64              `json:"total_tokens"`
	TotalEstimatedCostCents        int64              `json:"total_estimated_cost_cents"`
	UnpricedFallbackEstimatedCents int64              `json:"unpriced_fallback_estimated_cost_cents"`
	EstimatedCostSource            string             `json:"estimated_cost_source"`
	ByMerchant                     []AIUsageGroupStat `json:"by_merchant"`
	ByStore                        []AIUsageGroupStat `json:"by_store"`
	ByModel                        []AIUsageGroupStat `json:"by_model"`
	ByScenario                     []AIUsageGroupStat `json:"by_scenario"`
}

type aiUsageAggregateRow struct {
	MerchantID       uint   `gorm:"column:merchant_id"`
	StoreID          *uint  `gorm:"column:store_id"`
	Model            string `gorm:"column:model"`
	Scenario         string `gorm:"column:scenario"`
	Day              string `gorm:"column:day"`
	RequestCount     int64  `gorm:"column:request_count"`
	SuccessCount     int64  `gorm:"column:success_count"`
	FailedCount      int64  `gorm:"column:failed_count"`
	PromptTokens     int64  `gorm:"column:prompt_tokens"`
	CompletionTokens int64  `gorm:"column:completion_tokens"`
	CachedTokens     int64  `gorm:"column:cached_tokens"`
	TotalTokens      int64  `gorm:"column:total_tokens"`
	StoredCostCents  int64  `gorm:"column:stored_cost_cents"`
}

// AIUsageStats 聚合指定周期（左闭右开）的 AI 使用记录。
func (g *AICostGuard) AIUsageStats(start, end time.Time) (*AIUsageStats, error) {
	if g == nil || g.db == nil {
		return nil, errors.New("AI cost control database is unavailable")
	}
	var rows []aiUsageAggregateRow
	// SQLite 与 MySQL 的日期函数不同：用 GORM 方言判断。
	dayExpr := "DATE(used_at)"
	if g.db.Dialector.Name() == "sqlite" {
		dayExpr = "date(used_at)"
	}
	if err := g.db.Model(&model.MerchantAIUsageLog{}).
		Select("merchant_id, store_id, model, scenario, "+dayExpr+" AS day, COUNT(*) AS request_count, COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END),0) AS success_count, COALESCE(SUM(CASE WHEN success THEN 0 ELSE 1 END),0) AS failed_count, COALESCE(SUM(prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(completion_tokens),0) AS completion_tokens, COALESCE(SUM(cached_tokens),0) AS cached_tokens, COALESCE(SUM(total_tokens),0) AS total_tokens, COALESCE(SUM(estimated_cost_cents),0) AS stored_cost_cents").
		Where("used_at >= ? AND used_at < ?", start, end).
		Group("merchant_id, store_id, model, scenario, day").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	merchantNames := make(map[uint]string)
	var merchants []model.Merchant
	if err := g.db.Model(&model.Merchant{}).Select("id, name").Find(&merchants).Error; err == nil {
		for _, merchant := range merchants {
			merchantNames[merchant.ID] = merchant.Name
		}
	}
	stats := &AIUsageStats{
		PeriodStart:         start.Format("2006-01-02"),
		PeriodEnd:           end.AddDate(0, 0, -1).Format("2006-01-02"),
		EstimatedCostSource: "调用记录×模型价格表重算（按调用日期取当日最晚生效价）；无生效价的调用回落到记录上的估算成本",
		ByMerchant:          []AIUsageGroupStat{},
		ByStore:             []AIUsageGroupStat{},
		ByModel:             []AIUsageGroupStat{},
		ByScenario:          []AIUsageGroupStat{},
	}
	byMerchant := make(map[uint]*AIUsageGroupStat)
	byStore := make(map[uint]*AIUsageGroupStat)
	byModel := make(map[string]*AIUsageGroupStat)
	byScenario := make(map[string]*AIUsageGroupStat)
	accumulate := func(target *AIUsageGroupStat, row aiUsageAggregateRow, costCents int64) {
		target.RequestCount += row.RequestCount
		target.SuccessCount += row.SuccessCount
		target.FailedCount += row.FailedCount
		target.PromptTokens += row.PromptTokens
		target.CompletionTokens += row.CompletionTokens
		target.CachedTokens += row.CachedTokens
		target.TotalTokens += row.TotalTokens
		target.EstimatedCostCents += costCents
	}
	for _, row := range rows {
		day, parseErr := time.ParseInLocation("2006-01-02", row.Day, time.Local)
		if parseErr != nil {
			day = start
		}
		cost, _, priced, err := g.EstimateAICostCents("", row.Model, int(row.PromptTokens), int(row.CompletionTokens), int(row.CachedTokens), day.Add(23*time.Hour+59*time.Minute+59*time.Second))
		if err != nil {
			return nil, err
		}
		if !priced {
			cost = row.StoredCostCents
			stats.UnpricedFallbackEstimatedCents += row.StoredCostCents
		}
		merchantStat, ok := byMerchant[row.MerchantID]
		if !ok {
			merchantStat = &AIUsageGroupStat{MerchantID: row.MerchantID, MerchantName: merchantNames[row.MerchantID]}
			byMerchant[row.MerchantID] = merchantStat
		}
		accumulate(merchantStat, row, cost)
		storeID := uint(0)
		if row.StoreID != nil {
			storeID = *row.StoreID
		}
		storeStat, ok := byStore[storeID]
		if !ok {
			storeStat = &AIUsageGroupStat{StoreID: storeID}
			byStore[storeID] = storeStat
		}
		accumulate(storeStat, row, cost)
		modelStat, ok := byModel[row.Model]
		if !ok {
			modelStat = &AIUsageGroupStat{Model: row.Model}
			byModel[row.Model] = modelStat
		}
		accumulate(modelStat, row, cost)
		scenarioStat, ok := byScenario[row.Scenario]
		if !ok {
			scenarioStat = &AIUsageGroupStat{Scenario: row.Scenario}
			byScenario[row.Scenario] = scenarioStat
		}
		accumulate(scenarioStat, row, cost)
		stats.TotalRequests += row.RequestCount
		stats.TotalPromptTokens += row.PromptTokens
		stats.TotalCompletionTokens += row.CompletionTokens
		stats.TotalCachedTokens += row.CachedTokens
		stats.TotalTokens += row.TotalTokens
		stats.TotalEstimatedCostCents += cost
	}
	stats.ByMerchant = sortedUsageStats(mapValues(byMerchant))
	stats.ByStore = sortedUsageStats(mapValues(byStore))
	stats.ByModel = sortedUsageStats(mapValues(byModel))
	stats.ByScenario = sortedUsageStats(mapValues(byScenario))
	return stats, nil
}

func mapValues[K comparable](groups map[K]*AIUsageGroupStat) []*AIUsageGroupStat {
	out := make([]*AIUsageGroupStat, 0, len(groups))
	for _, stat := range groups {
		out = append(out, stat)
	}
	return out
}

func sortedUsageStats(groups []*AIUsageGroupStat) []AIUsageGroupStat {
	out := make([]AIUsageGroupStat, 0, len(groups))
	for _, stat := range groups {
		out = append(out, *stat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EstimatedCostCents > out[j].EstimatedCostCents })
	return out
}
