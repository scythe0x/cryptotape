package storage

import (
	"encoding/json"
	"path/filepath"
	"reflect"

	"cryptotape/internal/core"
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
	if settings.SoundMuted || !settings.BannersEnabled {
		t.Fatal("legacy settings did not default to enabled sounds and banners")
	}
	if !reflect.DeepEqual(settings.TableColumns, core.DefaultTableColumns()) {
		t.Fatalf("legacy table columns: %v", settings.TableColumns)
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
	var payload string
	if err = store.db.QueryRow(`SELECT payload FROM settings WHERE id=1`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal([]byte(payload), &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["cooldownSeconds"]; exists {
		t.Fatal("obsolete cooldown setting was saved")
	}
	settings.Language = "de"
	if err = store.SaveSettings(settings); err == nil {
		t.Fatal("unsupported language accepted")
	}
}

func TestTableColumnsDefaultValidateAndPersist(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "columns.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	settings, err := store.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(settings.TableColumns, core.DefaultTableColumns()) {
		t.Fatalf("column defaults: %v", settings.TableColumns)
	}
	for _, columns := range [][]core.TableColumnID{{core.ColumnEvent, core.ColumnTime}, {}} {
		settings.TableColumns = columns
		if err := store.SaveSettings(settings); err != nil {
			t.Fatal(err)
		}
		saved, err := store.Settings()
		if err != nil || !reflect.DeepEqual(saved.TableColumns, columns) {
			t.Fatalf("column order: %v %v", saved.TableColumns, err)
		}
	}
	for _, columns := range [][]core.TableColumnID{{core.ColumnTime, core.ColumnTime}, {"unknown"}} {
		settings.TableColumns = columns
		if err := store.SaveSettings(settings); err == nil {
			t.Fatalf("invalid columns accepted: %v", columns)
		}
	}
}
