package storage

import (
	"errors"
	"slices"

	"cryptotape/internal/core"
	"cryptotape/internal/listings"
)

func validateRuleKind(rule *core.Rule) error {
	switch rule.Kind {
	case core.PriceMove:
		if rule.Percent < 0.1 || rule.Percent > 100 || !finite(rule.Percent) || rule.WindowSeconds < 5 || rule.WindowSeconds > 600 || !finite(rule.MinQuoteVolume) || rule.MinQuoteVolume < 0 {
			return errors.New("invalid alert rule")
		}
		rule.OpenExchanges = []string{}
		rule.ListingExchanges = []string{}
		rule.ListingTypes = []string{}
	case core.Listing:
		if len(rule.OpenExchanges) == 0 {
			rule.OpenExchanges = core.DefaultOpenExchanges()
		}
		if !validOpenExchanges(rule.OpenExchanges) {
			return errors.New("invalid exchange priority")
		}
		rule.OpenExchanges = slices.Clone(rule.OpenExchanges)
		if len(rule.ListingExchanges) > 32 || len(rule.ListingTypes) == 0 || len(rule.ListingTypes) > 6 {
			return errors.New("invalid listing filters")
		}
		for _, exchange := range rule.ListingExchanges {
			if !listings.ValidExchange(exchange) {
				return errors.New("invalid listing filters")
			}
		}
		for _, kind := range rule.ListingTypes {
			if !listings.ValidMarketType(kind) {
				return errors.New("invalid listing filters")
			}
		}
		rule.ListingExchanges = slices.Clone(rule.ListingExchanges)
		slices.Sort(rule.ListingExchanges)
		rule.ListingExchanges = slices.Compact(rule.ListingExchanges)
		if rule.ListingExchanges == nil {
			rule.ListingExchanges = []string{}
		}
		rule.ListingTypes = slices.Clone(rule.ListingTypes)
		slices.Sort(rule.ListingTypes)
		rule.ListingTypes = slices.Compact(rule.ListingTypes)
		rule.Percent, rule.WindowSeconds, rule.MinQuoteVolume = 0, 0, 0
	default:
		return errors.New("invalid alert type")
	}
	return nil
}

func validBannerSeconds(seconds int) bool {
	switch seconds {
	case core.BannerUntilClosed, 0, 5, 15, 30, 60, 300:
		return true
	default:
		return false
	}
}

func validOpenExchanges(exchanges []string) bool {
	if len(exchanges) != 4 {
		return false
	}
	seen := make(map[string]bool, 4)
	for _, exchange := range exchanges {
		if !slices.Contains(core.DefaultOpenExchanges(), exchange) || seen[exchange] {
			return false
		}
		seen[exchange] = true
	}
	return true
}
