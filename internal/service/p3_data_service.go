package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

// P3 数据底座（P2 建表，P3 经营看板直接读取）：活动目标、活动复盘快照、
// AI 建议执行跟踪、抖音团购手工核销录入。口径红线：
//   - 目标是人工设定值；复盘实际值按统一交易口径统计
//     （paid_at 不为空且金额大于 0），生成时落库快照，不现场重算；
//   - 新增顾客指周期内首次出现有效交易的手机号（周期前无有效交易）；
//   - 手工核销数是人工事实陈述，只能与链接点击数对照参考，绝不可说成
//     平台验证的核销。

// --- 活动目标 ---

// CreateCampaignTarget creates a target for one merchant's campaign in one
// period. The (merchant, campaign, period_start) scope is unique: a second
// create for the same scope is a conflict, use UpdateCampaignTarget.
func (s *AdminService) CreateCampaignTarget(merchantID uint, req dto.CreateCampaignTargetRequest, actor AuditActor) (*model.CampaignTarget, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("campaign database is unavailable")
	}
	if merchantID == 0 {
		return nil, errors.New("merchant id is required")
	}
	start, err := parseBeijingDate(req.PeriodStart)
	if err != nil {
		return nil, err
	}
	end, err := parseBeijingDate(req.PeriodEnd)
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, errors.New("目标周期结束日期不能早于开始日期")
	}
	if err := s.checkCampaignExists(merchantID, req.CampaignType, req.CampaignID); err != nil {
		return nil, err
	}
	entry := &model.CampaignTarget{
		MerchantID:         merchantID,
		CampaignType:       strings.TrimSpace(req.CampaignType),
		CampaignID:         req.CampaignID,
		TargetOrders:       req.TargetOrders,
		TargetAmountCents:  req.TargetAmountCents,
		TargetNewCustomers: req.TargetNewCustomers,
		PeriodStart:        model.PlatformExpenseDate(req.PeriodStart),
		PeriodEnd:          model.PlatformExpenseDate(req.PeriodEnd),
		CreatedBy:          actor.ID,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entry).Error; err != nil {
			if isUniqueViolation(err) {
				return errors.New("该活动该周期已有目标，请修改已有目标")
			}
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "campaign_target_create", "campaign_target", entry.ID)
	}); err != nil {
		return nil, err
	}
	return entry, nil
}

// UpdateCampaignTarget adjusts the target numbers of an existing target.
// The scope (merchant, campaign, period) never changes.
func (s *AdminService) UpdateCampaignTarget(id uint, req dto.CreateCampaignTargetRequest, actor AuditActor) (*model.CampaignTarget, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("campaign database is unavailable")
	}
	var entry model.CampaignTarget
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&entry, id).Error; err != nil {
			return err
		}
		entry.TargetOrders = req.TargetOrders
		entry.TargetAmountCents = req.TargetAmountCents
		entry.TargetNewCustomers = req.TargetNewCustomers
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "campaign_target_update", "campaign_target", entry.ID)
	}); err != nil {
		return nil, err
	}
	return &entry, nil
}

