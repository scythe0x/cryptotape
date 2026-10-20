package desktop

import (
	"context"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func TestProxyProfilesPersistAndOnlyOneIsSelected(t *testing.T) {
	controller, path := testController(t, func(string, any) {})
	first := core.ProxyProfile{Name: "proxy-1", Host: "proxy.example", Port: 443, Protocol: "http", Login: "test-user", Password: "test-password"}
	if err := controller.SaveProxy(first, false); err != nil {
		t.Fatal(err)
	}
	snapshot, err := controller.GetSnapshot()
	if err != nil || len(snapshot.Settings.Proxies) != 1 {
		t.Fatalf("save profile: %v", err)
	}
	first = snapshot.Settings.Proxies[0]
	if snapshot.Settings.ActiveProxyID != "" {
		t.Fatal("new profile activated without an explicit checkbox")
	}
	second := core.ProxyProfile{Name: "proxy-2", Host: "127.0.0.1", Port: 1080, Protocol: "socks5"}
	if err := controller.SaveProxy(second, true); err != nil {
		t.Fatal(err)
	}
	snapshot, err = controller.GetSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	second = snapshot.Settings.Proxies[1]
	if snapshot.Settings.ActiveProxyID != second.ID {
		t.Fatal("second proxy not activated")
	}
	first.Name = "renamed"
	if err := controller.SaveProxy(first, false); err != nil {
		t.Fatal(err)
	}
	if controller.proxySettings().ActiveProxyID != second.ID {
		t.Fatal("editing an inactive profile disabled the active one")
	}
	if err := controller.SaveProxy(first, true); err != nil {
		t.Fatal(err)
	}
	if controller.proxySettings().ActiveProxyID != first.ID {
		t.Fatal("activation did not replace the previous selection")
	}
	stale := core.DefaultSettings()
	if err := controller.SaveSettings(stale); err != nil {
		t.Fatal(err)
	}
	if controller.proxySettings().ActiveProxyID != first.ID {
		t.Fatal("general settings overwrote proxy selection")
	}
	reopened, err := NewController(path, func(string, any) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	})
	saved := reopened.proxySettings()
	if len(saved.Proxies) != 2 || saved.Proxies[0] != first || saved.ActiveProxyID != first.ID {
		t.Fatal("proxy profiles did not survive reopen")
	}
	if err := controller.SelectProxy(""); err != nil {
		t.Fatal(err)
	}
	if route, err := controller.proxySettings().ProxyURL(); err != nil || route != "" {
		t.Fatal("direct selection failed")
	}
	if err := controller.SelectProxy(second.ID); err != nil {
		t.Fatal(err)
	}
	if err := controller.DeleteProxy(second.ID); err != nil {
		t.Fatal(err)
	}
	if controller.proxySettings().ActiveProxyID != "" || len(controller.proxySettings().Proxies) != 1 {
		t.Fatal("deleting selected proxy did not restore direct selection")
	}
	if err := controller.SelectProxy("missing"); err == nil {
		t.Fatal("unknown proxy selected")
	}
}

func TestManualReconnectStartsNewSessionAndClearsPriceHistory(t *testing.T) {
	controller, _ := testController(t, func(string, any) {})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	controller.root = ctx
	before := controller.version.Load()
	controller.Reconnect()
	controller.feedWG.Wait()
	if controller.version.Load() != before+1 || controller.health.State != "connecting" {
		t.Fatal("manual reconnect did not start a fresh session")
	}
	controller.Reconnect()
	controller.feedWG.Wait()
	if controller.version.Load() != before+2 {
		t.Fatal("manual reconnect unavailable on another attempt")
	}
}

func TestOldConnectionEventsCannotReviveStoppedSession(t *testing.T) {
	controller, _ := testController(t, func(string, any) {})
	controller.version.Store(2)
	controller.setFeedHealth(2, core.Health{State: "error", ViaProxy: true, Detail: "proxy connection failed"})
	controller.setFeedHealth(1, core.Health{State: "warming"})
	controller.reportPing(1, time.Millisecond)
	if controller.health.State != "error" || controller.health.PingMs != nil || !controller.health.ViaProxy {
		t.Fatal("old session overwrote failed proxy state")
	}
}

func TestProxyProtocolIsInferredFromCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, login, password, supplied, want string
	}{
		{"local shadowsocks", "", "", "http", "socks5"},
		{"authenticated upstream", "test-user", "test-password", "socks5", "http"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller, _ := testController(t, func(string, any) {})
			profile := core.ProxyProfile{Name: "proxy-1", Host: "127.0.0.1", Port: 1080, Protocol: tc.supplied, Login: tc.login, Password: tc.password}
			if err := controller.SaveProxy(profile, false); err != nil {
				t.Fatal(err)
			}
			if got := controller.proxySettings().Proxies[0].Protocol; got != tc.want {
				t.Fatalf("protocol = %q, want %q", got, tc.want)
			}
		})
	}
}
