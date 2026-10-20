package desktop

import "testing"

func TestSilentAlertNeedsNoAudioDeviceAndAdvancesSequence(t *testing.T) {
	audio := &Audio{}
	if !audio.Has("none") {
		t.Fatal("silent preset is missing")
	}
	if err := audio.PlayAlert(12, "none", 1); err != nil {
		t.Fatal(err)
	}
	if audio.latest != 12 || audio.player != nil || audio.context != nil {
		t.Fatal("silent alert initialized audio or failed to advance sequence")
	}
	if err := audio.PlayAlert(11, "none", 1); err != nil {
		t.Fatal(err)
	}
	if audio.latest != 12 {
		t.Fatal("older alert replaced the latest one")
	}
	if err := audio.Play("missing", 1); err == nil {
		t.Fatal("unknown sound accepted")
	}
}
