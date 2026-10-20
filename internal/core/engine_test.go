package core

import (
	"fmt"
	"testing"
	"time"
)

func quote(symbol string, price float64, at time.Time) Quote {
	return Quote{Symbol: symbol, Bid: price * 0.9999, Ask: price * 1.0001, EventTime: at, Received: at}
}

func warm(t *testing.T, engine *Engine, symbol string, start time.Time, window time.Duration) time.Time {
	t.Helper()
	for elapsed := time.Duration(0); elapsed <= window; elapsed += time.Second {
		if got := engine.Process(quote(symbol, 100, start.Add(elapsed))); len(got) != 0 {
			t.Fatalf("alert on a constant price: %+v", got)
		}
	}
	return start.Add(window)
}

func TestKnifeFirstCrossingAndRebound(t *testing.T) {
	rule := Rule{ID: 1, Name: "knife", Percent: 5, WindowSeconds: 60, MinQuoteVolume: 80_000_000}
	engine := NewEngine([]Rule{rule}, DefaultSettings())
	engine.SetVolumes(map[string]float64{"BTCUSDT": 100_000_000})
	base := warm(t, engine, "BTCUSDT", time.Unix(1_800_000_000, 0), time.Minute)

	alerts := 0
	for second := 1; second <= 8; second++ {
		price := 100 - float64(second)*15/8
		found := engine.Process(quote("BTCUSDT", price, base.Add(time.Duration(second)*time.Second)))
		if second == 3 && len(found) != 1 {
			t.Fatalf("expected first crossing at 5.6%% drop, got %d", len(found))
		}
		alerts += len(found)
	}
	for second := 1; second <= 3; second++ {
		alerts += len(engine.Process(quote("BTCUSDT", 85+float64(second)*5, base.Add(time.Duration(8+second)*time.Second))))
	}
	if alerts != 1 {
		t.Fatalf("wanted one alert during fall and rebound, got %d", alerts)
	}
}

func TestRulesAreIndependent(t *testing.T) {
	rules := []Rule{
		{ID: 1, Percent: 3, WindowSeconds: 60, MinQuoteVolume: 80_000_000},
		{ID: 2, Percent: 10, WindowSeconds: 60, MinQuoteVolume: 80_000_000},
	}
	engine := NewEngine(rules, DefaultSettings())
	engine.SetVolumes(map[string]float64{"BTCUSDT": 100_000_000})
	base := warm(t, engine, "BTCUSDT", time.Unix(1_800_000_000, 0), time.Minute)
	first := engine.Process(quote("BTCUSDT", 96, base.Add(time.Second)))
	if len(first) != 1 || first[0].Rule.ID != 1 || first[0].Direction != "down" {
		t.Fatalf("expected 3%% down rule: %+v", first)
	}
	second := engine.Process(quote("BTCUSDT", 111, base.Add(2*time.Second)))
	if len(second) != 1 || second[0].Rule.ID != 2 || second[0].Direction != "up" {
		t.Fatalf("expected independent 10%% up rule: %+v", second)
	}
}

func TestExactThresholdCrossing(t *testing.T) {
	for _, test := range []struct {
		price     float64
		direction string
	}{{90, "down"}, {110, "up"}} {
		t.Run(test.direction, func(t *testing.T) {
			rule := Rule{ID: 1, Percent: 10, WindowSeconds: 60, MinQuoteVolume: 80_000_000}
			engine := NewEngine([]Rule{rule}, DefaultSettings())
			engine.SetVolumes(map[string]float64{"BTCUSDT": 100_000_000})
			base := warm(t, engine, "BTCUSDT", time.Unix(1_800_000_000, 0), time.Minute)
			before := test.price + .000001
			if test.direction == "up" {
				before = test.price - .000001
			}
			if found := engine.Process(quote("BTCUSDT", before, base.Add(500*time.Millisecond))); len(found) != 0 {
				t.Fatalf("threshold fired early: %+v", found)
			}
			found := engine.Process(quote("BTCUSDT", test.price, base.Add(time.Second)))
			if len(found) != 1 || found[0].Direction != test.direction {
				t.Fatalf("exact threshold was missed: %+v", found)
			}
		})
	}
}

