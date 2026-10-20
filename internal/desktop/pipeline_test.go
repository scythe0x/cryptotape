package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func TestSQLiteLockDoesNotBlockQuoteReader(t *testing.T) {
	published := make(chan core.Alert, 1)
	controller, path := testController(t, func(name string, data any) {
		if name == "alert:new" {
			published <- data.(core.Alert)
		}
	})
	if _, err := controller.SaveRule(lifecycleRule()); err != nil {
		t.Fatal(err)
	}
	controller.engine.SetVolumes(map[string]float64{"BTCUSDT": 100e6})
	controller.onQuote(marketQuote("BTCUSDT", 100, time.Now().Add(-time.Second)))
	lock, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err := lock.Exec("BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer lock.Exec("ROLLBACK")
	done := make(chan struct{})
	started := time.Now()
	go func() { controller.onQuote(marketQuote("BTCUSDT", 94, started)); close(done) }()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("quote callback waited for SQLite")
	}
	callback := time.Since(started)
	select {
	case <-published:
		t.Fatal("published before commit")
	case <-time.After(200 * time.Millisecond):
	}
	if _, err := lock.Exec("COMMIT"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-published:
	case <-time.After(2 * time.Second):
		t.Fatal("accepted candidate was lost")
	}
	controller.actionWG.Wait()
	t.Logf("SQLite held for 200ms; callback returned in %s", callback)
}

func TestCandidateOverflowRemainsVisibleAfterHealthyFeed(t *testing.T) {
	c := &Controller{engine: core.NewEngine([]core.Rule{{ID: 1, Percent: 5, WindowSeconds: 300}}, core.DefaultSettings()), candidates: make(chan queuedCandidate, 1), pending: make(map[uint64]time.Time), emit: func(string, any) {}}
	for _, symbol := range []string{"AAAUSDT", "BBBUSDT"} {
		c.onQuote(marketQuote(symbol, 100, time.Now()))
		c.onQuote(marketQuote(symbol, 94, time.Now()))
	}
	c.setFeedHealth(0, core.Health{State: "warming"})
	if c.health.Storage.Unsaved != 1 || c.health.Storage.Error == "" || len(c.candidates) != 1 {
		t.Fatalf("overflow hidden: %+v", c.health)
	}
	c.actionWG.Done()
}

func TestCrashPipelinePersistsAllCandidates(t *testing.T) {
	published := make(chan core.Alert, 1500)
	c, _ := testController(t, func(name string, data any) {
		if name == "alert:new" {
			published <- data.(core.Alert)
		}
	})
	for _, seconds := range []int{60, 300} {
		r := lifecycleRule()
		r.WindowSeconds = seconds
		if _, err := c.SaveRule(r); err != nil {
			t.Fatal(err)
		}
	}
	volumes := make(map[string]float64)
	for i := range 750 {
		volumes[fmt.Sprintf("COIN%dUSDT", i)] = 100e6
	}
	c.engine.SetVolumes(volumes)
	base := time.Now().Add(-time.Second)
	for symbol := range volumes {
		c.onQuote(marketQuote(symbol, 100, base))
	}
	started := time.Now()
	for symbol := range volumes {
		c.onQuote(marketQuote(symbol, 85, started))
	}
	c.actionWG.Wait()
	if len(published) != 1500 || c.pipelineHealth().Unsaved != 0 {
		t.Fatalf("lost alerts: %d %+v", len(published), c.pipelineHealth())
	}
	cursor, count := int64(0), 0
	for {
		page, err := c.AlertsAfter(cursor, 200)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		count += len(page)
		cursor = page[len(page)-1].ID
	}
	if count != 1500 {
		t.Fatalf("persisted %d/1500", count)
	}
	t.Logf("1500 committed and dispatched alerts: %s", time.Since(started))
}

