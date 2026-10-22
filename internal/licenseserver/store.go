package licenseserver

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cryptotape/internal/licensing"
	_ "modernc.org/sqlite"
)

type Store struct {
	db         *sql.DB
	encryption cipher.AEAD
	lookup     []byte
}
type License struct {
	ID        int64
	Key       string
	ExpiresAt int64
}

func Open(path string, encryptionKey, lookupKey []byte) (*Store, error) {
	if len(encryptionKey) != 32 || len(lookupKey) != 32 {
		return nil, errors.New("server keys must contain 32 bytes")
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		`PRAGMA journal_mode=WAL`, `PRAGMA busy_timeout=5000`, `PRAGMA foreign_keys=ON`,
		`CREATE TABLE IF NOT EXISTS licenses (id INTEGER PRIMARY KEY, telegram_id INTEGER NOT NULL UNIQUE, key_hash BLOB NOT NULL UNIQUE, key_cipher BLOB NOT NULL, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, revoked INTEGER NOT NULL DEFAULT 0, bound INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS hardware (fingerprint TEXT PRIMARY KEY, license_id INTEGER NOT NULL REFERENCES licenses(id))`,
		`CREATE TABLE IF NOT EXISTS metadata (name TEXT PRIMARY KEY, value INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS admin_updates (update_id INTEGER PRIMARY KEY, actor INTEGER NOT NULL, command TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS bot_outbox (id INTEGER PRIMARY KEY, recipient INTEGER NOT NULL, payload BLOB NOT NULL, kind TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0, available_at INTEGER NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS bot_outbox_pending ON bot_outbox(status,available_at,id)`,
		`PRAGMA user_version=2`,
	} {
		if _, err = db.Exec(statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{db: db, encryption: aead, lookup: append([]byte(nil), lookupKey...)}, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) hash(key string) []byte {
	m := hmac.New(sha256.New, s.lookup)
	m.Write([]byte(licensing.NormalizeKey(key)))
	return m.Sum(nil)
}

func (s *Store) Issue(ctx context.Context, telegramID int64, now time.Time) (License, error) {
	if telegramID <= 0 {
		return License{}, errors.New("invalid telegram id")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return License{}, err
	}
	defer tx.Rollback()
	var l License
	var encrypted []byte
	err = tx.QueryRowContext(ctx, `SELECT id,key_cipher,expires_at FROM licenses WHERE telegram_id=?`, telegramID).Scan(&l.ID, &encrypted, &l.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		raw := make([]byte, 16)
		if _, err = rand.Read(raw); err != nil {
			return l, err
		}
		key := strings.ToUpper(hex.EncodeToString(raw))
		l.Key = key[:8] + "-" + key[8:16] + "-" + key[16:24] + "-" + key[24:]
		encrypted, err = s.encrypt([]byte(l.Key), []byte(fmt.Sprint(telegramID)))
		if err != nil {
			return l, err
		}
		l.ExpiresAt = now.Add(30 * 24 * time.Hour).Unix()
		result, err := tx.ExecContext(ctx, `INSERT INTO licenses(telegram_id,key_hash,key_cipher,created_at,expires_at) VALUES(?,?,?,?,?)`, telegramID, s.hash(l.Key), encrypted, now.Unix(), l.ExpiresAt)
		if err != nil {
			return l, err
		}
		l.ID, err = result.LastInsertId()
		if err != nil {
			return l, err
		}
	} else if err != nil {
		return l, err
	} else {
		plain, err := s.decrypt(encrypted, []byte(fmt.Sprint(telegramID)))
		if err != nil {
			return l, errors.New("cannot decrypt license")
		}
		l.Key = string(plain)
	}
	return l, tx.Commit()
}

func (s *Store) Check(ctx context.Context, request licensing.Request, activate bool, now time.Time) (licensing.Grant, error) {
	g := licensing.Grant{Version: 1, Code: "INVALID_KEY", Nonce: request.Nonce, Hardware: licensing.HardwareDigest(request.Hardware), ServerTime: now.Unix()}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return g, err
	}
	defer tx.Rollback()
	var revoked, bound bool
	err = tx.QueryRowContext(ctx, `SELECT id,expires_at,revoked,bound FROM licenses WHERE key_hash=?`, s.hash(request.Key)).Scan(&g.LicenseID, &g.ExpiresAt, &revoked, &bound)
	if errors.Is(err, sql.ErrNoRows) {
		return g, nil
	}
	if err != nil {
		return g, err
	}
	if revoked {
		g.Code = "REVOKED"
		return g, nil
	}
	if g.ExpiresAt <= now.Unix() {
		g.Code = "EXPIRED"
		return g, nil
	}
	matched := false
	for _, id := range request.Hardware {
		var owner int64
		err = tx.QueryRowContext(ctx, `SELECT license_id FROM hardware WHERE fingerprint=?`, id).Scan(&owner)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return g, err
		}
		if err == nil {
			if owner != g.LicenseID {
				g.Code = "PC_USED"
				return g, nil
			}
			matched = true
		}
	}
	if bound && !matched {
		g.Code = "OTHER_PC"
		return g, nil
	}
	if !bound && !activate {
		g.Code = "INVALID_KEY"
		return g, nil
	}
	for _, id := range request.Hardware {
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO hardware(fingerprint,license_id) VALUES(?,?)`, id, g.LicenseID); err != nil {
			return g, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE licenses SET bound=1 WHERE id=?`, g.LicenseID); err != nil {
		return g, err
	}
	if err = tx.Commit(); err != nil {
		return g, err
	}
	g.Code = "OK"
	return g, nil
}

func (s *Store) Extend(ctx context.Context, telegramID int64, days int, now time.Time) error {
	if days < 1 || days > 3650 {
		return errors.New("days must be between 1 and 3650")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err = s.applyAdminCommand(ctx, tx, adminCommand{Name: "/add", TelegramID: telegramID, Days: days}, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Revoke(ctx context.Context, telegramID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `UPDATE licenses SET revoked=1 WHERE telegram_id=?`, telegramID)
	if err != nil {
		return err
	}
	count, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("license not found")
	}
	if err = s.queueNotice(ctx, tx, telegramID, "Ваша подписка Mythra была деактивирована", "text", time.Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Offset(ctx context.Context) (int64, error) {
	var offset int64
	err := s.db.QueryRowContext(ctx, `SELECT value FROM metadata WHERE name='telegram_offset'`).Scan(&offset)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return offset, err
}
func (s *Store) SetOffset(ctx context.Context, offset int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO metadata(name,value) VALUES('telegram_offset',?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, offset)
	return err
}

func (s *Store) Backup(ctx context.Context, path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("backup path must be absolute")
	}
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path)
	return err
}
