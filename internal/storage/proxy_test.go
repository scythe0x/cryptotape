package storage

import (
	"path/filepath"
	"testing"
)

func TestLegacyProxyBecomesNamedProfile(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "legacy.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	_, err = store.db.Exec(`UPDATE settings SET payload='{"proxy":"127.0.0.1:8080","language":"ru","cooldownSeconds":60,"tigerPort":7819,"tigerLinkGroup":"A"}'`)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := store.Settings()
	if err != nil || len(settings.Proxies) != 1 || settings.ActiveProxyID != settings.Proxies[0].ID {
		t.Fatalf("legacy profile not restored: %+v %v", settings, err)
	}
	if settings.Proxies[0].Host != "127.0.0.1" || settings.Proxies[0].Port != 8080 || settings.Proxies[0].Name != "proxy-1" {
		t.Fatal("legacy proxy changed")
	}
	if err := store.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Settings()
	if err != nil || len(restored.Proxies) != 1 || restored.ActiveProxyID != settings.ActiveProxyID {
		t.Fatal("migrated profile lost")
	}
}
