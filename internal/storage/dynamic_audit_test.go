package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"cryptotape/internal/core"
)

// this model is deliberately test-only; no dynamic updates enter the application.
type dynamicPeak struct {
	ID      int64     `json:"id"`
	Change  float64   `json:"bestChangePercent"`
	Origin  time.Time `json:"-"`
	Expires time.Time `json:"-"`
}

func (p *dynamicPeak) observe(at time.Time, change float64) bool {
	improved := p.Change < 0 && change < p.Change || p.Change > 0 && change > p.Change
	if at.Before(p.Origin) || !at.Before(p.Expires) || !improved {
		return false
	}
	p.Change = change
	return true
}

func TestDynamicPeakWindowAndRebound(t *testing.T) {
	base := time.Unix(1800000000, 0)
	peak := dynamicPeak{ID: 1, Change: -10, Origin: base, Expires: base.Add(5 * time.Minute)}
	if !peak.observe(base.Add(time.Minute), -20) || peak.observe(base.Add(2*time.Minute), -15) || peak.observe(peak.Expires, -30) {
		t.Fatal("peak/window semantics changed")
	}
	if peak.observe(base.Add(3*time.Minute), 30) {
		t.Fatal("opposite direction changed peak")
	}
	if peak.Change != -20 || peak.ID != 1 {
		t.Fatal("rebound or expiry replaced the peak")
	}
}

func TestDynamicLoadAudit(t *testing.T) {
	if os.Getenv("CRYPTOTAPE_DYNAMIC_AUDIT") != "1" {
		t.Skip("opt-in SQLite and serialization load model")
	}
	for _, rules := range []int{2, 10} {
		for _, mode := range []string{"every_tick", "batch_250ms", "ui_250ms_db_1s"} {
			t.Run(fmt.Sprintf("rules=%d/%s", rules, mode), func(t *testing.T) { auditDynamicLoad(t, rules, mode) })
		}
	}
}

func auditDynamicLoad(t *testing.T, rules int, mode string) {
	const markets, steps = 750, 100
	count := markets * rules
	path := filepath.Join(t.TempDir(), "dynamic.sqlite")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	base := time.Now()
	candidates := make([]core.Candidate, count)
	for i := range candidates {
		candidates[i] = core.Candidate{Symbol: fmt.Sprintf("COIN%dUSDT", i/rules), Direction: "down", ChangePercent: -10, Price: 90, EventTime: base, Received: base, Detected: base}
	}
	initial, err := store.InsertAlerts(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	peaks := make([]dynamicPeak, count)
	dirtyUI, dirtyDB := make([]bool, count), make([]bool, count)
	for i, alert := range initial {
		peaks[i] = dynamicPeak{ID: alert.ID, Change: -10, Origin: base, Expires: base.Add(5 * time.Minute)}
	}
	statement, err := store.db.Prepare("UPDATE alerts SET change_percent=? WHERE id=?")
	if err != nil {
		t.Fatal(err)
	}
	defer statement.Close()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started, cpuStart := time.Now(), auditCPU()
	var updates, publications, wireBytes, walPeak int64
	samples := make(chan time.Duration, 65536)
	stop := make(chan struct{})
	stopped := make(chan struct{})
	probeErrors := make(chan error, 1)
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				at := time.Now()
				_, err := store.InsertAlert(core.Candidate{Symbol: "CONCURRENTUSDT", EventTime: at, Received: at, Detected: at})
				if err != nil {
					probeErrors <- err
					return
				}
				samples <- time.Since(at)
			}
		}
	}()
	emit := func(patches []dynamicPeak) {
		if len(patches) == 0 {
			return
		}
		raw, err := json.Marshal(patches)
		if err != nil {
			t.Fatal(err)
		}
		publications++
		wireBytes += int64(len(raw)) * 2
	}
	probeNew := func() {
		at := time.Now()
		candidate := core.Candidate{Symbol: "NEWUSDT", Direction: "down", EventTime: at, Received: at, Detected: at}
		if _, err := store.InsertAlert(candidate); err != nil {
			t.Fatal(err)
		}
		samples <- time.Since(at)
	}
	flush := func() {
		tx, err := store.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		prepared := tx.Stmt(statement)
		for i := range peaks {
			if dirtyDB[i] {
				if _, err := prepared.Exec(peaks[i].Change, peaks[i].ID); err != nil {
					tx.Rollback()
					t.Fatal(err)
				}
				updates++
				dirtyDB[i] = false
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	for step := 1; step <= steps; step++ {
		at := base.Add(time.Duration(step) * 20 * time.Millisecond)
		// a fresh crossing is serviced before continuation updates in every cycle.
		probeNew()
		for i := range peaks {
			change := -10 - float64(step)/5
			if !peaks[i].observe(at, change) {
				continue
			}
			dirtyUI[i], dirtyDB[i] = true, true
			if mode == "every_tick" {
				if _, err := statement.Exec(peaks[i].Change, peaks[i].ID); err != nil {
					t.Fatal(err)
				}
				updates++
				dirtyDB[i] = false
				emit(peaks[i : i+1])
				dirtyUI[i] = false
			}
		}
		if mode != "every_tick" && (step*20/250 != (step-1)*20/250 || step == steps) {
			patches := make([]dynamicPeak, 0, count)
			for i := range peaks {
				if dirtyUI[i] {
					patches = append(patches, peaks[i])
					dirtyUI[i] = false
				}
			}
			emit(patches)
		}
		if mode == "batch_250ms" && step*20/250 != (step-1)*20/250 || mode == "ui_250ms_db_1s" && step%50 == 0 {
			flush()
		}
		if info, err := os.Stat(path + "-wal"); err == nil {
			walPeak = max(walPeak, info.Size())
		}
	}
	flush()
	close(stop)
	<-stopped
	close(samples)
	select {
	case err := <-probeErrors:
		t.Fatal(err)
	default:
	}
	var detection []time.Duration
	for sample := range samples {
		detection = append(detection, sample)
	}
	elapsed, cpu := time.Since(started), auditCPU()-cpuStart
	runtime.GC()
	runtime.ReadMemStats(&after)
	sort.Slice(detection, func(i, j int) bool { return detection[i] < detection[j] })
	var final float64
	if err := store.db.QueryRow("SELECT change_percent FROM alerts WHERE id=?", peaks[0].ID).Scan(&final); err != nil {
		t.Fatal(err)
	}
	if final != -30 {
		t.Fatalf("final peak lost: %f", final)
	}
	heapDelta := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("active=%d model_state_bytes=%d simulated=2s ticks=%d elapsed=%s cpu=%s live_heap_delta=%d allocated=%d mallocs=%d db_updates=%d publications=%d two_webview_bytes=%d wal_peak=%d new_p99=%s", count, count*(int(reflect.TypeOf(dynamicPeak{}).Size())+2), markets*steps, elapsed, cpu, heapDelta, after.TotalAlloc-before.TotalAlloc, after.Mallocs-before.Mallocs, updates, publications, wireBytes, walPeak, detection[len(detection)*99/100])
	runtime.KeepAlive(peaks)
}

func BenchmarkDynamicPeakObservation(b *testing.B) {
	for _, count := range []int{1500, 7500} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			base := time.Unix(1800000000, 0)
			peaks := make([]dynamicPeak, count)
			for i := range peaks {
				peaks[i] = dynamicPeak{ID: int64(i + 1), Change: -10, Origin: base, Expires: base.Add(5 * time.Minute)}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				peaks[i%count].observe(base.Add(time.Duration(i%100)*time.Millisecond), -10-float64(i)/1e9)
			}
			runtime.KeepAlive(peaks)
		})
	}
}
