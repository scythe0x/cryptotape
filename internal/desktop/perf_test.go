package desktop

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func BenchmarkAlertPipelineBurst(b *testing.B) {
	var published atomic.Int64
	c, _ := testController(b, func(name string, _ any) {
		if name == "alert:new" {
			published.Add(1)
		}
	})
	rules := make([]core.Rule, 0, 2)
	for _, seconds := range []int{60, 300} {
		rule := lifecycleRule()
		rule.WindowSeconds = seconds
		saved, err := c.SaveRule(rule)
		if err != nil {
			b.Fatal(err)
		}
		rules = append(rules, saved)
	}
	var symbols [750]string
	volumes := make(map[string]float64, len(symbols))
	for i := range symbols {
		symbols[i] = fmt.Sprintf("COIN%dUSDT", i)
		volumes[symbols[i]] = 100e6
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		c.engine = core.NewEngine(rules, core.DefaultSettings())
		c.engine.SetVolumes(volumes)
		base := time.Now().Add(-time.Second)
		for _, symbol := range symbols {
			c.onQuote(marketQuote(symbol, 100, base))
		}
		published.Store(0)
		at := time.Now()
		b.StartTimer()
		for _, symbol := range symbols {
			c.onQuote(marketQuote(symbol, 85, at))
		}
		c.actionWG.Wait()
		b.StopTimer()
		if published.Load() != 1500 || c.pipelineHealth().Unsaved != 0 {
			b.Fatalf("published=%d health=%+v", published.Load(), c.pipelineHealth())
		}
	}
	b.ReportMetric(float64(b.N)*1500/b.Elapsed().Seconds(), "alerts/s")
	b.ReportMetric(b.Elapsed().Seconds()*1000/float64(b.N), "ms/burst")
}
