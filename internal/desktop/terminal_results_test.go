package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func TestTerminalWorkersDoNotWaitForStatusPersistence(t *testing.T) {
	var published atomic.Int32
	c, path := testController(t, func(name string, _ any) {
		if name == "alert:update" {
			published.Add(1)
		}
	})
	candidates := make([]core.Candidate, 64)
	for i := range candidates {
		candidates[i] = core.Candidate{Symbol: fmt.Sprintf("COIN%dUSDT", i), Rule: core.Rule{ID: 1, Terminals: []string{"vataga"}}}
	}
	alerts, err := c.store.InsertAlerts(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	received := make(chan struct{}, len(alerts))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received <- struct{}{}
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()
	c.terminal.baseURL = server.URL
	lock, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err := lock.Exec("BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer lock.Exec("ROLLBACK")
	for _, alert := range alerts {
		c.enqueueAutomatic(alert, time.Now().Add(10*time.Second))
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for range alerts {
		select {
		case <-received:
		case <-deadline.C:
			t.Fatal("terminal workers waited for locked status storage")
		}
	}
	if published.Load() != 0 {
		t.Fatal("status published before commit")
	}
	if _, err := lock.Exec("COMMIT"); err != nil {
		t.Fatal(err)
	}
	c.actionWG.Wait()
	saved, err := c.AlertsAfter(0, 200)
	if err != nil {
		t.Fatal(err)
	}
	if published.Load() != 64 || len(saved) != 64 || c.terminalErrors.Load() != 0 {
		t.Fatalf("published=%d saved=%d errors=%d", published.Load(), len(saved), c.terminalErrors.Load())
	}
	for _, alert := range saved {
		if alert.TerminalStatus != "ok" {
			t.Fatalf("status not persisted: %+v", alert)
		}
	}
}

func TestTerminalResultOverflowIsReported(t *testing.T) {
	c := &Controller{terminalResults: make(chan core.AlertUpdate, 1)}
	c.saveTerminalResult(1, "ok", "")
	c.saveTerminalResult(2, "ok", "")
	if c.terminalErrors.Load() != 1 || c.pipelineHealth().Error != "terminal result queue overflow" || c.pipelineError == nil {
		t.Fatal("terminal result overflow was hidden")
	}
	c.actionWG.Done()
}

func TestTerminalResultWriteFailureIsReturnedOnClose(t *testing.T) {
	c, err := NewController(t.TempDir()+"/failure.sqlite", func(string, any) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.store.Close(); err != nil {
		t.Fatal(err)
	}
	c.saveTerminalResult(1, "ok", "")
	c.actionWG.Wait()
	if c.terminalErrors.Load() != 1 || c.pipelineHealth().Error == "" {
		t.Error("terminal result write failure was hidden")
	}
	if err := c.Close(); err == nil {
		t.Fatal("shutdown hid terminal result write failure")
	}
}
