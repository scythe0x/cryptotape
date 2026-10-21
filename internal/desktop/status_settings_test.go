package desktop

import (
	"testing"
	"time"

	"cryptotape/internal/core"
)

func TestPingMedianUsesLastFiveSamplesAndResets(t *testing.T) {
	c := &Controller{emit: func(string, any) {}}
	c.version.Store(1)
	c.setFeedHealth(1, core.Health{State: "connected"})
	samples := []int64{100, 900, 20, 80, 60, 40, 10}
	expected := []int64{100, 500, 100, 90, 80, 60, 40}
	for i, sample := range samples {
		c.reportPing(1, time.Duration(sample)*time.Millisecond)
		if c.health.PingMs == nil || *c.health.PingMs != expected[i] {
			t.Fatalf("sample %d: got %v, want %d", i, c.health.PingMs, expected[i])
		}
	}
	c.reportPing(0, time.Second)
	if *c.health.PingMs != 40 {
		t.Fatal("old session changed the median")
	}
	c.setFeedHealth(1, core.Health{State: "reconnecting"})
	if c.health.PingMs != nil || len(c.pingSamples) != 0 {
		t.Fatal("reconnect retained ping history")
	}
	c.setFeedHealth(1, core.Health{State: "connected"})
	c.reportPing(1, 200*time.Millisecond)
	if *c.health.PingMs != 200 {
		t.Fatal("new connection reused old samples")
	}
}

func TestSoundMuteAndBannersPersistIndependently(t *testing.T) {
	c, _ := testController(t, func(string, any) {})
	if !c.BannersEnabled() {
		t.Fatal("banners should default to enabled")
	}
	if err := c.SetSoundMuted(true); err != nil {
		t.Fatal(err)
	}
	if !c.BannersEnabled() {
		t.Fatal("muting sounds disabled banners")
	}
	settings := core.DefaultSettings()
	settings.BannersEnabled = false
	if err := c.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	saved, err := c.store.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if !saved.SoundMuted || saved.BannersEnabled {
		t.Fatalf("settings overwritten: %+v", saved)
	}
	c.audio.mu.RLock()
	muted := c.audio.muted
	c.audio.mu.RUnlock()
	if !muted {
		t.Fatal("audio mute was not applied")
	}
	if err := c.SetSoundMuted(false); err != nil {
		t.Fatal(err)
	}
	saved, err = c.store.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if saved.SoundMuted || saved.BannersEnabled {
		t.Fatal("unmuting changed banner preference")
	}
}

func TestMutedAlertsDoNotStartAudio(t *testing.T) {
	audio, err := NewAudio(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	audio.SetMuted(true)
	if err := audio.PlayAlert(1, "bell", 1); err != nil {
		t.Fatal(err)
	}
	if audio.context != nil || audio.player != nil {
		t.Fatal("muted alert started audio")
	}
	audio.SetMuted(false)
	if err := audio.PlayAlert(2, "none", 0); err != nil {
		t.Fatal(err)
	}
	if audio.latest != 2 {
		t.Fatal("unmuting did not restore alert playback")
	}
}

func TestFeedOnlyRuleStillPublishesAndPersistsAlerts(t *testing.T) {
	published := make(chan core.Alert, 1)
	c, _ := testController(t, func(name string, data any) {
		if name == "alert:new" {
			published <- data.(core.Alert)
		}
	})
	rule := lifecycleRule()
	rule.BannerSeconds = 0
	if _, err := c.SaveRule(rule); err != nil {
		t.Fatal(err)
	}
	c.engine.SetVolumes(map[string]float64{"BTCUSDT": 100e6})
	c.onQuote(marketQuote("BTCUSDT", 100, time.Now().Add(-time.Second)))
	c.onQuote(marketQuote("BTCUSDT", 94, time.Now()))
	c.actionWG.Wait()
	select {
	case alert := <-published:
		history, err := c.AlertsAfter(alert.ID-1, 100)
		if err != nil || len(history) != 1 || history[0].ID != alert.ID || history[0].BannerSeconds != 0 {
			t.Fatalf("feed-only alert was lost: %+v %v", history, err)
		}
	default:
		t.Fatal("feed-only alert was not published")
	}
}
