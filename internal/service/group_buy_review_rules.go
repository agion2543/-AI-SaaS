package service

import (
	"net"
	"net/url"
	"strings"
	"unicode"
)

const (
	GroupBuyRuleAutoApproved       = "GBR-AUTO-001"
	GroupBuyRuleURLUnsafe          = "GBR-SEC-001"
	GroupBuyRuleUnsupportedChannel = "GBR-CHAN-001"
	GroupBuyRuleWrongChannel       = "GBR-CHAN-002"
	GroupBuyRuleLookalikeHost      = "GBR-DOMAIN-001"
	GroupBuyRuleUnknownHost        = "GBR-DOMAIN-002"
	GroupBuyRuleAmbiguousHost      = "GBR-DOMAIN-003"
	GroupBuyRuleShortOrRootLink    = "GBR-LINK-001"
	GroupBuyRuleMerchantState      = "GBR-STATE-001"
	GroupBuyRuleChangedAfterPass   = "GBR-CHANGE-001"
	GroupBuyRuleAdminDecision      = "GBR-ADMIN-001"
	GroupBuyRuleAdminStateChange   = "GBR-ADMIN-002"
)

const (
	groupBuyReviewApproved = "approved"
	groupBuyReviewPending  = "pending"
	groupBuyReviewRejected = "rejected"

	groupBuyReviewAuto   = "auto"
	groupBuyReviewManual = "manual"
)

type groupBuyReviewDecision struct {
	Status string
	Method string
	RuleID string
	Reason string
}

// evaluateGroupBuyReview is deliberately local and deterministic: it never
// resolves DNS, requests the destination, or contacts a group-buy platform.
func evaluateGroupBuyReview(channel, rawURL, merchantStatus, storeStatus string, previouslyApproved bool) groupBuyReviewDecision {
	reject := func(ruleID, reason string) groupBuyReviewDecision {
		return groupBuyReviewDecision{Status: groupBuyReviewRejected, Method: groupBuyReviewAuto, RuleID: ruleID, Reason: reason}
	}
	manual := func(ruleID, reason string) groupBuyReviewDecision {
		return groupBuyReviewDecision{Status: groupBuyReviewPending, Method: groupBuyReviewManual, RuleID: ruleID, Reason: reason}
	}

	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel != GroupBuyChannelDouyin && channel != GroupBuyChannelMeituan {
		return reject(GroupBuyRuleUnsupportedChannel, "不支持的团购平台，需使用抖音或美团/大众点评")
	}

	parsed, err := parseSafeGroupBuyURL(rawURL)
	if err != nil {
		return reject(GroupBuyRuleURLUnsafe, err.Error())
	}
	host := strings.ToLower(parsed.Hostname())

	if groupBuyHostInFamily(host, otherGroupBuyChannelFamilies(channel)...) {
		return reject(GroupBuyRuleWrongChannel, "链接域名属于另一团购平台，与所选平台不一致")
	}
	family := groupBuyChannelFamilies(channel)
	if !groupBuyHostInFamily(host, family...) {
		if looksLikeGroupBuyPlatformHost(host) {
			return reject(GroupBuyRuleLookalikeHost, "域名疑似仿冒抖音或美团/大众点评官方域名")
		}
		return reject(GroupBuyRuleUnknownHost, "域名不在该平台可信域名范围内，已拒绝")
	}

	if !groupBuyExactAutoHost(channel, host) {
		return manual(GroupBuyRuleAmbiguousHost, "域名属于平台域名家族，但不在当前精确自动通过白名单内，需人工核验")
	}
	if host == "v.douyin.com" || parsed.Path == "" || parsed.Path == "/" {
		return manual(GroupBuyRuleShortOrRootLink, "短链或平台根页无法在本地确认具体团购落地页，需人工核验")
	}
	if previouslyApproved {
		return manual(GroupBuyRuleChangedAfterPass, "已通过入口的目标链接发生变更，必须重新人工审核")
	}
	if merchantStatus != "active" || storeStatus != "active" {
		return manual(GroupBuyRuleMerchantState, "商家或门店状态缺失/未启用，需人工核实后再审核")
	}

	return groupBuyReviewDecision{
		Status: groupBuyReviewApproved, Method: groupBuyReviewAuto,
		RuleID: GroupBuyRuleAutoApproved,
		Reason: "HTTPS 安全检查通过，平台、精确域名、商家和门店状态均符合自动通过条件",
	}
}

func parseSafeGroupBuyURL(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len([]rune(value)) > 1000 {
		return nil, urlInputError("链接为空或超过 1000 个字符")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" {
		return nil, urlInputError("仅支持格式有效且不含账号信息的 HTTPS 链接")
	}
	// Reject even :443 so the rule is simple, explicit and consistent at submit
	// and redirect time. IPv6 literals are also blocked below.
	if strings.Contains(parsed.Host, ":") {
		return nil, urlInputError("平台链接不能包含显式端口")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || strings.HasSuffix(host, ".") || net.ParseIP(host) != nil {
		return nil, urlInputError("平台链接域名无效")
	}
	for _, r := range host {
		if r > unicode.MaxASCII || unicode.IsControl(r) || unicode.IsSpace(r) {
			return nil, urlInputError("不支持国际化或含控制字符的域名")
		}
	}
	return parsed, nil
}

type groupBuyURLInputError string

func (e groupBuyURLInputError) Error() string { return string(e) }

func urlInputError(message string) error { return groupBuyURLInputError(message) }

func groupBuyChannelFamilies(channel string) []string {
	switch channel {
	case GroupBuyChannelDouyin:
		return []string{"douyin.com"}
	case GroupBuyChannelMeituan:
		return []string{"meituan.com", "dianping.com"}
	default:
		return nil
	}
}

func otherGroupBuyChannelFamilies(channel string) []string {
	if channel == GroupBuyChannelDouyin {
		return []string{"meituan.com", "dianping.com"}
	}
	if channel == GroupBuyChannelMeituan {
		return []string{"douyin.com"}
	}
	return nil
}

func groupBuyHostInFamily(host string, roots ...string) bool {
	for _, root := range roots {
		if host == root || strings.HasSuffix(host, "."+root) {
			return true
		}
	}
	return false
}

func groupBuyExactAutoHost(channel, host string) bool {
	switch channel {
	case GroupBuyChannelDouyin:
		return host == "life.douyin.com" || host == "v.douyin.com"
	case GroupBuyChannelMeituan:
		return host == "i.meituan.com" || host == "www.dianping.com"
	default:
		return false
	}
}

func looksLikeGroupBuyPlatformHost(host string) bool {
	for _, marker := range []string{"douyin", "meituan", "dianping"} {
		if strings.Contains(host, marker) {
			return true
		}
	}
	return strings.Contains(host, "xn--")
}