func TestGapRequiresFreshBaselineAndVolume(t *testing.T) {
	rule := Rule{ID: 1, Percent: 5, WindowSeconds: 5, MinQuoteVolume: 80_000_000}
	engine := NewEngine([]Rule{rule}, DefaultSettings())
	base := time.Unix(1_800_000_000, 0)
	for second := 0; second <= 5; second++ {
		engine.Process(quote("BTCUSDT", 100, base.Add(time.Duration(second)*time.Second)))
	}
	if got := engine.Process(quote("BTCUSDT", 94, base.Add(6*time.Second))); len(got) != 0 {
		t.Fatalf("volume filter failed: %+v", got)
	}
	engine.SetVolumes(map[string]float64{"BTCUSDT": 100_000_000})
	engine.Reset([]string{"BTCUSDT"})
	if got := engine.Process(quote("BTCUSDT", 100, base.Add(20*time.Second))); len(got) != 0 {
		t.Fatalf("first fresh quote must only establish a baseline: %+v", got)
	}
	if got := engine.Process(quote("BTCUSDT", 94, base.Add(21*time.Second))); len(got) != 1 {
		t.Fatalf("fresh crossing must alert without waiting for a full window: %+v", got)
	}
}

func TestWindowBucketsStayBounded(t *testing.T) {
	w := newWindow(5 * time.Second)
	base := time.Unix(1_800_000_000, 0)
	for tick := 0; tick < 20_000; tick++ {
		w.add(base.Add(time.Duration(tick)*time.Millisecond), float64(tick))
	}
	if len(w.maxima)-w.maxHead > 51 || len(w.minima)-w.minHead > 51 {
		t.Fatalf("bucket storage grew past window: min=%d max=%d", len(w.minima)-w.minHead, len(w.maxima)-w.maxHead)
	}
}

func TestCooldownDoesNotEmitLateContinuation(t *testing.T) {
	rule := Rule{ID: 1, Percent: 5, WindowSeconds: 60, MinQuoteVolume: 80_000_000}
	engine := NewEngine([]Rule{rule}, DefaultSettings())
	engine.SetVolumes(map[string]float64{"BTCUSDT": 100_000_000})
	base := warm(t, engine, "BTCUSDT", time.Unix(1_800_000_000, 0), time.Minute)
	if got := engine.Process(quote("BTCUSDT", 94, base.Add(time.Second))); len(got) != 1 {
		t.Fatalf("first crossing: %+v", got)
	}
	for second := 2; second <= 65; second++ {
		price := 94.0
		if second > 10 {
			price = 93.0
		}
		if got := engine.Process(quote("BTCUSDT", price, base.Add(time.Duration(second)*time.Second))); len(got) != 0 {
			t.Fatalf("late continuation at %ds: %+v", second, got)
		}
	}
}

func BenchmarkProcessLiquid60(b *testing.B) {
	rule := Rule{ID: 1, Percent: 5, WindowSeconds: 300, MinQuoteVolume: 80_000_000}
	engine := NewEngine([]Rule{rule}, DefaultSettings())
	volumes := make(map[string]float64)
	var symbols [60]string
	for index := 0; index < 60; index++ {
		symbols[index] = fmt.Sprintf("COIN%dUSDT", index)
		volumes[symbols[index]] = 100_000_000
	}
	engine.SetVolumes(volumes)
	start := time.Unix(1_800_000_000, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		at := start.Add(time.Duration(index) * 100 * time.Millisecond)
		engine.Process(quote(symbols[index%60], 100+float64(index%100)/1000, at))
	}
}

