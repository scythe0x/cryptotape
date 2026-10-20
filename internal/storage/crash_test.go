package storage_test

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"cryptotape/internal/core"
	"cryptotape/internal/storage"
)

func TestMarketCrashPersistsEveryCrossing(t *testing.T) {
	const markets = 750
	rules := []core.Rule{
		{ID: 1, Name: "soft", Percent: 3, WindowSeconds: 60, MinQuoteVolume: 80_000_000},
		{ID: 2, Name: "strong", Percent: 10, WindowSeconds: 300, MinQuoteVolume: 80_000_000},
	}
	engine := core.NewEngine(rules, core.DefaultSettings())
	volumes := make(map[string]float64, markets)
	symbols := make([]string, markets)
	for index := range symbols {
		symbols[index] = fmt.Sprintf("COIN%dUSDT", index)
		volumes[symbols[index]] = 100_000_000
	}
	engine.SetVolumes(volumes)
	quote := func(symbol string, price float64, at time.Time) core.Quote {
		return core.Quote{Symbol: symbol, Bid: price * .9999, Ask: price * 1.0001, EventTime: at, Received: at}
	}
	base := time.Now().Add(-5 * time.Minute)
	for second := 0; second <= 300; second++ {
		for _, symbol := range symbols {
			if found := engine.Process(quote(symbol, 100, base.Add(time.Duration(second)*time.Second))); len(found) != 0 {
				t.Fatal("unexpected signal during warmup")
			}
		}
	}
	store, err := storage.Open(filepath.Join(t.TempDir(), "crash.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	seen := make(map[string]int)
	latencies := make([]time.Duration, 0, markets*len(rules))
	started := time.Now()
	for step := 1; step <= 110; step++ {
		at := base.Add(5*time.Minute + time.Duration(step)*100*time.Millisecond)
		for index, symbol := range symbols {
			drop := 10 + 50*float64(index)/float64(markets-1)
			price := 100 - drop*float64(min(step, 80))/80
			if step > 80 {
				price = (100 - drop) * (1 + .5*math.Abs(math.Sin(float64(step-80))))
			}
			for _, candidate := range engine.Process(quote(symbol, price, at)) {
				if candidate.Direction != "down" || step > 80 {
					t.Fatalf("late/rebound crossing for %s rule %d at step %d: %+v", symbol, candidate.Rule.ID, step, candidate)
				}
				before := time.Now()
				if _, err := store.InsertAlert(candidate); err != nil {
					t.Fatal(err)
				}
				latencies = append(latencies, time.Since(before))
				seen[fmt.Sprintf("%s/%d", candidate.Symbol, candidate.Rule.ID)]++
			}
		}
	}
	if len(seen) != markets*len(rules) {
		t.Fatalf("lost crossings: got %d, want %d", len(seen), markets*len(rules))
	}
	for key, count := range seen {
		if count != 1 {
			t.Fatalf("continuation/rebound spam for %s: %d alerts", key, count)
		}
	}
	cursor, persisted := int64(0), 0
	for {
		page, err := store.AlertsAfter(context.Background(), cursor, 200)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		persisted += len(page)
		cursor = page[len(page)-1].ID
	}
	if persisted != markets*len(rules) {
		t.Fatalf("incomplete cursor recovery: got %d", persisted)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	t.Logf("750 markets, 82,500 volatile ticks, %d alerts: total=%s sqlite p99=%s", persisted, time.Since(started), latencies[len(latencies)*99/100])
}
