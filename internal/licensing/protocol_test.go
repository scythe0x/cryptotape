package licensing

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func testRequest() Request {
	return Request{Key: "0123456789ABCDEF0123456789ABCDEF", Hardware: []string{fingerprint("uuid", "pc")}, Nonce: NewNonce()}
}
func TestSignaturesBindNonceHardwareAndExpiry(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r := testRequest()
	g := Grant{Version: 1, Code: "OK", Nonce: r.Nonce, Hardware: HardwareDigest(r.Hardware), LicenseID: 1, ServerTime: 100, ExpiresAt: 200}
	envelope, err := Sign(g, priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(envelope, pub, r); err != nil {
		t.Fatal(err)
	}
	replay := r
	replay.Nonce = NewNonce()
	if _, err = Verify(envelope, pub, replay); err == nil {
		t.Fatal("replay accepted")
	}
	replay = r
	replay.Hardware = []string{fingerprint("uuid", "other")}
	if _, err = Verify(envelope, pub, replay); err == nil {
		t.Fatal("hardware mismatch accepted")
	}
	_, other, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	forged, err := Sign(g, other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(forged, pub, r); err == nil {
		t.Fatal("forged server accepted")
	}
	g.ExpiresAt = g.ServerTime
	expired, err := Sign(g, priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(expired, pub, r); err == nil {
		t.Fatal("expired grant accepted")
	}
}
func TestRetriesAreBoundedAndDenialsAreImmediate(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	attempts := 0
	denial := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if !denial {
			http.Error(w, "down", 503)
			return
		}
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		envelope, err := Sign(Grant{Version: 1, Code: "REVOKED", Nonce: request.Nonce, Hardware: HardwareDigest(request.Hardware), ServerTime: 100}, priv)
		if err != nil {
			t.Error(err)
			return
		}
		if err = json.NewEncoder(w).Encode(envelope); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	var delays []time.Duration
	c := Client{Endpoint: server.URL, PublicKey: pub, HTTP: server.Client(), Wait: func(_ context.Context, d time.Duration) error { delays = append(delays, d); return nil }}
	r := testRequest()
	if _, err := c.Check(context.Background(), r.Key, r.Hardware, true); err == nil {
		t.Fatal("outage accepted")
	}
	if attempts != 5 || !reflect.DeepEqual(delays, []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}) {
		t.Fatal(attempts, delays)
	}
	attempts = 0
	denial = true
	if _, err := c.Check(context.Background(), r.Key, r.Hardware, true); err == nil || attempts != 1 {
		t.Fatal("denial was retried")
	}
}
func TestSMBIOSIdentityAndInvalidValues(t *testing.T) {
	data := make([]byte, 24)
	data[0] = 1
	data[1] = 24
	for i := 8; i < 24; i++ {
		data[i] = byte(i)
	}
	data = append(data, 0, 0)
	ids, err := hardwareFromSMBIOS(data)
	if err != nil || len(ids) != 1 {
		t.Fatal(ids, err)
	}
	for i := 8; i < 24; i++ {
		data[i] = 0
	}
	if _, err = hardwareFromSMBIOS(data); err == nil {
		t.Fatal("empty uuid accepted")
	}
	if _, err = hardwareFromSMBIOS([]byte{1, 24, 0, 0}); err == nil {
		t.Fatal("truncated firmware accepted")
	}
	for _, s := range []string{"", "To be filled by O.E.M.", "00000000", "Default string", "System Serial Number", "123456789", "0123456789", "ABCDEFG"} {
		if usableSerial(s) {
			t.Fatal("placeholder accepted", s)
		}
	}
	if !usableSerial("AB12-XYZ8") {
		t.Fatal("valid serial rejected")
	}
}
