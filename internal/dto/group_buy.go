package dto

// PublicGroupBuyLink exposes only the platform label and opaque redirect code
// needed by the customer store page. It deliberately omits the destination URL
// and aggregate redirect counters.
type PublicGroupBuyLink struct {
	Channel string `json:"channel"`
	Code    string `json:"code"`
}
