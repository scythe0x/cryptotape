package main

import (
	"context"
	"time"
)

func (s *DesktopService) ClockOffset() (float64, error) {
	ctx, cancel := context.WithTimeout(s.app.Context(), 4*time.Second)
	defer cancel()
	return s.controller.ClockOffset(ctx)
}
