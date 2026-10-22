package licenseserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Bot struct {
	Token          string
	Store          *Store
	Client         *http.Client
	adminMenuReady bool
}
type telegramUser struct {
	ID int64 `json:"id"`
}
type telegramMessage struct {
	ID   int64        `json:"message_id"`
	From telegramUser `json:"from"`
	Chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
	Text string `json:"text"`
}
type telegramUpdate struct {
	ID       int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
	Callback *struct {
		ID      string           `json:"id"`
		From    telegramUser     `json:"from"`
		Data    string           `json:"data"`
		Message *telegramMessage `json:"message"`
	} `json:"callback_query"`
}

func (b *Bot) call(ctx context.Context, method string, body interface{}, result interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.telegram.org/bot"+b.Token+"/"+method, bytes.NewReader(data))
	if err != nil {
		return errors.New("telegram request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := b.Client.Do(req)
	if err != nil {
		return errors.New("telegram transport failed")
	}
	defer response.Body.Close()
	return decodeTelegram(response, result)
}

func (b *Bot) Run(ctx context.Context) error {
	if b.Token == "" {
		return errors.New("MYTHRA_BOT_TOKEN is required")
	}
	if b.Client == nil {
		b.Client = &http.Client{Timeout: 40 * time.Second}
	}
	outboxDone := make(chan struct{})
	go func() { defer close(outboxDone); b.runOutbox(ctx) }()
	defer func() { <-outboxDone }()
	configured := false
	backoff := time.Second
	for ctx.Err() == nil {
		err := error(nil)
		if !configured {
			err = b.configure(ctx)
			configured = err == nil
		}
		if err == nil {
			err = b.poll(ctx)
		}
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			slog.Warn("telegram polling retry")
		} else {
			backoff = time.Second
			continue
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		backoff = min(backoff*2, 30*time.Second)
	}
	return nil
}

func (b *Bot) poll(ctx context.Context) error {
	offset, err := b.Store.Offset(ctx)
	if err != nil {
		return err
	}
	var updates []telegramUpdate
	if err = b.call(ctx, "getUpdates", map[string]interface{}{"offset": offset, "timeout": 25, "allowed_updates": []string{"message", "callback_query"}}, &updates); err != nil {
		return err
	}
	for _, u := range updates {
		if err = b.handle(ctx, u); err != nil {
			return err
		}
		if err = b.Store.SetOffset(ctx, u.ID+1); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) handle(ctx context.Context, u telegramUpdate) error {
	if m := u.Message; m != nil && m.From.ID == AdminID && m.Chat.ID == AdminID && m.Chat.Type == "private" {
		if !b.adminMenuReady {
			if err := b.configureAdmin(ctx); err != nil {
				return err
			}
		}
		fields := strings.Fields(m.Text)
		if len(fields) > 0 {
			command, _, _ := strings.Cut(fields[0], "@")
			switch command {
			case "/add", "/set", "/get", "/reset_hwid", "/public_add":
				return b.Store.ApplyAdmin(ctx, u.ID, m.From.ID, m.Text, time.Now())
			}
		}
	}
	if m := u.Message; m != nil && m.Chat.Type == "private" && (m.Text == "/start" || m.Text == "/start@mythrascalping_bot") && m.Chat.ID == m.From.ID {
		return b.call(ctx, "sendMessage", map[string]interface{}{"chat_id": m.Chat.ID, "parse_mode": "HTML", "text": "<b>Mythra</b> зарабатывает секунды, чтобы ты заработал деньги\n\nСейчас доступен бесплатный доступ для всех пользователей", "reply_markup": map[string]interface{}{"inline_keyboard": [][]map[string]string{{{"text": "Получить ключ", "callback_data": "get_key"}}}}}, nil)
	}
	c := u.Callback
	if c == nil {
		return nil
	}
	if c.Data != "get_key" || c.Message == nil || c.Message.Chat.Type != "private" || c.Message.Chat.ID != c.From.ID {
		return b.call(ctx, "answerCallbackQuery", map[string]string{"callback_query_id": c.ID}, nil)
	}
	l, err := b.Store.Issue(ctx, c.From.ID, time.Now())
	if err != nil {
		return err
	}
	text := fmt.Sprintf("<code>%s</code>\n\nДоступ до %s", l.Key, time.Unix(l.ExpiresAt, 0).UTC().Format("02.01.2006 15:04 UTC"))
	// editing the same message is idempotent; telegram returns 400 when it already has this content.
	err = b.call(ctx, "editMessageText", map[string]interface{}{"chat_id": c.Message.Chat.ID, "message_id": c.Message.ID, "parse_mode": "HTML", "text": text, "reply_markup": map[string]interface{}{"inline_keyboard": [][]map[string]string{}}}, nil)
	if err != nil {
		return err
	}
	return b.call(ctx, "answerCallbackQuery", map[string]string{"callback_query_id": c.ID}, nil)
}
