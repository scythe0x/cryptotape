package desktop

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func TestCancelledActionsDropQueuedNotificationsAndTerminalCommands(t *testing.T) {
	var events atomic.Int64
	c, err := NewController(filepath.Join(t.TempDir(), "data.sqlite"), func(name string, _ any) {
		if name == "alert:new" {
			events.Add(1)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := c.actionContext()
	c.actionCancel()
	if ctx.Err() != context.Canceled {
		t.Fatal("action contexts remain active")
	}
	c.actionWG.Add(1)
	c.notifications <- core.Alert{ID: 123}
	c.actionWG.Wait()
	if events.Load() != 0 {
		t.Fatal("notification escaped after cancellation")
	}
	if result := c.OpenTerminals(0, "BTCUSDT", []string{"vataga"}); result.Error == "" {
		t.Fatal("manual command escaped")
	}
	complete := make(chan error, 1)
	c.enqueueTerminal(terminalJob{terminal: "vataga", expires: time.Now().Add(time.Minute), complete: func(err error, _ string) { complete <- err }})
	select {
	case err := <-complete:
		if err != context.Canceled {
			t.Fatal("queued command not cancelled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queue stalled")
	}
}

func TestCloseCancelsActionsAndRejectsNewCommands(t *testing.T) {
	controller, err := NewController(filepath.Join(t.TempDir(), "data.sqlite"), func(string, any) {})
	if err != nil {
		t.Fatal(err)
	}
	ctx := controller.actionContext()
	if ctx.Err() != nil {
		t.Fatal("new controller has no active action context")
	}
	if err := controller.Close(); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != context.Canceled {
		t.Fatal("action context survived shutdown")
	}
	if result := controller.OpenTerminals(0, "BTCUSDT", []string{"vataga"}); result.Error != "controller closed" {
		t.Fatalf("command admitted after shutdown: %+v", result)
	}
	if err := controller.Close(); err != nil {
		t.Fatal("repeated close failed", err)
	}
}
