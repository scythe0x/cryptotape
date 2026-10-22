package licenseserver

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type notice struct {
	ID, Recipient int64
	Text, Kind    string
	Attempts      int
}

func (s *Store) encrypt(plain, associated []byte) ([]byte, error) {
	nonce := make([]byte, s.encryption.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.encryption.Seal(nonce, nonce, plain, associated), nil
}
func (s *Store) decrypt(data, associated []byte) ([]byte, error) {
	n := s.encryption.NonceSize()
	if len(data) < n {
		return nil, errors.New("invalid encrypted data")
	}
	return s.encryption.Open(nil, data[:n], data[n:], associated)
}
func (s *Store) queueNotice(ctx context.Context, tx *sql.Tx, recipient int64, text, kind string, now time.Time) error {
	encrypted, err := s.encrypt([]byte(text), []byte(fmt.Sprintf("notice:%d", recipient)))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO bot_outbox(recipient,payload,kind,available_at) VALUES(?,?,?,?)`, recipient, encrypted, kind, now.Unix())
	return err
}
func (s *Store) nextNotice(ctx context.Context, now time.Time) (notice, error) {
	var n notice
	var encrypted []byte
	err := s.db.QueryRowContext(ctx, `SELECT id,recipient,payload,kind,attempts FROM bot_outbox WHERE status='pending' AND available_at<=? ORDER BY recipient=? DESC,id LIMIT 1`, now.Unix(), AdminID).Scan(&n.ID, &n.Recipient, &encrypted, &n.Kind, &n.Attempts)
	if err != nil {
		return n, err
	}
	plain, err := s.decrypt(encrypted, []byte(fmt.Sprintf("notice:%d", n.Recipient)))
	n.Text = string(plain)
	return n, err
}
func (s *Store) finishNotice(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE bot_outbox SET status=? WHERE id=?`, status, id)
	return err
}
func (s *Store) retryNotice(ctx context.Context, n notice, delay time.Duration) error {
	_, err := s.db.ExecContext(ctx, `UPDATE bot_outbox SET attempts=attempts+1,available_at=? WHERE id=?`, time.Now().Add(delay).Unix(), n.ID)
	return err
}
