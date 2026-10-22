package licenseserver

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"cryptotape/internal/licensing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "licenses.sqlite"), make([]byte, 32), []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func hw(value string) []string {
	h := sha256.Sum256([]byte(value))
	return []string{hex.EncodeToString(h[:])}
}
func req(key, pc string) licensing.Request {
	return licensing.Request{Key: key, Hardware: hw(pc), Nonce: licensing.NewNonce()}
}
func checkCode(t *testing.T, s *Store, r licensing.Request, activate bool, at time.Time, want string) {
	t.Helper()
	g, err := s.Check(context.Background(), r, activate, at)
	if err != nil || g.Code != want {
		t.Fatalf("check: %s %v, want %s", g.Code, err, want)
	}
}
func TestLifetimeBindingAndRenewal(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Unix(1800000000, 0)
	a, err := s.Issue(ctx, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	same, err := s.Issue(ctx, 1, now.Add(time.Hour))
	if err != nil || same != a {
		t.Fatal("issue must be idempotent", err)
	}
	if a.ExpiresAt != now.Add(30*24*time.Hour).Unix() {
		t.Fatal("expiry must start on issue")
	}
	b, err := s.Issue(ctx, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	checkCode(t, s, req(a.Key, "a"), false, now, "INVALID_KEY")
	checkCode(t, s, req(a.Key, "a"), true, now, "OK")
	checkCode(t, s, req(a.Key, "a"), false, now, "OK")
	checkCode(t, s, req(a.Key, "b"), true, now, "OTHER_PC")
	checkCode(t, s, req(b.Key, "a"), true, now, "PC_USED")
	checkCode(t, s, req(a.Key, "a"), false, time.Unix(a.ExpiresAt, 0), "EXPIRED")
	if err = s.Extend(ctx, 1, 30, now); err != nil {
		t.Fatal(err)
	}
	renewed, err := s.Issue(ctx, 1, now)
	if err != nil || renewed.Key != a.Key || renewed.ExpiresAt != a.ExpiresAt+30*86400 {
		t.Fatal("renewal lost key or remaining days", err)
	}
	if err = s.Revoke(ctx, 1); err != nil {
		t.Fatal(err)
	}
	checkCode(t, s, req(a.Key, "a"), false, now, "REVOKED")
	checkCode(t, s, req(b.Key, "a"), true, now, "PC_USED")
	later := now.Add(100 * 24 * time.Hour)
	if err = s.Extend(ctx, 1, 30, later); err != nil {
		t.Fatal(err)
	}
	renewed, err = s.Issue(ctx, 1, later)
	if err != nil || renewed.ExpiresAt != later.Add(30*24*time.Hour).Unix() {
		t.Fatal("expired renewal", err)
	}
}
func TestConcurrentIssueAndActivation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now()
	const count = 12
	var wg sync.WaitGroup
	keys := make(chan string, count)
	for range count {
		wg.Go(func() {
			l, err := s.Issue(ctx, 42, now)
			if err != nil {
				t.Error(err)
				return
			}
			keys <- l.Key
		})
	}
	wg.Wait()
	close(keys)
	var key string
	for k := range keys {
		if key != "" && key != k {
			t.Fatal("multiple keys issued")
		}
		key = k
	}
	codes := make(chan string, 2)
	for _, pc := range []string{"one", "two"} {
		wg.Go(func() {
			g, err := s.Check(ctx, req(key, pc), true, now)
			if err != nil {
				t.Error(err)
				return
			}
			codes <- g.Code
		})
	}
	wg.Wait()
	close(codes)
	successes := 0
	for code := range codes {
		if code == "OK" {
			successes++
		} else if code != "OTHER_PC" {
			t.Fatal(code)
		}
	}
	if successes != 1 {
		t.Fatal("activation was not atomic")
	}
}
func TestBoardAliasSurvivesUUIDChange(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	l, err := s.Issue(context.Background(), 1, now)
	if err != nil {
		t.Fatal(err)
	}
	r := req(l.Key, "uuid")
	r.Hardware = append(r.Hardware, hw("board")[0])
	checkCode(t, s, r, true, now, "OK")
	r.Hardware = []string{hw("changed-uuid")[0], hw("board")[0]}
	checkCode(t, s, r, true, now, "OK")
	other, err := s.Issue(context.Background(), 2, now)
	if err != nil {
		t.Fatal(err)
	}
	r.Key = other.Key
	checkCode(t, s, r, true, now, "PC_USED")
}
func TestSignedAPI(t *testing.T) {
	s := testStore(t)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(Handler(s, priv))
	defer server.Close()
	l, err := s.Issue(context.Background(), 7, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	client := licensing.Client{Endpoint: server.URL, PublicKey: pub, HTTP: server.Client(), Wait: func(context.Context, time.Duration) error { return nil }}
	g, err := client.Check(context.Background(), l.Key, hw("pc"), true)
	if err != nil || g.Code != "OK" {
		t.Fatal(g, err)
	}
	_, err = client.Check(context.Background(), l.Key, hw("another"), true)
	if err == nil {
		t.Fatal("different pc accepted")
	}
}
