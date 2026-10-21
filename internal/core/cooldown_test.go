package core

import (
	"fmt"
	"testing"
	"time"
)

func TestCooldownMatchesRuleWindow(t *testing.T) {
	for _, seconds := range []int{5, 60, 300, 600} {
		for _, direction := range []string{"up", "down"} {
			t.Run(fmt.Sprintf("%ds/%s", seconds, direction), func(t *testing.T) {
				engine := NewEngine([]Rule{{ID: 1, Percent: 10, WindowSeconds: seconds}})
				base := time.Unix(1_800_000_000, 0)
				price := 110.0
				if direction == "down" {
					price = 90
				}
				engine.Process(quote("BTCUSDT", 100, base))
				if got := engine.Process(quote("BTCUSDT", price, base.Add(time.Second))); len(got) != 1 || got[0].Direction != direction {
					t.Fatalf("first crossing: %+v", got)
				}

				for second := 2; second <= seconds; second++ {
					if got := engine.Process(quote("BTCUSDT", 100, base.Add(time.Duration(second)*time.Second))); len(got) != 0 {
						t.Fatalf("rebound during cooldown at %ds: %+v", second, got)
					}
					at := base.Add(time.Duration(second)*time.Second + 500*time.Millisecond)
					if got := engine.Process(quote("BTCUSDT", price, at)); len(got) != 0 {
						t.Fatalf("repeat before window cooldown expired at %s: %+v", at.Sub(base), got)
					}
				}

				deadline := base.Add(time.Duration(seconds+1) * time.Second)
				if got := engine.Process(quote("BTCUSDT", 100, deadline.Add(-time.Millisecond))); len(got) != 0 {
					t.Fatalf("rebound just before deadline: %+v", got)
				}
				if got := engine.Process(quote("BTCUSDT", price, deadline)); len(got) != 1 || got[0].Direction != direction {
					t.Fatalf("fresh crossing at cooldown deadline: %+v", got)
				}
			})
		}
	}
}

func TestWindowCooldownIsIndependentForEachSymbolAndRule(t *testing.T) {
	engine := NewEngine([]Rule{
		{ID: 1, Percent: 10, WindowSeconds: 5},
		{ID: 2, Percent: 10, WindowSeconds: 10},
	})
	base := time.Unix(1_800_000_000, 0)
	engine.Process(quote("BTCUSDT", 100, base))
	if got := engine.Process(quote("BTCUSDT", 110, base.Add(time.Second))); len(got) != 2 {
		t.Fatalf("first crossings: %+v", got)
	}
	engine.Process(quote("ETHUSDT", 100, base.Add(time.Second)))
	if got := engine.Process(quote("ETHUSDT", 110, base.Add(2*time.Second))); len(got) != 2 {
		t.Fatalf("another symbol was blocked: %+v", got)
	}
	for second := 2; second <= 10; second++ {
		price := 100.0
		if second == 6 {
			price = 110
		}
		got := engine.Process(quote("BTCUSDT", price, base.Add(time.Duration(second)*time.Second)))
		if second == 6 {
			if len(got) != 1 || got[0].Rule.ID != 1 {
				t.Fatalf("shorter rule did not rearm independently: %+v", got)
			}
		} else if len(got) != 0 {
			t.Fatalf("unexpected signal at %ds: %+v", second, got)
		}
	}
	if got := engine.Process(quote("BTCUSDT", 110, base.Add(11*time.Second))); len(got) != 2 {
		t.Fatalf("rules did not rearm at their own deadlines: %+v", got)
	}
}

func TestWindowCooldownSuppressesAGTRepeat(t *testing.T) {
	engine := NewEngine([]Rule{{ID: 2, Percent: 10, WindowSeconds: 300}})
	base := time.Unix(1_800_000_000, 0)
	for second := -240; second <= 63; second++ {
		price := 0.060
		switch {
		case second == -240:
			price = 0.055821
		case second == -200:
			price = 0.055964
		case second == 63:
			price = 0.061566
		case second >= 0:
			price = 0.0614235
		}
		got := engine.Process(quote("AGTUSDT", price, base.Add(time.Duration(second)*time.Second)))
		if second == 0 {
			if len(got) != 1 || got[0].Direction != "up" {
				t.Fatalf("first AGT crossing: %+v", got)
			}
		} else if len(got) != 0 {
			t.Fatalf("duplicate AGT signal at %ds: %+v", second, got)
		}
	}
}

func TestWindowCooldownDoesNotQueueCrossings(t *testing.T) {
	engine := NewEngine([]Rule{{ID: 1, Percent: 10, WindowSeconds: 300}})
	base := time.Unix(1_800_000_000, 0)
	for second := 0; second <= 304; second++ {
		price := 100.0
		if second == 1 || second == 300 || second == 304 {
			price = 110
		}
		if second == 301 || second == 302 {
			price = 111
		}
		got := engine.Process(quote("BTCUSDT", price, base.Add(time.Duration(second)*time.Second)))
		if second == 1 || second == 304 {
			if len(got) != 1 || got[0].Direction != "up" {
				t.Fatalf("fresh crossing at %ds: %+v", second, got)
			}
		} else if len(got) != 0 {
			t.Fatalf("suppressed crossing replayed at %ds: %+v", second, got)
		}
	}
}
