package desktop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func TestDelayedAlertDoesNotOpenTerminal(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	controller, _ := testController(t, func(string, any) {})
	store := controller.store
	alert, err := store.InsertAlert(core.Candidate{
		Rule:   core.Rule{ID: 1, Name: "knife", Terminals: []string{"vataga"}},
		Symbol: "BTCUSDT", Direction: "down", EventTime: time.Now().Add(-11 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	terminal := NewTerminal()
	terminal.baseURL = server.URL
	controller.terminal = terminal
	controller.enqueueAutomatic(alert, time.Now().Add(-time.Second))
	controller.actionWG.Wait()
	if requests.Load() != 0 {
		t.Fatal("stale alert opened a market")
	}
	history, err := store.ListAlerts(context.Background(), 0, 1)
	if err != nil || len(history) != 1 || history[0].TerminalStatus != "stale" {
		t.Fatalf("stale status was not retained: %+v, %v", history, err)
	}
}

func TestDeletingAllRulesKeepsSnapshotUsable(t *testing.T) {
	controller, err := NewController(filepath.Join(t.TempDir(), "empty.sqlite"), func(string, any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	snapshot, err := controller.GetSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range snapshot.Rules {
		if err = controller.DeleteRule(rule.ID); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err = controller.GetSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Rules == nil || len(snapshot.Rules) != 0 {
		t.Fatalf("invalid empty rule list: %+v", snapshot.Rules)
	}
}
