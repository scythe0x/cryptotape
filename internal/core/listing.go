package core

type RuleKind string

const (
	PriceMove RuleKind = "price_move"
	Listing   RuleKind = "listing"
)

func (r Rule) IsListing() bool { return r.Kind == Listing }

func priceRules(rules []Rule) []Rule {
	result := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if !rule.IsListing() {
			result = append(result, rule)
		}
	}
	return result
}

type ListingDetails struct {
	Exchange       string   `json:"exchange"`
	Asset          string   `json:"asset"`
	MarketType     string   `json:"marketType"`
	Markets        []string `json:"markets"`
	URL            string   `json:"url"`
	DetectedTimeUs int64    `json:"detectedTimeUs"`
	SentTimeUs     int64    `json:"sentTimeUs"`
}

type ListingHealth struct {
	State          string `json:"state"`
	Code           string `json:"code"`
	Plan           string `json:"plan"`
	DelayMs        int64  `json:"delayMs"`
	Received       uint64 `json:"received"`
	Rejected       uint64 `json:"rejected"`
	Unmatched      uint64 `json:"unmatched"`
	LastEventAtMs  int64  `json:"lastEventAtMs"`
	ProviderWaitMs int64  `json:"providerWaitMs"`
	DeliveryMs     int64  `json:"deliveryMs"`
}

func DefaultOpenExchanges() []string { return []string{"binance", "bybit", "okx", "gate"} }
