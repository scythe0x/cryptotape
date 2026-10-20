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
			t.Fatalf("alert during warmup: %+v", got)
		}
	}
	return start.Add(window)
}

func TestKnifeFirstCrossingAndRebound(t *testing.T) {
	rule := Rule{ID: 1, Name: "knife", Percent: 5, WindowSeconds: 60, Enabled: true}
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
		{ID: 1, Percent: 3, WindowSeconds: 60, Enabled: true},
		{ID: 2, Percent: 10, WindowSeconds: 60, Enabled: true},
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

func TestGapRequiresFreshWindowAndVolume(t *testing.T) {
	rule := Rule{ID: 1, Percent: 5, WindowSeconds: 5, Enabled: true}
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
		t.Fatalf("gap should rewarm: %+v", got)
	}
	if got := engine.Process(quote("BTCUSDT", 94, base.Add(21*time.Second))); len(got) != 0 {
		t.Fatalf("alert before full window: %+v", got)
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
	rule := Rule{ID: 1, Percent: 5, WindowSeconds: 60, Enabled: true}
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
	rule := Rule{ID: 1, Percent: 5, WindowSeconds: 300, Enabled: true}
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
