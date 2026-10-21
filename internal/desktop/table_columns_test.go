package desktop

import (
	"reflect"
	"testing"

	"cryptotape/internal/core"
)

func TestDedicatedColumnUpdatesPreserveOtherSettings(t *testing.T) {
	c, _ := testController(t, func(string, any) {})
	stale, err := c.GetSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	columns := []core.TableColumnID{core.ColumnEvent, core.ColumnTime}
	if err := c.SetSoundMuted(true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetTableColumns(columns); err != nil {
		t.Fatal(err)
	}
	columns[0] = core.ColumnTicker
	stale.Settings.Language = "en"
	if err := c.SaveSettings(stale.Settings); err != nil {
		t.Fatal(err)
	}
	saved, err := c.store.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if !saved.SoundMuted || saved.Language != "en" || !reflect.DeepEqual(saved.TableColumns, []core.TableColumnID{core.ColumnEvent, core.ColumnTime}) {
		t.Fatalf("unrelated settings overwritten: %+v", saved)
	}
	if err := c.SetTableColumns([]core.TableColumnID{core.ColumnTime, core.ColumnTime}); err == nil {
		t.Fatal("duplicate columns accepted")
	}
	if err := c.SetTableColumns([]core.TableColumnID{}); err != nil {
		t.Fatal(err)
	}
	saved, err = c.store.Settings()
	if err != nil || saved.TableColumns == nil || len(saved.TableColumns) != 0 {
		t.Fatalf("all-hidden columns lost: %+v %v", saved.TableColumns, err)
	}
}
