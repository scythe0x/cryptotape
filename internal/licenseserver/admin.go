package licenseserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const AdminID int64 = 5766296513

type adminCommand struct {
	Name       string
	TelegramID int64
	Days       int
}

func parseAdminCommand(text string) (adminCommand, error) {
	fields := strings.Fields(text)
	var c adminCommand
	if len(fields) == 0 {
		return c, errors.New("Пустая команда")
	}
	name, mention, hasMention := strings.Cut(fields[0], "@")
	if hasMention && !strings.EqualFold(mention, "mythrascalping_bot") {
		return c, errors.New("Команда адресована другому боту")
	}
	c.Name = name
	count := 3
	switch name {
	case "/get", "/reset_hwid", "/public_add":
		count = 2
	case "/add", "/set":
	default:
		return c, errors.New("Неизвестная команда")
	}
	if len(fields) != count {
		return c, errors.New("Формат: " + commandUsage(name))
	}
	if name != "/public_add" {
		id, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || id <= 0 {
			return c, errors.New("Некорректный telegram id")
		}
		c.TelegramID = id
	}
	if name == "/add" || name == "/set" || name == "/public_add" {
		days, err := strconv.Atoi(fields[len(fields)-1])
		minimum := 1
		if name == "/set" {
			minimum = 0
		}
		if err != nil || days < minimum || days > 3650 {
			return c, fmt.Errorf("Количество дней: от %d до 3650", minimum)
		}
		c.Days = days
	}
	return c, nil
}
func commandUsage(name string) string {
	switch name {
	case "/get", "/reset_hwid":
		return name + " <id>"
	case "/public_add":
		return name + " <days>"
	default:
		return name + " <id> <days>"
	}
}

type licenseInfo struct {
	ID            int64     `json:"license_id"`
	TelegramID    int64     `json:"telegram_id"`
	Key           string    `json:"key"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	RemainingDays int64     `json:"remaining_days"`
	Revoked       bool      `json:"revoked"`
	Bound         bool      `json:"bound"`
	Hardware      []string  `json:"hardware_sha256"`
}

func (s *Store) licenseInfo(ctx context.Context, tx *sql.Tx, id int64, now time.Time) (licenseInfo, error) {
	info := licenseInfo{TelegramID: id, Hardware: []string{}}
	var created, expires int64
	var encrypted []byte
	err := tx.QueryRowContext(ctx, `SELECT id,key_cipher,created_at,expires_at,revoked,bound FROM licenses WHERE telegram_id=?`, id).Scan(&info.ID, &encrypted, &created, &expires, &info.Revoked, &info.Bound)
	if err != nil {
		return info, err
	}
	key, err := s.decrypt(encrypted, []byte(fmt.Sprint(id)))
	if err != nil {
		return info, err
	}
	info.Key = string(key)
	info.CreatedAt = time.Unix(created, 0).UTC()
	info.ExpiresAt = time.Unix(expires, 0).UTC()
	info.RemainingDays = max(0, (expires-now.Unix()+86399)/86400)
	rows, err := tx.QueryContext(ctx, `SELECT fingerprint FROM hardware WHERE license_id=? ORDER BY fingerprint`, info.ID)
	if err != nil {
		return info, err
	}
	defer rows.Close()
	for rows.Next() {
		var fingerprint string
		if err = rows.Scan(&fingerprint); err != nil {
			return info, err
		}
		info.Hardware = append(info.Hardware, fingerprint)
	}
	return info, rows.Err()
}

// the mutation, update receipt and outgoing messages commit together, so replay cannot add days twice.
func (s *Store) ApplyAdmin(ctx context.Context, updateID, actor int64, text string, now time.Time) error {
	if actor != AdminID {
		return errors.New("administrator required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM admin_updates WHERE update_id=?`, updateID).Scan(&exists)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	c, parseErr := parseAdminCommand(text)
	reply, kind := "", "text"
	if parseErr != nil {
		reply = parseErr.Error()
	} else {
		reply, kind, err = s.applyAdminCommand(ctx, tx, c, now)
		if errors.Is(err, sql.ErrNoRows) {
			reply = "Пользователь не найден"
			err = nil
		}
		if err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO admin_updates(update_id,actor,command,created_at) VALUES(?,?,?,?)`, updateID, actor, fmt.Sprintf("%s %d %d", c.Name, c.TelegramID, c.Days), now.Unix()); err != nil {
		return err
	}
	if err = s.queueNotice(ctx, tx, actor, reply, kind, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) applyAdminCommand(ctx context.Context, tx *sql.Tx, c adminCommand, now time.Time) (string, string, error) {
	if c.Name == "/get" {
		info, err := s.licenseInfo(ctx, tx, c.TelegramID, now)
		if err != nil {
			return "", "", err
		}
		data, err := json.MarshalIndent(info, "", "  ")
		return string(data), "json", err
	}
	if c.Name == "/public_add" {
		return s.addAll(ctx, tx, c.Days, now)
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM licenses WHERE telegram_id=?`, c.TelegramID).Scan(&id); err != nil {
		return "", "", err
	}
	var err error
	notice := ""
	switch c.Name {
	case "/add":
		_, err = tx.ExecContext(ctx, `UPDATE licenses SET expires_at=MAX(expires_at,?)+?,revoked=0 WHERE id=?`, now.Unix(), int64(c.Days)*86400, id)
		notice = fmt.Sprintf("Вам начислено %d дополнительных дней доступа в Mythra", c.Days)
	case "/set":
		_, err = tx.ExecContext(ctx, `UPDATE licenses SET expires_at=? WHERE id=?`, now.Unix()+int64(c.Days)*86400, id)
		notice = fmt.Sprintf("Время истечения подписки Mythra изменено на %d дней", c.Days)
		if c.Days == 0 {
			notice = "Ваша подписка Mythra была деактивирована"
		}
	case "/reset_hwid":
		_, err = tx.ExecContext(ctx, `DELETE FROM hardware WHERE license_id=?`, id)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE licenses SET bound=0 WHERE id=?`, id)
		}
	}
	if err != nil {
		return "", "", err
	}
	if notice != "" {
		if err = s.queueNotice(ctx, tx, c.TelegramID, notice, "text", now); err != nil {
			return "", "", err
		}
	}
	return fmt.Sprintf("Готово: %s, пользователь %d", c.Name, c.TelegramID), "text", nil
}

func (s *Store) addAll(ctx context.Context, tx *sql.Tx, days int, now time.Time) (string, string, error) {
	// add days to every key; legacy server-side revocations are independent of expiry.
	if _, err := tx.ExecContext(ctx, `UPDATE licenses SET expires_at=MAX(expires_at,?)+?`, now.Unix(), int64(days)*86400); err != nil {
		return "", "", err
	}
	rows, err := tx.QueryContext(ctx, `SELECT telegram_id FROM licenses ORDER BY telegram_id`)
	if err != nil {
		return "", "", err
	}
	var recipients []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return "", "", err
		}
		recipients = append(recipients, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return "", "", err
	}
	if err = rows.Close(); err != nil {
		return "", "", err
	}
	text := fmt.Sprintf("Вам начислено %d дополнительных дней доступа в Mythra", days)
	for _, id := range recipients {
		if err = s.queueNotice(ctx, tx, id, text, "text", now); err != nil {
			return "", "", err
		}
	}
	return fmt.Sprintf("Добавлено дней: %d. Ключей: %d. Уведомления в очереди.", days, len(recipients)), "text", nil
}
