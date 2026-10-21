package core

import (
	"testing"
	"time"
)

func TestWindowsUseExchangeTimeDespiteDeliveryJitter(t *testing.T) {
	base := time.Unix(1800000000, 0)
	for _, test := range []struct {
		name          string
		first, second Quote
		want          int
	}{
		{"late baseline cannot shorten movement", quote("BTCUSDT", 100, base.Add(-7*time.Second)), quote("BTCUSDT", 89, base), 0},
		{"late crossing cannot lengthen movement", quote("BTCUSDT", 100, base), quote("BTCUSDT", 89, base.Add(time.Second)), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := NewEngine([]Rule{{ID: 1, Percent: 10, WindowSeconds: 5}})
			test.first.Received = base
			test.second.Received = base.Add(7 * time.Second)
			if test.want == 0 {
				test.second.Received = base.Add(time.Millisecond)
			}
			engine.Process(test.first)
			if got := len(engine.Process(test.second)); got != test.want {
				t.Fatalf("got %d alerts, want %d", got, test.want)
			}
		})
	}
}

func TestUpdateOrderAndEqualEventTimestamps(t *testing.T) {
	engine := NewEngine([]Rule{{ID: 1, Percent: 5, WindowSeconds: 300}})
	at := time.Unix(1800000000, 0)
	first := quote("BTCUSDT", 100, at)
	first.UpdateID = 100
	engine.Process(first)
	for _, id := range []uint64{100, 99} {
		q := quote("BTCUSDT", 80, at)
		q.UpdateID = id
		if got := engine.Process(q); len(got) != 0 {
			t.Fatalf("duplicate/out-of-order id accepted: %d", id)
		}
	}
	next := quote("BTCUSDT", 94, at)
	next.UpdateID = 101
	if got := engine.Process(next); len(got) != 1 {
		t.Fatalf("distinct update in same millisecond was lost: %+v", got)
	}
}

func TestClockCorrectionDoesNotReindexWindows(t *testing.T) {
	engine := NewEngine([]Rule{{ID: 1, Percent: 10, WindowSeconds: 5}})
	local := time.Unix(1800000000, 0)
	first := quote("BTCUSDT", 100, local.Add(time.Hour))
	first.Received, first.ClockOffset = local, time.Hour
	engine.Process(first)
	next := quote("BTCUSDT", 89, first.EventTime.Add(time.Second))
	next.Received, next.ClockOffset = local.Add(500*time.Millisecond), time.Hour+500*time.Millisecond
	got := engine.Process(next)
	if len(got) != 1 || !got[0].EventTime.Equal(next.EventTime) {
		t.Fatalf("raw event time was changed: %+v", got)
	}
	if delta := got[0].Expires.Sub(next.Received); delta != 10*time.Second {
		t.Fatalf("incorrect local freshness deadline: %s", delta)
	}
}
