//go:build windows

package licensing

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDPAPIKeyStorageRoundTripAndReplacement(t *testing.T) {
	store := FileKeys{Path: filepath.Join(t.TempDir(), "license.dat")}
	if key, err := store.Load(); err != nil || key != "" {
		t.Fatal("missing key storage", err)
	}
	for _, key := range []string{"0123456789ABCDEF0123456789ABCDEF", "FEDCBA9876543210FEDCBA9876543210"} {
		if err := store.Save(key); err != nil {
			t.Fatal(err)
		}
		encrypted, err := os.ReadFile(store.Path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(encrypted, []byte(key)) {
			t.Fatal("key stored in plaintext")
		}
		actual, err := store.Load()
		if err != nil || actual != key {
			t.Fatal("protected key round trip failed", err)
		}
	}
	if err := os.WriteFile(store.Path, []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("corrupted key storage was accepted")
	}
}