// ListCampaignTargets lists a merchant's targets, newest period first.
func (s *AdminService) ListCampaignTargets(merchantID uint, campaignType string, limit, offset int) ([]model.CampaignTarget, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("campaign database is unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := s.db.Model(&model.CampaignTarget{}).Where("merchant_id = ?", merchantID)
	if campaignType != "" {
		query = query.Where("campaign_type = ?", campaignType)
	}
	var rows []model.CampaignTarget
	if err := query.Order("period_start DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *AdminService) checkCampaignExists(merchantID uint, campaignType string, campaignID uint) error {
	switch campaignType {
	case model.CampaignTypePromotion:
		var row model.Promotion
		if err := s.db.Where("id = ? AND merchant_id = ?", campaignID, merchantID).First(&row).Error; err != nil {
			return errors.New("活动不存在或不属于该商家")
		}
	case model.CampaignTypeShareCampaign:
		var row model.ShareCampaign
		if err := s.db.Where("id = ? AND merchant_id = ?", campaignID, merchantID).First(&row).Error; err != nil {
			return errors.New("活动不存在或不属于该商家")
		}
	default:
		return errors.New("活动类型无效")
	}
	return nil
}

// --- 活动复盘快照 ---

// CreateCampaignPostmortem generates a postmortem snapshot: the target values
// are copied from the merchant's target for the same scope (0 when no target
// was set), and the actuals are computed once from orders under the frozen
// trade definition and stored. The snapshot never changes afterwards, so the
// P3 dashboard reads history instead of recomputing it.
func (s *AdminService) CreateCampaignPostmortem(merchantID uint, req dto.CreateCampaignPostmortemRequest, actor AuditActor) (*model.CampaignPostmortem, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("campaign database is unavailable")
	}
	if merchantID == 0 {
		return nil, errors.New("merchant id is required")
	}
	start, err := parseBeijingDate(req.PeriodStart)
	if err != nil {
		return nil, err
	}
	end, err := parseBeijingDate(req.PeriodEnd)
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, errors.New("复盘周期结束日期不能早于开始日期")
	}
	if err := s.checkCampaignExists(merchantID, req.CampaignType, req.CampaignID); err != nil {
		return nil, err
	}
	generatedBy := strings.TrimSpace(req.GeneratedBy)
	if generatedBy == "" {
		generatedBy = model.SuggestionGeneratedBySystem
	}
	actualOrders, actualAmount, actualNewCustomers, err := s.computeCampaignActuals(merchantID, start, end)
	if err != nil {
		return nil, err
	}
	var targetOrders, targetAmount int64
	var target model.CampaignTarget
	if err := s.db.Where("merchant_id = ? AND campaign_type = ? AND campaign_id = ? AND period_start = ?",
		merchantID, req.CampaignType, req.CampaignID, req.PeriodStart).First(&target).Error; err == nil {
		targetOrders = target.TargetOrders
		targetAmount = target.TargetAmountCents
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	entry := &model.CampaignPostmortem{
		MerchantID:         merchantID,
		CampaignType:       strings.TrimSpace(req.CampaignType),
		CampaignID:         req.CampaignID,
		PeriodStart:        model.PlatformExpenseDate(req.PeriodStart),
		PeriodEnd:          model.PlatformExpenseDate(req.PeriodEnd),
		TargetOrders:       targetOrders,
		TargetAmountCents:  targetAmount,
		ActualOrders:       actualOrders,
		ActualAmountCents:  actualAmount,
		ActualNewCustomers: actualNewCustomers,
		Conclusion:         strings.TrimSpace(req.Conclusion),
		GeneratedBy:        generatedBy,
		GeneratedAt:        time.Now(),
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "campaign_postmortem_create", "campaign_postmortem", entry.ID)
	}); err != nil {
		return nil, err
	}
	return entry, nil
}