func TestSlowOverlayDoesNotDelayTerminal(t *testing.T) {
	block := make(chan struct{})
	started := make(chan struct{}, 1)
	c, _ := testController(t, func(name string, _ any) {
		if name == "alert:new" {
			started <- struct{}{}
			<-block
		}
	})
	defer close(block)
	requests := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests <- struct{}{}; fmt.Fprint(w, `{"status":"ok"}`) }))
	defer server.Close()
	c.terminal.baseURL = server.URL
	r := lifecycleRule()
	r.Terminals = []string{"vataga"}
	if _, err := c.SaveRule(r); err != nil {
		t.Fatal(err)
	}
	c.engine.SetVolumes(map[string]float64{"BTCUSDT": 100e6})
	c.onQuote(marketQuote("BTCUSDT", 100, time.Now().Add(-time.Second)))
	c.onQuote(marketQuote("BTCUSDT", 94, time.Now()))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("no overlay event")
	}
	select {
	case <-requests:
	case <-time.After(time.Second):
		t.Fatal("terminal blocked by overlay")
	}
}

func TestQueuedCrashOpens750MarketsWithSlowResponses(t *testing.T) {
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		received.Add(1)
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()
	c, _ := testController(t, func(string, any) {})
	c.terminal.baseURL = server.URL
	candidates := make([]core.Candidate, 750)
	prepared := time.Now()
	for i := range 750 {
		candidates[i] = core.Candidate{Rule: core.Rule{ID: 1, Terminals: []string{"vataga"}}, Symbol: fmt.Sprintf("COIN%dUSDT", i), EventTime: prepared, Received: prepared, Detected: prepared}
	}
	alerts, err := c.store.InsertAlerts(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	// measure the terminal queue, excluding database fixture preparation.
	started := time.Now()
	for _, alert := range alerts {
		c.enqueueAutomatic(alert, started.Add(10*time.Second))
	}
	c.actionWG.Wait()
	if received.Load() != 750 || c.terminalErrors.Load() != 0 {
		t.Fatalf("received=%d errors=%d", received.Load(), c.terminalErrors.Load())
	}
	t.Logf("750 commands with 100ms responses: %s", time.Since(started))
}

func TestQueuedExpiredCommandIsReportedWithoutSending(t *testing.T) {
	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { sent.Add(1); fmt.Fprint(w, `{"status":"ok"}`) }))
	defer server.Close()
	c, _ := testController(t, func(string, any) {})
	c.terminal.baseURL = server.URL
	completed := make(chan string, 1)
	c.enqueueTerminal(terminalJob{symbol: "BTCUSDT", terminal: "vataga", queued: time.Now().Add(-11 * time.Second), expires: time.Now().Add(-time.Second), complete: func(err error, status string) {
		if err == nil {
			completed <- "missing error"
		} else {
			completed <- status
		}
	}})
	c.actionWG.Wait()
	if status := <-completed; status != "stale" || sent.Load() != 0 {
		t.Fatalf("status=%s sent=%d", status, sent.Load())
	}
}

func TestCloseDrainsAcceptedCandidatesAndRejectsNewQuotes(t *testing.T) {
	c, err := NewController(t.TempDir()+"/close.sqlite", func(string, any) {})
	if err != nil {
		t.Fatal(err)
	}
	c.engine.SetVolumes(map[string]float64{"BTCUSDT": 100e6})
	c.onQuote(marketQuote("BTCUSDT", 100, time.Now().Add(-time.Second)))
	c.onQuote(marketQuote("BTCUSDT", 85, time.Now()))
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	c.onQuote(marketQuote("BTCUSDT", 120, time.Now()))
	reopened, err := NewController(filepath.Join(c.audio.dir, "..", "close.sqlite"), func(string, any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	history, err := reopened.ListAlerts(0, 10)
	if err != nil || len(history) != 2 {
		t.Fatalf("shutdown lost accepted alerts: %d %v", len(history), err)
	}
}

func TestStorageFailureIsLatchedAndReturnedOnClose(t *testing.T) {
	c, err := NewController(filepath.Join(t.TempDir(), "failure.sqlite"), func(string, any) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.store.Close(); err != nil {
		t.Fatal(err)
	}
	c.engine.SetVolumes(map[string]float64{"BTCUSDT": 100e6})
	c.onQuote(marketQuote("BTCUSDT", 100, time.Now().Add(-time.Second)))
	c.onQuote(marketQuote("BTCUSDT", 85, time.Now()))
	c.actionWG.Wait()
	c.setFeedHealth(0, core.Health{State: "warming"})
	if c.health.Storage.Unsaved != 2 || c.health.Storage.Error == "" {
		t.Fatalf("storage failure hidden: %+v", c.health.Storage)
	}
	if err := c.Close(); err == nil {
		t.Fatal("shutdown hid unsaved candidates")
	}
}
