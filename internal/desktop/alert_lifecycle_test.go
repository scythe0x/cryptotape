package desktop

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func testController(t *testing.T, emit EventSink) (*Controller, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "alerts.sqlite")
	controller, err := NewController(path, emit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := controller.Close(); err != nil {
			t.Error(err)
		}
	})
	snapshot, err := controller.GetSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range snapshot.Rules {
		if err := controller.DeleteRule(rule.ID); err != nil {
			t.Fatal(err)
		}
	}
	return controller, path
}

func lifecycleRule() core.Rule {
	return core.Rule{Name: "нож", Percent: 5, WindowSeconds: 300, Sound: "none",
		Color: "#46c6df", Volume: .4, MinQuoteVolume: 80e6, Terminals: []string{}}
}

func marketQuote(symbol string, price float64, at time.Time) core.Quote {
	return core.Quote{Symbol: symbol, Bid: price * .9999, Ask: price * 1.0001, EventTime: at, Received: at}
}

func TestCreatedRulePersistsAndImmediatelyDetectsKnife(t *testing.T) {
	published := make(chan core.Alert, 10)
	var controller *Controller
	var path string
	controller, path = testController(t, func(name string, data any) {
		if name != "alert:new" {
			return
		}
		alert, ok := data.(core.Alert)
		if !ok {
			t.Error("invalid alert event")
			return
		}
		history, err := controller.AlertsAfter(alert.ID-1, 1)
		if err != nil || len(history) != 1 || !reflect.DeepEqual(history[0], alert) {
			t.Errorf("alert published before durable history: %+v %v", history, err)
		}
		published <- alert
	})
	rule, err := controller.SaveRule(lifecycleRule())
	if err != nil || rule.ID <= 0 {
		t.Fatalf("create rule: %+v %v", rule, err)
	}
	controller.engine.SetVolumes(map[string]float64{"BTCUSDT": 100e6, "LOWUSDT": 10e6})
	base := time.Now().Add(-time.Second)
	for _, symbol := range []string{"BTCUSDT", "LOWUSDT"} {
		controller.onQuote(marketQuote(symbol, 100, base))
		controller.onQuote(marketQuote(symbol, 94, base.Add(time.Second)))
		controller.onQuote(marketQuote(symbol, 85, base.Add(1500*time.Millisecond)))
		controller.onQuote(marketQuote(symbol, 100, base.Add(2*time.Second)))
	}
	controller.actionWG.Wait()
	if len(published) != 1 {
		t.Fatalf("wanted one knife alert, got %d", len(published))
	}
	alert := <-published
	if alert.Symbol != "BTCUSDT" || alert.RuleID != rule.ID || alert.Direction != "down" ||
		alert.RuleName != rule.Name || alert.Color != rule.Color || alert.Sound != "none" ||
		alert.Volume != rule.Volume || alert.TerminalStatus != "skipped" {
		t.Fatalf("wrong alert configuration: %+v", alert)
	}
	if controller.audio.latest != alert.ID {
		t.Fatal("sound action was not dispatched")
	}
	session, err := controller.SessionAlerts()
	if err != nil || len(session) != 1 || session[0].ID != alert.ID {
		t.Fatalf("notification recovery missed alert: %+v %v", session, err)
	}

	// a second controller reopens the real database, as on the next launch.
	reopened, err := NewController(path, func(string, any) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	})
	snapshot, err := reopened.GetSnapshot()
	if err != nil || len(snapshot.Rules) != 1 || !reflect.DeepEqual(snapshot.Rules[0], rule) ||
		len(snapshot.Alerts) != 1 || snapshot.Alerts[0].ID != alert.ID {
		t.Fatalf("rule/history not retained: %+v %v", snapshot, err)
	}
	session, err = reopened.SessionAlerts()
	if err != nil || len(session) != 0 {
		t.Fatalf("old alerts replayed after restart: %+v %v", session, err)
	}
}

