package repository

import (
	"strings"
	"time"

	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type MerchantRepository struct {
	db *gorm.DB
}

func NewMerchantRepository(db *gorm.DB) *MerchantRepository {
	return &MerchantRepository{db: db}
}

func (r *MerchantRepository) Create(merchant *model.Merchant) error {
	return r.db.Create(merchant).Error
}

func (r *MerchantRepository) Save(merchant *model.Merchant) error {
	return r.db.Save(merchant).Error
}

func (r *MerchantRepository) FindByID(id uint) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.First(&merchant, id).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *MerchantRepository) FindByName(name string) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.Where("name = ?", name).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

// List 返回一页商家（id DESC）。keyword 非空时按 name 模糊搜（LIKE，
// % _ \ 先转义），用于下拉框远程搜索；total 是筛选后的计数。
// 分页仍走全局 NormalizePage 上限（page_size > 100 会被压成 10），
// 调用方不得靠放大 page_size 拉全量。
func (r *MerchantRepository) List(offset, limit int, keyword string) ([]model.Merchant, int64, error) {
	return r.ListFiltered(MerchantListFilter{Keyword: keyword, SortBy: "id", SortOrder: "desc"}, offset, limit)
}

// MerchantListFilter 是商家列表的服务端筛选/排序参数（与 Users 的
// UserListFilter 模式一致）。Subscription 取 ""（不过滤）/"active"
// （订阅有效：subscription_status=active 且当前到期日在未来）/
// "inactive"（其余）；ExpiringWithinDays > 0 时只取订阅有效且当前
// 到期日在未来 N 天内的商家（与 Subscription 合用，N 天内到期）。
// SortBy 只认白名单：id / created_at / name / subscription_expire_at；
// 聚合排序（risk_score / trade_amount / open_follow_up_count）不在
// 本层做，由 service 在聚合后排序。
type MerchantListFilter struct {
	Keyword            string
	Status             string
	Subscription       string
	Risk               string // 占位：风险筛查在 service 层按聚合算分后过滤，本层忽略
	ExpiringWithinDays int
	SortBy             string
	SortOrder          string
}

// merchantListSortColumn 把 SortBy 映射到白名单列；聚合排序与非法值
// 一律回落 id，保证 ORDER BY 永远是常量拼接、无注入面。
func merchantListSortColumn(sortBy string) string {
	switch sortBy {
	case "created_at":
		return "merchants.created_at"
	case "name":
		return "merchants.name"
	case "subscription_expire_at":
		return "COALESCE(merchants.subscription_expire_at, merchants.subscription_expired_at)"
	case "id":
		return "merchants.id"
	default:
		return "merchants.id"
	}
}

// applyMerchantListFilter 把基础筛选（关键字/状态/订阅/即将到期）
// 叠到 query 上。关键字同时匹配 name 与 contact_phone（与旧前端
// 本地过滤一致：搜手机号也能找到商家）；% _ \ 先转义按字面匹配。
// 订阅有效用 COALESCE(expire_at, expired_at) 与前端 subscriptionValid
// 的回落逻辑一致；inactive 分支显式写出，避免 NULL 参与 NOT 运算
// 时把无到期日的商家漏掉。
func applyMerchantListFilter(query *gorm.DB, filter MerchantListFilter, now time.Time) *gorm.DB {
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(keyword)
		like := "%" + escaped + "%"
		query = query.Where("merchants.name LIKE ? ESCAPE '\\' OR merchants.contact_phone LIKE ? ESCAPE '\\'", like, like)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("merchants.status = ?", status)
	}
	expire := "COALESCE(merchants.subscription_expire_at, merchants.subscription_expired_at)"
	switch strings.TrimSpace(filter.Subscription) {
	case "active":
		query = query.Where("merchants.subscription_status = ? AND "+expire+" > ?", "active", now)
	case "inactive":
		query = query.Where("merchants.subscription_status <> ? OR "+expire+" IS NULL OR "+expire+" <= ?", "active", now)
	}
	if filter.ExpiringWithinDays > 0 {
		query = query.Where("merchants.subscription_status = ? AND "+expire+" > ? AND "+expire+" <= ?",
			"active", now, now.AddDate(0, 0, filter.ExpiringWithinDays))
	}
	return query
}

// ListFiltered 返回一页商家 + 筛选后总数。total 是基础筛选后的计数
// （风险筛查不在本层，见 MerchantListFilter.Risk 注释）。
func (r *MerchantRepository) ListFiltered(filter MerchantListFilter, offset, limit int) ([]model.Merchant, int64, error) {
	query := applyMerchantListFilter(r.db.Model(&model.Merchant{}), filter, time.Now())
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	sortOrder := "desc"
	if strings.EqualFold(filter.SortOrder, "asc") {
		sortOrder = "asc"
	}
	var merchants []model.Merchant
	err := query.Order(merchantListSortColumn(filter.SortBy) + " " + sortOrder).Offset(offset).Limit(limit).Find(&merchants).Error
	return merchants, total, err
}

// ListAllFiltered 返回基础筛选后的全部商家（id DESC），供 service 在
// 需要聚合排序/风险筛查时全量算分后自行分页；调用方不得用它当列表
// 返回，也不许靠它绕过分页。
func (r *MerchantRepository) ListAllFiltered(filter MerchantListFilter) ([]model.Merchant, error) {
	query := applyMerchantListFilter(r.db.Model(&model.Merchant{}), filter, time.Now())
	var merchants []model.Merchant
	err := query.Order("merchants.id desc").Find(&merchants).Error
	return merchants, err
}