// ListCampaignPostmortems lists a merchant's postmortem snapshots, newest first.
func (s *AdminService) ListCampaignPostmortems(merchantID uint, campaignType string, limit, offset int) ([]model.CampaignPostmortem, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("campaign database is unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := s.db.Model(&model.CampaignPostmortem{}).Where("merchant_id = ?", merchantID)
	if campaignType != "" {
		query = query.Where("campaign_type = ?", campaignType)
	}
	var rows []model.CampaignPostmortem
	if err := query.Order("period_start DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// computeCampaignActuals counts valid trades (frozen definition) paid inside
// [start, end] and the new customers among them: phones with a valid trade
// in the period and no valid trade before the period. Empty phones never
// count as customers.
func (s *AdminService) computeCampaignActuals(merchantID uint, start, end time.Time) (orders int64, amount int64, newCustomers int64, err error) {
	periodEndExclusive := end.AddDate(0, 0, 1)
	type tradeAgg struct {
		Orders int64 `gorm:"column:orders"`
		Amount int64 `gorm:"column:amount"`
	}
	var agg tradeAgg
	tradeErr := s.db.Model(&model.Order{}).
		Select("COUNT(*) AS orders, COALESCE(SUM(total_amount),0) AS amount").
		Where("merchant_id = ? AND order_type = ? AND "+ValidTradeAmountCondition, merchantID, "store_order").
		Where("paid_at >= ? AND paid_at < ?", start, periodEndExclusive).
		Scan(&agg).Error
	if tradeErr != nil {
		return 0, 0, 0, tradeErr
	}
	var customers int64
	customerErr := s.db.Model(&model.Order{}).
		Select("COUNT(DISTINCT customer_phone)").
		Where("merchant_id = ? AND order_type = ? AND "+ValidTradeAmountCondition, merchantID, "store_order").
		Where("paid_at >= ? AND paid_at < ?", start, periodEndExclusive).
		Where("customer_phone <> ''").
		Where("customer_phone NOT IN (?)", s.db.Model(&model.Order{}).
			Select("DISTINCT customer_phone").
			Where("merchant_id = ? AND order_type = ? AND "+ValidTradeAmountCondition, merchantID, "store_order").
			Where("paid_at < ?", start).
			Where("customer_phone <> ''")).
		Scan(&customers).Error
	if customerErr != nil {
		return 0, 0, 0, customerErr
	}
	return agg.Orders, agg.Amount, customers, nil
}

// --- AI 建议执行跟踪 ---

// RecordAISuggestionExecution records one AI suggestion (title + content
// snapshot) for a merchant so its adoption can be tracked. The suggestion
// itself is generated by the AI centre; this only opens the tracking loop.
func (s *AdminService) RecordAISuggestionExecution(merchantID uint, req dto.RecordAISuggestionExecutionRequest, actor AuditActor) (*model.AISuggestionExecution, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("suggestion database is unavailable")
	}
	if merchantID == 0 {
		return nil, errors.New("merchant id is required")
	}
	entry := &model.AISuggestionExecution{
		MerchantID:      merchantID,
		StoreID:         req.StoreID,
		Source:          strings.TrimSpace(req.Source),
		Title:           strings.TrimSpace(req.Title),
		ContentSnapshot: strings.TrimSpace(req.ContentSnapshot),
		Status:          model.SuggestionExecutionPending,
		UpdatedBy:       actor.ID,
	}
	if entry.Source == "" || entry.Title == "" {
		return nil, errors.New("建议来源与标题不能为空")
	}
	if err := s.db.Create(entry).Error; err != nil {
		return nil, err
	}
	return entry, nil
}

// ListAISuggestionExecutions lists a merchant's suggestion executions, newest first.
func (s *AdminService) ListAISuggestionExecutions(merchantID uint, status string, limit, offset int) ([]model.AISuggestionExecution, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("suggestion database is unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := s.db.Model(&model.AISuggestionExecution{}).Where("merchant_id = ?", merchantID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var rows []model.AISuggestionExecution
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpdateAISuggestionExecution moves a suggestion along pending → adopted /
// ignored, adopted → executed. Executed and ignored are terminal. Executing
// stamps executed_at and may link the order / coupon the suggestion produced.
func (s *AdminService) UpdateAISuggestionExecution(id uint, req dto.UpdateAISuggestionExecutionRequest, actor AuditActor) (*model.AISuggestionExecution, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("suggestion database is unavailable")
	}
	target := strings.TrimSpace(req.Status)
	var entry model.AISuggestionExecution
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&entry, id).Error; err != nil {
			return err
		}
		switch entry.Status {
		case model.SuggestionExecutionPending:
			if target != model.SuggestionExecutionAdopted && target != model.SuggestionExecutionIgnored {
				return fmt.Errorf("待处理建议只能改为已采纳或已忽略，不能改为 %s", target)
			}
		case model.SuggestionExecutionAdopted:
			if target != model.SuggestionExecutionExecuted {
				return fmt.Errorf("已采纳建议只能改为已执行，不能改为 %s", target)
			}
		default:
			return fmt.Errorf("已%s的建议不可再改", entry.Status)
		}
		entry.Status = target
		if target == model.SuggestionExecutionExecuted {
			entry.ExecutedAt = &now
		}
		if req.RelatedOrderID != nil {
			entry.RelatedOrderID = req.RelatedOrderID
		}
		if req.RelatedCouponID != nil {
			entry.RelatedCouponID = req.RelatedCouponID
		}
		if note := strings.TrimSpace(req.EffectNote); note != "" {
			entry.EffectNote = note
		}
		entry.UpdatedBy = actor.ID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "suggestion_execution_"+target, "ai_suggestion_execution", entry.ID)
	}); err != nil {
		return nil, err
	}
	return &entry, nil
}

// --- 抖音团购手工核销录入 ---

