package storage

import (
	"path/filepath"
	"testing"
)

func TestLanguageDefaultsForExistingSettingsAndPersists(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "settings.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.db.Exec(`UPDATE settings SET payload='{"proxy":"","cooldownSeconds":60,"monitorId":"","tigerPort":7819,"tigerLinkGroup":"A"}'`); err != nil {
		t.Fatal(err)
	}
	settings, err := store.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Language != "ru" {
		t.Fatalf("legacy settings language: %q", settings.Language)
	}
	settings.Language = "en"
	if err = store.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Language != "en" {
		t.Fatal("language was not saved")
	}
	settings.Language = "de"
	if err = store.SaveSettings(settings); err == nil {
		t.Fatal("unsupported language accepted")
	}
}
