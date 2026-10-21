package core

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkProcessMarkets(b *testing.B) {
	for _, ruleCount := range []int{2, 10} {
		b.Run(fmt.Sprintf("750_markets/%d_rules", ruleCount), func(b *testing.B) {
			rules := make([]Rule, ruleCount)
			for i := range rules {
				rules[i] = Rule{ID: int64(i + 1), Percent: float64(5 + i), WindowSeconds: []int{60, 300}[i%2], MinQuoteVolume: 80e6}
			}
			engine := NewEngine(rules)
			var symbols [750]string
			volumes := make(map[string]float64, len(symbols))
			for i := range symbols {
				symbols[i] = fmt.Sprintf("COIN%dUSDT", i)
				volumes[symbols[i]] = 100e6
			}
			engine.SetVolumes(volumes)
			base := time.Unix(1_800_000_000, 0)
			for _, symbol := range symbols {
				engine.Process(quote(symbol, 100, base))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// each market receives 50 ticks/s; movement stays below every threshold.
				round := i/len(symbols) + 1
				at := base.Add(time.Duration(round) * 20 * time.Millisecond)
				if alerts := engine.Process(quote(symbols[i%len(symbols)], 100+float64(round%100)/1000, at)); len(alerts) != 0 {
					b.Fatal("unexpected signal")
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ticks/s")
		})
	}
}
