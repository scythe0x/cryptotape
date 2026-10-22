package licenseserver

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type botCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

var adminMenu = []botCommand{
	{"start", "Главное меню"}, {"add", "Добавить дни: id days"}, {"set", "Установить срок: id days"},
	{"get", "Данные пользователя: id"}, {"reset_hwid", "Сбросить привязку ПК: id"}, {"public_add", "Добавить дни всем: days"},
}

func (b *Bot) configure(ctx context.Context) error {
	if err := b.call(ctx, "setMyCommands", map[string]interface{}{"commands": []botCommand{{"start", "Главное меню"}}}, nil); err != nil {
		return err
	}
	if err := b.configureAdmin(ctx); err != nil {
		var apiErr *telegramError
		if !errors.As(err, &apiErr) || apiErr.Code != 400 {
			return err
		}
		// telegram can reject a chat scope before the admin has opened this bot.
		slog.Info("admin menu awaits private chat")
	}
	return b.call(ctx, "setChatMenuButton", map[string]interface{}{"menu_button": map[string]string{"type": "commands"}}, nil)
}

func (b *Bot) configureAdmin(ctx context.Context) error {
	if err := b.call(ctx, "setMyCommands", map[string]interface{}{"scope": map[string]interface{}{"type": "chat", "chat_id": AdminID}, "commands": adminMenu}, nil); err != nil {
		return err
	}
	b.adminMenuReady = true
	return nil
}

func (b *Bot) runOutbox(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		delivered, err := b.deliverNext(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			slog.Warn("telegram outbox retry")
		}
		delay := time.Second
		if delivered {
			delay = 100 * time.Millisecond
		}
		var apiErr *telegramError
		if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
			delay = max(delay, time.Duration(apiErr.RetryAfter)*time.Second)
		}
		timer.Reset(delay)
	}
}

func (b *Bot) deliverNext(ctx context.Context) (bool, error) {
	n, err := b.Store.nextNotice(ctx, time.Now())
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	text := n.Text
	if n.Kind == "json" && utf8.RuneCountInString(text) > 3500 {
		err = b.sendJSON(ctx, n)
	} else {
		request := map[string]interface{}{"chat_id": n.Recipient, "text": text}
		if n.Kind == "json" {
			request["text"] = "<pre>" + html.EscapeString(text) + "</pre>"
			request["parse_mode"] = "HTML"
		}
		err = b.call(ctx, "sendMessage", request, nil)
	}
	if err == nil {
		return true, b.Store.finishNotice(ctx, n.ID, "sent")
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	var apiErr *telegramError
	if errors.As(err, &apiErr) && (apiErr.Code == 400 || apiErr.Code == 403) {
		slog.Warn("telegram notification rejected", "recipient", n.Recipient, "status", apiErr.Code)
		return true, b.Store.finishNotice(ctx, n.ID, fmt.Sprintf("failed_%d", apiErr.Code))
	}
	delay := min(time.Second*time.Duration(1<<min(n.Attempts+1, 8)), 5*time.Minute)
	if apiErr != nil && apiErr.RetryAfter > 0 {
		delay = max(delay, time.Duration(apiErr.RetryAfter)*time.Second)
	}
	if retryErr := b.Store.retryNotice(ctx, n, delay); retryErr != nil {
		return false, retryErr
	}
	return false, err
}

func (b *Bot) sendJSON(ctx context.Context, n notice) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", strconv.FormatInt(n.Recipient, 10)); err != nil {
		return err
	}
	file, err := writer.CreateFormFile("document", "mythra-user.json")
	if err != nil {
		return err
	}
	if _, err = io.Copy(file, strings.NewReader(n.Text)); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.telegram.org/bot"+b.Token+"/sendDocument", &body)
	if err != nil {
		return errors.New("telegram request failed")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := b.Client.Do(req)
	if err != nil {
		return errors.New("telegram transport failed")
	}
	defer response.Body.Close()
	return decodeTelegram(response, nil)
}

type telegramError struct {
	Code       int
	RetryAfter int
}

func (e *telegramError) Error() string { return fmt.Sprintf("telegram api status %d", e.Code) }
func decodeTelegram(response *http.Response, result interface{}) error {
	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
		ErrorCode   int             `json:"error_code"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&envelope); err != nil {
		return errors.New("telegram response failed")
	}
	if !envelope.OK {
		if strings.Contains(envelope.Description, "message is not modified") {
			return nil
		}
		code := envelope.ErrorCode
		if code == 0 {
			code = response.StatusCode
		}
		return &telegramError{Code: code, RetryAfter: envelope.Parameters.RetryAfter}
	}
	if result != nil {
		return json.Unmarshal(envelope.Result, result)
	}
	return nil
}