func TestRuleEditsAndDeletionApplyToRunningEngine(t *testing.T) {
	controller, _ := testController(t, func(string, any) {})
	rule, err := controller.SaveRule(lifecycleRule())
	if err != nil {
		t.Fatal(err)
	}
	controller.engine.SetVolumes(map[string]float64{"BTCUSDT": 100e6})
	base := time.Now().Add(-time.Second)
	controller.onQuote(marketQuote("BTCUSDT", 100, base))
	rule.Percent, rule.MinQuoteVolume, rule.Color = 10, 120e6, "#41c977"
	rule, err = controller.SaveRule(rule)
	if err != nil {
		t.Fatal(err)
	}
	controller.onQuote(marketQuote("BTCUSDT", 94, base.Add(100*time.Millisecond)))
	history, err := controller.ListAlerts(0, 10)
	if err != nil || len(history) != 0 {
		t.Fatalf("old threshold remained active: %+v %v", history, err)
	}
	controller.onQuote(marketQuote("BTCUSDT", 89, base.Add(200*time.Millisecond)))
	controller.engine.SetVolumes(map[string]float64{"BTCUSDT": 150e6})
	controller.onQuote(marketQuote("BTCUSDT", 88, base.Add(300*time.Millisecond)))
	history, err = controller.ListAlerts(0, 10)
	if err != nil || len(history) != 0 {
		t.Fatalf("volume filter or delayed crossing failed: %+v %v", history, err)
	}
	controller.onQuote(marketQuote("BTCUSDT", 100, base.Add(400*time.Millisecond)))
	controller.actionWG.Wait()
	history, err = controller.ListAlerts(0, 10)
	if err != nil || len(history) != 1 || history[0].Color != rule.Color || history[0].Direction != "up" {
		t.Fatalf("edited rule not applied: %+v %v", history, err)
	}
	if err := controller.DeleteRule(rule.ID); err != nil {
		t.Fatal(err)
	}
	controller.onQuote(marketQuote("BTCUSDT", 50, base.Add(70*time.Second)))
	controller.onQuote(marketQuote("BTCUSDT", 100, base.Add(71*time.Second)))
	history, err = controller.ListAlerts(0, 10)
	if err != nil || len(history) != 1 {
		t.Fatalf("deleted rule still fires: %+v %v", history, err)
	}
}

func TestAlertDispatchSurvivesTerminalFailure(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			requests := make(chan string, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Method + " " + r.URL.Path
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}))
			defer server.Close()
			published := make(chan core.Alert, 2)
			controller, _ := testController(t, func(name string, data any) {
				if name == "alert:new" {
					alert, ok := data.(core.Alert)
					if !ok {
						t.Error("invalid alert payload")
						return
					}
					published <- alert
				}
			})
			controller.terminal.baseURL = server.URL
			rule := lifecycleRule()
			rule.Terminals = []string{"vataga"}
			if _, err := controller.SaveRule(rule); err != nil {
				t.Fatal(err)
			}
			controller.engine.SetVolumes(map[string]float64{"BTCUSDT": 100e6})
			base := time.Now().Add(-time.Second)
			controller.onQuote(marketQuote("BTCUSDT", 100, base))
			controller.onQuote(marketQuote("BTCUSDT", 94, base.Add(time.Second)))
			controller.actionWG.Wait()
			if len(published) != 1 || len(requests) != 1 {
				t.Fatal("notification or terminal action lost")
			}
			alert := <-published
			if got := <-requests; got != "POST /api/open/zenica/binance/F/BTCUSDT" {
				t.Fatalf("wrong command: %s", got)
			}
			history, err := controller.ListAlerts(0, 10)
			expected := "ok"
			if status != http.StatusOK {
				expected = "error"
			}
			if err != nil || len(history) != 1 || history[0].TerminalStatus != expected {
				t.Fatalf("terminal outcome not persisted: %+v %v", history, err)
			}
			if expected == "error" && history[0].TerminalError == "" {
				t.Fatal("terminal failure hidden")
			}
			if controller.audio.latest != alert.ID {
				t.Fatal("terminal failure blocked sound action")
			}
		})
	}
}

func TestInvalidRuleDoesNotChangeActiveRules(t *testing.T) {
	controller, _ := testController(t, func(string, any) {})
	for _, change := range []func(*core.Rule){
		func(r *core.Rule) { r.Name = " " },
		func(r *core.Rule) { r.Percent = 0 },
		func(r *core.Rule) { r.WindowSeconds = 0 },
		func(r *core.Rule) { r.Sound = "missing" },
	} {
		rule := lifecycleRule()
		change(&rule)
		if _, err := controller.SaveRule(rule); err == nil {
			t.Fatalf("invalid rule accepted: %+v", rule)
		}
	}
	snapshot, err := controller.GetSnapshot()
	if err != nil || len(snapshot.Rules) != 0 {
		t.Fatalf("invalid rules leaked into engine: %+v %v", snapshot.Rules, err)
	}
}
