package engine

import (
	"testing"
	"time"

	"github.com/whoiscyberial/cryptotape/backend/types"
)

func TestProcessKeepsSameImpulseIDWhenBaseShiftsInsideWindow(t *testing.T) {
	rule := ImpulseRule{
		ThresholdPct: 1.0,
		MaxDuration:  5 * time.Second,
	}

	engine := NewEngine([]ImpulseRule{rule}, 2*time.Minute)
	alerts := make(chan types.Alert, 8)
	start := time.Unix(0, 0)

	process := func(offset time.Duration, price float64) []types.Alert {
		engine.Process(types.BookTickerUpdate{
			Symbol:   "TESTUSDT",
			MidPrice: price,
		}, alerts, start.Add(offset))

		out := make([]types.Alert, 0, len(alerts))
		for len(alerts) > 0 {
			out = append(out, <-alerts)
		}
		return out
	}

	if got := process(0, 100); len(got) != 0 {
		t.Fatalf("unexpected initial alerts: %+v", got)
	}

	first := process(3*time.Second, 101.5)
	if len(first) != 1 {
		t.Fatalf("expected first impulse alert, got %d", len(first))
	}

	if got := process(6*time.Second, 100.8); len(got) != 0 {
		t.Fatalf("unexpected retrace alerts: %+v", got)
	}

	second := process(7*time.Second, 102.0)
	if len(second) != 1 {
		t.Fatalf("expected continued impulse alert, got %d", len(second))
	}

	if first[0].ImpulseID != second[0].ImpulseID {
		t.Fatalf("expected same impulse id, got %q then %q", first[0].ImpulseID, second[0].ImpulseID)
	}

	if second[0].MaxPtc < second[0].CurrentPct {
		t.Fatalf("expected maxPtc >= currentPct, got max=%f current=%f", second[0].MaxPtc, second[0].CurrentPct)
	}
}

func TestProcessKeepsSameImpulseIDAfterRetraceInsideSameMove(t *testing.T) {
	rule := ImpulseRule{
		ThresholdPct: 1.0,
		MaxDuration:  5 * time.Second,
	}

	engine := NewEngine([]ImpulseRule{rule}, 2*time.Minute)
	alerts := make(chan types.Alert, 16)
	start := time.Unix(0, 0)

	process := func(offset time.Duration, price float64) []types.Alert {
		engine.Process(types.BookTickerUpdate{
			Symbol:   "TESTUSDT",
			MidPrice: price,
		}, alerts, start.Add(offset))

		out := make([]types.Alert, 0, len(alerts))
		for len(alerts) > 0 {
			out = append(out, <-alerts)
		}
		return out
	}

	process(0, 100.0)

	first := process(3*time.Second, 98.5)
	if len(first) != 1 {
		t.Fatalf("expected first impulse alert, got %d", len(first))
	}

	second := process(4*time.Second, 98.0)
	if len(second) != 1 {
		t.Fatalf("expected second impulse alert, got %d", len(second))
	}

	if second[0].ImpulseID != first[0].ImpulseID {
		t.Fatalf("expected same id on extension, got %q then %q", first[0].ImpulseID, second[0].ImpulseID)
	}

	if got := process(10*time.Second, 99.6); len(got) != 0 {
		t.Fatalf("unexpected retrace alert: %+v", got)
	}

	third := process(14*time.Second, 98.3)
	if len(third) != 1 {
		t.Fatalf("expected continued impulse after retrace, got %d", len(third))
	}

	if third[0].ImpulseID != first[0].ImpulseID {
		t.Fatalf("expected same id after retrace, got %q then %q", first[0].ImpulseID, third[0].ImpulseID)
	}

	if !(third[0].CurrentPct > third[0].MaxPtc) {
		// for a down move both values are negative, so correction means current becomes less negative than max
		t.Fatalf("expected corrected currentPct to be above maxPtc, got current=%f max=%f", third[0].CurrentPct, third[0].MaxPtc)
	}
}

func TestNewEngineUsesSampleRetainForLongImpulseWindow(t *testing.T) {
	rule := ImpulseRule{
		ThresholdPct: 1.0,
		MaxDuration:  5 * time.Second,
	}

	engine := NewEngine([]ImpulseRule{rule}, 30*time.Second)
	alerts := make(chan types.Alert, 16)
	start := time.Unix(0, 0)

	process := func(offset time.Duration, price float64) []types.Alert {
		engine.Process(types.BookTickerUpdate{
			Symbol:   "TESTUSDT",
			MidPrice: price,
		}, alerts, start.Add(offset))

		out := make([]types.Alert, 0, len(alerts))
		for len(alerts) > 0 {
			out = append(out, <-alerts)
		}
		return out
	}

	process(0, 100.0)

	first := process(4*time.Second, 101.4)
	if len(first) != 1 {
		t.Fatalf("expected first impulse alert, got %d", len(first))
	}

	second := process(20*time.Second, 102.6)
	if len(second) != 1 {
		t.Fatalf("expected continued long impulse alert, got %d", len(second))
	}

	if second[0].ImpulseID != first[0].ImpulseID {
		t.Fatalf("expected same id for long impulse, got %q then %q", first[0].ImpulseID, second[0].ImpulseID)
	}

	if second[0].DurationMs <= first[0].DurationMs {
		t.Fatalf("expected duration to keep growing, got first=%d second=%d", first[0].DurationMs, second[0].DurationMs)
	}
}
