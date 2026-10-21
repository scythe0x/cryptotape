package listings

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"cryptotape/internal/core"
)

var ticker = regexp.MustCompile(`^[A-Z0-9_]{1,32}$`)

type Event struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Detected int64  `json:"detected_time_us"`
	Sent     int64  `json:"sent_time_us"`
	Parser   struct {
		Exchange       string `json:"exchange"`
		Classification struct {
			Event    string   `json:"event"`
			Type     string   `json:"type"`
			Category string   `json:"category"`
			Markets  []string `json:"markets"`
		} `json:"classification"`
		Assets []struct {
			Symbol string `json:"symbol"`
		} `json:"assets"`
	} `json:"parser"`
}

func (e Event) Listings(received time.Time) ([]core.ListingDetails, error) {
	classification := e.Parser.Classification
	if (e.Type != "announcement" && e.Type != "tweet") || classification.Event != "listing" || classification.Category != "crypto" || e.Parser.Exchange == "binance" {
		return nil, nil
	}
	age := received.Sub(time.UnixMicro(e.Detected))
	if e.Detected <= 0 || e.Sent < e.Detected || age > 10*time.Second || age < -2*time.Second || time.UnixMicro(e.Sent).After(received.Add(2*time.Second)) {
		return nil, errors.New("stale or invalid listing timestamps")
	}
	source, err := url.Parse(e.URL)
	if err != nil || source.Scheme != "https" || source.Hostname() == "" || source.User != nil || len(e.URL) > 2048 || len(e.Parser.Assets) > 100 {
		return nil, errors.New("invalid listing source")
	}
	if !ValidExchange(e.Parser.Exchange) || !ValidMarketType(classification.Type) {
		return nil, nil
	}
	markets := slices.Clone(classification.Markets)
	for i, market := range markets {
		markets[i] = strings.ToLower(market)
	}
	slices.Sort(markets)
	markets = slices.Compact(markets)
	var result []core.ListingDetails
	seen := make(map[string]bool)
	for _, asset := range e.Parser.Assets {
		symbol := strings.ToUpper(strings.TrimSpace(asset.Symbol))
		if !ticker.MatchString(symbol) || seen[symbol] {
			continue
		}
		seen[symbol] = true
		result = append(result, core.ListingDetails{Exchange: e.Parser.Exchange, Asset: symbol, MarketType: classification.Type, Markets: markets, URL: e.URL, DetectedTimeUs: e.Detected, SentTimeUs: e.Sent})
	}
	return result, nil
}

func ValidExchange(exchange string) bool {
	if exchange == "" || exchange == "binance" || len(exchange) > 64 {
		return false
	}
	for _, ch := range exchange {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}

func ValidMarketType(kind string) bool {
	return slices.Contains([]string{"spot", "futures", "pre-market", "roadmap", "soon-spot", "assets"}, kind)
}

// group duplicate posts about the same market addition, independently of edge event ids.
func DedupKey(event core.ListingDetails) string {
	markets := slices.Clone(event.Markets)
	slices.Sort(markets)
	sum := sha256.Sum256([]byte(strings.Join([]string{event.Exchange, event.Asset, event.MarketType, strings.Join(slices.Compact(markets), ",")}, "\x00")))
	return hex.EncodeToString(sum[:])
}

type Catalog map[string][]string

func NewCatalog(symbols []string) Catalog {
	catalog := make(Catalog)
	seen := make(map[string]bool)
	for _, symbol := range symbols {
		if !strings.HasSuffix(symbol, "USDT") || !ticker.MatchString(symbol) || seen[symbol] {
			continue
		}
		seen[symbol] = true
		asset := strings.TrimSuffix(symbol, "USDT")
		catalog[asset] = append(catalog[asset], symbol)
		for _, multiplier := range []string{"1000000", "100000", "10000", "1000"} {
			if strings.HasPrefix(asset, multiplier) && len(asset) > len(multiplier) {
				base := strings.TrimPrefix(asset, multiplier)
				catalog[base] = append(catalog[base], symbol)
				break
			}
		}
	}
	return catalog
}

func (c Catalog) Resolve(asset string) (string, bool) {
	symbols := c[asset]
	if len(symbols) != 1 {
		return "", false
	}
	return symbols[0], true
}