// GroupBuyManualRedemptionList is a merchant's manual redemption rows plus
// the totals over the returned scope, so the P3 dashboard can show the
// manual numbers next to link clicks without recomputing them.
type GroupBuyManualRedemptionList struct {
	Rows                []model.GroupBuyManualRedemption `json:"rows"`
	TotalCount          int64                           `json:"total_count"`
	TotalAmountCents    int64                           `json:"total_amount_cents"`
	SourceNote          string                          `json:"source_note"`
}

// CreateGroupBuyManualRedemption records one manual redemption entry: after
// a customer redeems in Douyin's 来客 app, the merchant (or super-admin)
// records how many redemptions and how much money that day. The link must
// belong to the merchant. RedemptionDate cannot be in the future.
func (s *AdminService) CreateGroupBuyManualRedemption(merchantID uint, req dto.CreateGroupBuyManualRedemptionRequest, actor AuditActor) (*model.GroupBuyManualRedemption, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("redemption database is unavailable")
	}
	if merchantID == 0 {
		return nil, errors.New("merchant id is required")
	}
	date, err := parseBeijingDate(req.RedemptionDate)
	if err != nil {
		return nil, err
	}
	// Compare against Beijing midnight, not Truncate(24h): truncating an
	// absolute duration lands at 08:00 Beijing, which would wrongly refuse
	// today's date.
	nowBeijing := time.Now().In(beijingTimeZone)
	today := time.Date(nowBeijing.Year(), nowBeijing.Month(), nowBeijing.Day(), 0, 0, 0, 0, beijingTimeZone)
	if date.After(today) {
		return nil, errors.New("核销日期不能是未来日期")
	}
	var link model.MerchantGroupBuyLink
	if err := s.db.Where("id = ? AND merchant_id = ?", req.LinkID, merchantID).First(&link).Error; err != nil {
		return nil, errors.New("团购链接不存在或不属于该商家")
	}
	if req.StoreID != nil {
		var store model.Store
		if err := s.db.Where("id = ? AND merchant_id = ?", *req.StoreID, merchantID).First(&store).Error; err != nil {
			return nil, errors.New("门店不存在或不属于该商家")
		}
	}
	entry := &model.GroupBuyManualRedemption{
		MerchantID:            merchantID,
		StoreID:               req.StoreID,
		LinkID:                req.LinkID,
		RedemptionDate:        model.PlatformExpenseDate(req.RedemptionDate),
		RedemptionCount:       req.RedemptionCount,
		RedemptionAmountCents: req.RedemptionAmountCents,
		Note:                  strings.TrimSpace(req.Note),
		RecordedBy:            actor.ID,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "group_buy_manual_redemption_create", "group_buy_manual_redemption", entry.ID)
	}); err != nil {
		return nil, err
	}
	return entry, nil
}

// ListGroupBuyManualRedemptions lists a merchant's manual redemption rows
// (optionally for one link), newest date first, with totals over the scope.
func (s *AdminService) ListGroupBuyManualRedemptions(merchantID uint, linkID uint, limit, offset int) (*GroupBuyManualRedemptionList, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("redemption database is unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := s.db.Model(&model.GroupBuyManualRedemption{}).Where("merchant_id = ?", merchantID)
	if linkID != 0 {
		query = query.Where("link_id = ?", linkID)
	}
	var rows []model.GroupBuyManualRedemption
	if err := query.Order("redemption_date DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	type totals struct {
		Count  int64 `gorm:"column:count"`
		Amount int64 `gorm:"column:amount"`
	}
	var total totals
	totalQuery := s.db.Model(&model.GroupBuyManualRedemption{}).
		Select("COALESCE(SUM(redemption_count),0) AS count, COALESCE(SUM(redemption_amount_cents),0) AS amount").
		Where("merchant_id = ?", merchantID)
	if linkID != 0 {
		totalQuery = totalQuery.Where("link_id = ?", linkID)
	}
	if err := totalQuery.Scan(&total).Error; err != nil {
		return nil, err
	}
	return &GroupBuyManualRedemptionList{
		Rows:             rows,
		TotalCount:       total.Count,
		TotalAmountCents: total.Amount,
		SourceNote:       "手工录入：抖音来客核销后人工登记，未经平台 API 验证，仅供与链接点击数对照参考",
	}, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate entry")
}