func TestEachRuleHasIndependentTurnoverFilter(t *testing.T) {
	rules := []Rule{
		{ID: 1, Percent: 3, WindowSeconds: 60, MinQuoteVolume: 80e6},
		{ID: 2, Percent: 10, WindowSeconds: 60, MinQuoteVolume: 200e6},
	}
	engine := NewEngine(rules, DefaultSettings())
	engine.SetVolumes(map[string]float64{"BTCUSDT": 100e6})
	base := warm(t, engine, "BTCUSDT", time.Unix(1_800_000_000, 0), time.Minute)
	got := engine.Process(quote("BTCUSDT", 89, base.Add(time.Second)))
	if len(got) != 1 || got[0].Rule.ID != 1 {
		t.Fatalf("per-rule filter failed: %+v", got)
	}
	engine.SetVolumes(map[string]float64{"BTCUSDT": 250e6})
	if got := engine.Process(quote("BTCUSDT", 88, base.Add(2*time.Second))); len(got) != 0 {
		t.Fatalf("delayed crossing after turnover change: %+v", got)
	}
	if MinimumVolume(rules) != 80e6 {
		t.Fatal("wrong subscription floor")
	}
}

func TestPartialWindowAlertsImmediately(t *testing.T) {
	for _, duration := range []time.Duration{time.Second, 2 * time.Minute} {
		t.Run(duration.String(), func(t *testing.T) {
			engine := NewEngine([]Rule{{ID: 1, Percent: 10, WindowSeconds: 300}}, DefaultSettings())
			base := time.Unix(1_800_000_000, 0)
			for elapsed := time.Duration(0); elapsed < duration; elapsed += time.Second {
				if got := engine.Process(quote("BTCUSDT", 100, base.Add(elapsed))); len(got) != 0 {
					t.Fatalf("constant price produced alerts: %+v", got)
				}
			}
			got := engine.Process(quote("BTCUSDT", 90, base.Add(duration)))
			if len(got) != 1 || got[0].Direction != "down" {
				t.Fatalf("partial-window crossing was missed: %+v", got)
			}
			if engine.ReadyCount(base.Add(duration)) != 1 {
				t.Fatal("fresh partial history must be ready")
			}
		})
	}
}

func TestReconnectDiscardsPricesButPreservesCooldown(t *testing.T) {
	for _, reset := range []string{"gap", "stream", "all"} {
		t.Run(reset, func(t *testing.T) {
			engine := NewEngine([]Rule{{ID: 1, Percent: 5, WindowSeconds: 300}}, DefaultSettings())
			base := time.Unix(1_800_000_000, 0)
			engine.Process(quote("BTCUSDT", 100, base))
			if got := engine.Process(quote("BTCUSDT", 94, base.Add(time.Second))); len(got) != 1 {
				t.Fatalf("first crossing: %+v", got)
			}
			if reset == "stream" {
				engine.Reset([]string{"BTCUSDT"})
			}
			if reset == "all" {
				engine.ResetAll()
			}
			if got := engine.Process(quote("BTCUSDT", 80, base.Add(20*time.Second))); len(got) != 0 {
				t.Fatalf("movement across gap inferred: %+v", got)
			}
			if got := engine.Process(quote("BTCUSDT", 88, base.Add(21*time.Second))); len(got) != 0 {
				t.Fatalf("reconnect bypassed cooldown: %+v", got)
			}
		})
	}
}

func TestPartialWindowExpiresOldExtrema(t *testing.T) {
	engine := NewEngine([]Rule{{ID: 1, Percent: 10, WindowSeconds: 5}}, DefaultSettings())
	base := time.Unix(1_800_000_000, 0)
	for second := 0; second <= 6; second++ {
		engine.Process(quote("BTCUSDT", 100-float64(second), base.Add(time.Duration(second)*time.Second)))
	}
	if got := engine.Process(quote("BTCUSDT", 90, base.Add(7*time.Second))); len(got) != 0 {
		t.Fatalf("price outside the maximum window used: %+v", got)
	}
}
