package service

import "testing"

func TestValidateGroupBuyDestinationAcceptsOfficialHTTPSHosts(t *testing.T) {
	cases := []struct {
		channel string
		url     string
	}{
		{GroupBuyChannelDouyin, "https://life.douyin.com/discount/item?id=12"},
		{GroupBuyChannelDouyin, "https://v.douyin.com/AbCd/"},
		{GroupBuyChannelMeituan, "https://i.meituan.com/deal/123"},
		{GroupBuyChannelMeituan, "https://www.dianping.com/shop/abc"},
	}
	for _, item := range cases {
		t.Run(item.channel+"/"+item.url, func(t *testing.T) {
			got, err := ValidateGroupBuyDestination(item.channel, item.url)
			if err != nil {
				t.Fatalf("expected valid URL, got error: %v", err)
			}
			if got != item.url {
				t.Fatalf("expected URL %q, got %q", item.url, got)
			}
		})
	}
}

func TestValidateGroupBuyDestinationRejectsUnsafeOrMismatchedURLs(t *testing.T) {
	cases := []struct {
		name    string
		channel string
		url     string
	}{
		{"http", GroupBuyChannelDouyin, "http://life.douyin.com/item"},
		{"wrong platform", GroupBuyChannelDouyin, "https://www.meituan.com/item"},
		{"lookalike suffix", GroupBuyChannelMeituan, "https://meituan.com.evil.test/item"},
		{"credentials", GroupBuyChannelDouyin, "https://user@life.douyin.com/item"},
		{"custom port", GroupBuyChannelMeituan, "https://i.meituan.com:8443/item"},
		{"ip address", GroupBuyChannelDouyin, "https://127.0.0.1/item"},
		{"unknown channel", "other", "https://example.com/item"},
		{"empty", GroupBuyChannelDouyin, "  "},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if _, err := ValidateGroupBuyDestination(item.channel, item.url); err == nil {
				t.Fatalf("expected %q to be rejected", item.url)
			}
		})
	}
}

func TestNewGroupBuyCodeIsUniqueSizedHex(t *testing.T) {
	first, err := newGroupBuyCode()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newGroupBuyCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 || first == second {
		t.Fatalf("unexpected generated codes: %q and %q", first, second)
	}
	for _, value := range first {
		if !((value >= '0' && value <= '9') || (value >= 'a' && value <= 'f')) {
			t.Fatalf("code is not lowercase hex: %q", first)
		}
	}
}
