package desktop

import (
	"context"
	"testing"
)

func TestClockOffsetFallsBackWhenFeedIsUnavailable(t *testing.T) {
	controller := &Controller{}
	if _, err := controller.ClockOffset(context.Background()); err == nil {
		t.Fatal("unavailable feed returned a synchronized offset")
	}
	controller.SetAccessCheck(func() bool { return false })
	if _, err := controller.ClockOffset(context.Background()); err == nil {
		t.Fatal("clock synchronized without access")
	}
}
