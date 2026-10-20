package storage

import (
	"context"
	"path/filepath"
	"testing"

	"cryptotape/internal/core"
)

func TestTerminalResultBatchRollsBackOnFailure(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "results.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	alerts, err := s.InsertAlerts(context.Background(), []core.Candidate{{Symbol: "AAAUSDT"}, {Symbol: "BBBUSDT"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_terminal_result BEFORE UPDATE ON alerts WHEN OLD.symbol = 'BBBUSDT' BEGIN SELECT RAISE(ABORT, 'test write failure'); END`); err != nil {
		t.Fatal(err)
	}
	updates := []core.AlertUpdate{
		{ID: alerts[0].ID, TerminalStatus: "ok"},
		{ID: alerts[1].ID, TerminalStatus: "error", TerminalError: "rejected"},
	}
	if err := s.UpdateTerminalsContext(context.Background(), updates); err == nil {
		t.Fatal("write failure was hidden")
	}
	saved, err := s.AlertsAfter(context.Background(), 0, 200)
	if err != nil || len(saved) != 2 {
		t.Fatalf("saved=%d error=%v", len(saved), err)
	}
	for _, alert := range saved {
		if alert.TerminalStatus != "skipped" {
			t.Fatal("failed batch partially committed")
		}
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_terminal_result`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateTerminalsContext(context.Background(), updates); err != nil {
		t.Fatal(err)
	}
	saved, err = s.AlertsAfter(context.Background(), 0, 200)
	if err != nil || len(saved) != 2 || saved[0].TerminalStatus != "ok" || saved[1].TerminalStatus != "error" || saved[1].TerminalError != "rejected" {
		t.Fatalf("batch not saved: %+v %v", saved, err)
	}
}
