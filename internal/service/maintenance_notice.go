package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/repository"
)

const merchantMaintenanceNoticeConfigKey = "merchant_maintenance_notice"

var beijingTimeZone = time.FixedZone("Asia/Shanghai", 8*60*60)

const maintenanceDateTimeLayout = "2006-01-02T15:04"

func (s *AdminService) GetMaintenanceNotice() (dto.MerchantMaintenanceNotice, error) {
	if s == nil || s.db == nil {
		return dto.MerchantMaintenanceNotice{}, errors.New("maintenance notice database is unavailable")
	}
	config, err := repository.NewSystemRepository(s.db).Get(merchantMaintenanceNoticeConfigKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.MerchantMaintenanceNotice{}, nil
	}
	if err != nil {
		return dto.MerchantMaintenanceNotice{}, err
	}
	var notice dto.MerchantMaintenanceNotice
	if err := json.Unmarshal([]byte(config.ConfigValue), &notice); err != nil {
		return dto.MerchantMaintenanceNotice{}, fmt.Errorf("decode maintenance notice: %w", err)
	}
	return notice, nil
}

func (s *AdminService) SaveMaintenanceNotice(req dto.MerchantMaintenanceNotice, actor AuditActor) (dto.MerchantMaintenanceNotice, error) {
	if s == nil || s.db == nil {
		return dto.MerchantMaintenanceNotice{}, errors.New("maintenance notice database is unavailable")
	}
	notice := dto.MerchantMaintenanceNotice{
		Published:      req.Published,
		Title:          strings.TrimSpace(req.Title),
		WindowStart:    strings.TrimSpace(req.WindowStart),
		WindowEnd:      strings.TrimSpace(req.WindowEnd),
		ExpectedImpact: strings.TrimSpace(req.ExpectedImpact),
		Preparation:    strings.TrimSpace(req.Preparation),
	}
	if err := validateMaintenanceNotice(notice); err != nil {
		return dto.MerchantMaintenanceNotice{}, err
	}
	payload, err := json.Marshal(notice)
	if err != nil {
		return dto.MerchantMaintenanceNotice{}, err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := repository.NewSystemRepository(tx).Upsert(merchantMaintenanceNoticeConfigKey, string(payload), false); err != nil {
			return err
		}
		return appendSystemConfigAuditTx(tx, actor, "merchant_maintenance_notice_update", "商家维护公告发布或更新", []string{merchantMaintenanceNoticeConfigKey})
	})
	if err != nil {
		return dto.MerchantMaintenanceNotice{}, err
	}
	return notice, nil
}

func (s *AdminService) ActiveMaintenanceNotice() (*dto.MerchantMaintenanceNotice, error) {
	notice, err := s.GetMaintenanceNotice()
	if err != nil {
		return nil, err
	}
	if !notice.Published {
		return nil, nil
	}
	windowEnd, err := parseMaintenanceDateTime(notice.WindowEnd)
	if err != nil {
		return nil, fmt.Errorf("decode maintenance notice end time: %w", err)
	}
	if time.Now().After(windowEnd) {
		return nil, nil
	}
	return &notice, nil
}

func validateMaintenanceNotice(notice dto.MerchantMaintenanceNotice) error {
	if runeCount(notice.Title) > 100 {
		return errors.New("公告标题不能超过 100 个字符")
	}
	if runeCount(notice.ExpectedImpact) > 1000 {
		return errors.New("预期影响说明不能超过 1000 个字符")
	}
	if runeCount(notice.Preparation) > 1000 {
		return errors.New("商家准备事项不能超过 1000 个字符")
	}

	var start, end time.Time
	var err error
	if notice.WindowStart != "" {
		start, err = parseMaintenanceDateTime(notice.WindowStart)
		if err != nil {
			return errors.New("维护开始时间格式无效，请使用北京时间 YYYY-MM-DD HH:mm")
		}
	}
	if notice.WindowEnd != "" {
		end, err = parseMaintenanceDateTime(notice.WindowEnd)
		if err != nil {
			return errors.New("维护结束时间格式无效，请使用北京时间 YYYY-MM-DD HH:mm")
		}
	}
	if !start.IsZero() && !end.IsZero() && !end.After(start) {
		return errors.New("维护结束时间必须晚于开始时间")
	}
	if notice.Published {
		if notice.Title == "" || notice.WindowStart == "" || notice.WindowEnd == "" || notice.ExpectedImpact == "" || notice.Preparation == "" {
			return errors.New("发布公告前请填写标题、维护时间窗口、预期影响和商家准备事项")
		}
		if !end.After(time.Now()) {
			return errors.New("维护结束时间已过，不能发布该公告")
		}
	}
	return nil
}

func parseMaintenanceDateTime(value string) (time.Time, error) {
	return time.ParseInLocation(maintenanceDateTimeLayout, value, beijingTimeZone)
}

func runeCount(value string) int {
	return utf8.RuneCountInString(value)
}
