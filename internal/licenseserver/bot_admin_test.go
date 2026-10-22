package licenseserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func adminUpdate(t *testing.T, id, actor, chat int64, chatType, text string) telegramUpdate {
	t.Helper()
	var u telegramUpdate
	data, err := json.Marshal(map[string]interface{}{"update_id": id, "message": map[string]interface{}{"message_id": id, "from": map[string]int64{"id": actor}, "chat": map[string]interface{}{"id": chat, "type": chatType}, "text": text}})
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &u); err != nil {
		t.Fatal(err)
	}
	return u
}
func TestBotAdminIdentityAndPrivateChatRequired(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	l, err := s.Issue(context.Background(), 42, now)
	if err != nil {
		t.Fatal(err)
	}
	b := &Bot{Store: s, adminMenuReady: true}
	for _, u := range []telegramUpdate{
		adminUpdate(t, 1, 42, 42, "private", "/set 42 0"),
		adminUpdate(t, 2, AdminID, 42, "private", "/set 42 0"),
		adminUpdate(t, 3, AdminID, AdminID, "group", "/set 42 0"),
	} {
		if err = b.handle(context.Background(), u); err != nil {
			t.Fatal(err)
		}
	}
	expires, revoked, _ := licenseState(t, s, 42)
	if expires != l.ExpiresAt || revoked {
		t.Fatal("spoofed admin command was accepted")
	}
	if err = b.handle(context.Background(), adminUpdate(t, 4, AdminID, AdminID, "private", "/set@mythrascalping_bot 42 0")); err != nil {
		t.Fatal(err)
	}
	expires, revoked, _ = licenseState(t, s, 42)
	if revoked || expires > time.Now().Unix() {
		t.Fatal("admin command was ignored")
	}
}
func TestNotificationFailureDoesNotReplayDayAddition(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	l, err := s.Issue(context.Background(), 42, now)
	if err != nil {
		t.Fatal(err)
	}
	var sent []string
	failUser := true
	b := &Bot{Token: "test", Store: s, adminMenuReady: true, Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var message struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			t.Fatal(err)
		}
		if message.ChatID == 42 && failUser {
			return nil, errors.New("network unavailable")
		}
		sent = append(sent, message.Text)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`))}, nil
	})}}
	u := adminUpdate(t, 10, AdminID, AdminID, "private", "/add 42 20")
	if err = b.handle(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if _, err = b.deliverNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = b.deliverNext(context.Background()); err == nil {
		t.Fatal("delivery failure hidden")
	}
	if err = b.handle(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	expires, _, _ := licenseState(t, s, 42)
	if expires != l.ExpiresAt+20*86400 {
		t.Fatal("redelivery added days twice")
	}
	failUser = false
	if _, err = s.db.Exec(`UPDATE bot_outbox SET available_at=0 WHERE status='pending'`); err != nil {
		t.Fatal(err)
	}
	if _, err = b.deliverNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 2 || sent[1] != "Вам начислено 20 дополнительных дней доступа в Mythra" {
		t.Fatal(sent)
	}
}
func TestBlockedRecipientDoesNotBlockOutbox(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	if _, err := s.Issue(context.Background(), 42, now); err != nil {
		t.Fatal(err)
	}
	applyCommand(t, s, 10, "/add 42 20", now)
	b := &Bot{Token: "test", Store: s, adminMenuReady: true, Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`))}, nil
	})}}
	for range 2 {
		if _, err := b.deliverNext(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	var failed int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM bot_outbox WHERE status='failed_403'`).Scan(&failed); err != nil || failed != 2 {
		t.Fatal(failed, err)
	}
}
func TestAdminMenuIsChatScoped(t *testing.T) {
	var requests []struct {
		Commands []botCommand `json:"commands"`
		Scope    struct {
			Type   string `json:"type"`
			ChatID int64  `json:"chat_id"`
		} `json:"scope"`
	}
	b := &Bot{Token: "test", Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "setMyCommands") {
			var body struct {
				Commands []botCommand `json:"commands"`
				Scope    struct {
					Type   string `json:"type"`
					ChatID int64  `json:"chat_id"`
				} `json:"scope"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			requests = append(requests, body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`))}, nil
	})}}
	if err := b.configure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || len(requests[0].Commands) != 1 || requests[1].Scope.Type != "chat" || requests[1].Scope.ChatID != AdminID || len(requests[1].Commands) != 6 {
		t.Fatal(requests)
	}
}

func TestAdminMenuWaitsForFirstPrivateChatWithoutStoppingSetup(t *testing.T) {
	adminRequests := 0
	b := &Bot{Token: "test", Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Scope struct {
				Type string `json:"type"`
			} `json:"scope"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Scope.Type == "chat" {
			adminRequests++
			if adminRequests == 1 {
				return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`))}, nil
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`))}, nil
	})}}
	if err := b.configure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b.adminMenuReady {
		t.Fatal("unknown chat menu marked ready")
	}
	if err := b.configureAdmin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !b.adminMenuReady || adminRequests != 2 {
		t.Fatal("admin menu did not recover")
	}
}
func TestTelegramRetryAfterIsPreserved(t *testing.T) {
	response := &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error_code":429,"parameters":{"retry_after":19}}`))}
	defer response.Body.Close()
	err := decodeTelegram(response, nil)
	var apiErr *telegramError
	if !errors.As(err, &apiErr) || apiErr.Code != 429 || apiErr.RetryAfter != 19 {
		t.Fatal("retry_after was lost", err)
	}
}
