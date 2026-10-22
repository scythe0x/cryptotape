package licenseserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBotIssueIsIdempotentAndRejectsForeignCallback(t *testing.T) {
	s := testStore(t)
	var methods []string
	var messages []string
	b := &Bot{Token: "test", Store: s, Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		parts := strings.Split(r.URL.Path, "/")
		method := parts[len(parts)-1]
		methods = append(methods, method)
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if method == "editMessageText" {
			messages = append(messages, body.Text)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`)), Header: make(http.Header)}, nil
	})}}
	var update telegramUpdate
	if err := json.Unmarshal([]byte(`{"update_id":1,"callback_query":{"id":"c","from":{"id":42},"data":"get_key","message":{"message_id":1,"chat":{"id":42,"type":"private"}}}}`), &update); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := b.handle(context.Background(), update); err != nil {
			t.Fatal(err)
		}
	}
	if len(messages) != 2 || messages[0] != messages[1] || !strings.HasPrefix(messages[0], "<code>") {
		t.Fatal("key message not idempotent")
	}
	l, err := s.Issue(context.Background(), 42, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(messages[0], l.Key) {
		t.Fatal("wrong key returned")
	}
	update.Callback.From.ID = 99
	if err = b.handle(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || methods[len(methods)-1] != "answerCallbackQuery" {
		t.Fatal("foreign callback exposed key")
	}
}
func TestBackupCanBeReopenedWithEncryptedKeys(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	original, err := s.Issue(ctx, 42, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/backup.sqlite"
	if err = s.Backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(path, make([]byte, 32), []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	l, err := restored.Issue(ctx, 42, time.Now())
	if err != nil || l != original {
		t.Fatal("backup lost encrypted license", err)
	}
}
