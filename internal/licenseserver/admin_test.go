package licenseserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func applyCommand(t *testing.T, s *Store, update int64, text string, now time.Time) {
	t.Helper()
	if err := s.ApplyAdmin(context.Background(), update, AdminID, text, now); err != nil {
		t.Fatal(err)
	}
}
func licenseState(t *testing.T, s *Store, id int64) (int64, bool, bool) {
	t.Helper()
	var expires int64
	var revoked, bound bool
	if err := s.db.QueryRow(`SELECT expires_at,revoked,bound FROM licenses WHERE telegram_id=?`, id).Scan(&expires, &revoked, &bound); err != nil {
		t.Fatal(err)
	}
	return expires, revoked, bound
}
func noticesFor(t *testing.T, s *Store, id int64) []string {
	t.Helper()
	rows, err := s.db.Query(`SELECT payload FROM bot_outbox WHERE recipient=? ORDER BY id`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var texts []string
	for rows.Next() {
		var encrypted []byte
		if err = rows.Scan(&encrypted); err != nil {
			t.Fatal(err)
		}
		plain, err := s.decrypt(encrypted, []byte(fmt.Sprintf("notice:%d", id)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encrypted), string(plain)) {
			t.Fatal("notification stored in plaintext")
		}
		texts = append(texts, string(plain))
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestAdminAddSetAndReplay(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Unix(1800000000, 0)
	l, err := s.Issue(ctx, 42, now)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		applyCommand(t, s, 100, "/add 42 20", now)
	}
	expires, revoked, _ := licenseState(t, s, 42)
	if expires != l.ExpiresAt+20*86400 || revoked {
		t.Fatal("add lost days or applied twice")
	}
	texts := noticesFor(t, s, 42)
	if len(texts) != 1 || texts[0] != "Вам начислено 20 дополнительных дней доступа в Mythra" {
		t.Fatal(texts)
	}
	applyCommand(t, s, 101, "/set 42 20", now)
	expires, revoked, _ = licenseState(t, s, 42)
	if expires != now.Add(20*24*time.Hour).Unix() || revoked {
		t.Fatal("set did not replace remaining days")
	}
	applyCommand(t, s, 102, "/set 42 0", now)
	expires, revoked, _ = licenseState(t, s, 42)
	if expires != now.Unix() || revoked {
		t.Fatal("set 0 left access active")
	}
	checkCode(t, s, req(l.Key, "pc"), true, now, "EXPIRED")
	texts = noticesFor(t, s, 42)
	if texts[1] != "Время истечения подписки Mythra изменено на 20 дней" || texts[2] != "Ваша подписка Mythra была деактивирована" {
		t.Fatal(texts)
	}
	applyCommand(t, s, 103, "/add 42 2", now.Add(time.Hour))
	expires, revoked, _ = licenseState(t, s, 42)
	if expires != now.Add(time.Hour+48*time.Hour).Unix() || revoked {
		t.Fatal("add did not reactivate expired key")
	}
}

func TestAdminResetReleasesEveryHardwareAlias(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now()
	a, err := s.Issue(ctx, 42, now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Issue(ctx, 43, now)
	if err != nil {
		t.Fatal(err)
	}
	original := req(a.Key, "old-pc")
	original.Hardware = append(original.Hardware, hw("board")[0])
	checkCode(t, s, original, true, now, "OK")
	applyCommand(t, s, 100, "/reset_hwid 42", now)
	_, _, bound := licenseState(t, s, 42)
	if bound {
		t.Fatal("binding flag survived reset")
	}
	var aliases int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM hardware WHERE license_id=?`, a.ID).Scan(&aliases); err != nil || aliases != 0 {
		t.Fatal(aliases, err)
	}
	checkCode(t, s, req(a.Key, "new-pc"), true, now, "OK")
	original.Key = b.Key
	checkCode(t, s, original, true, now, "OK")
}

func TestAdminPublicAddIsAtomicAndRenewsCancelledSubscriptions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Unix(1800000000, 0)
	active, err := s.Issue(ctx, 42, now)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := s.Issue(ctx, 43, now.Add(-40*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	applyCommand(t, s, 10, "/set 43 0", now)
	for range 2 {
		applyCommand(t, s, 11, "/public_add 20", now)
	}
	expires, revoked, _ := licenseState(t, s, 42)
	if expires != active.ExpiresAt+20*86400 || revoked {
		t.Fatal("global add changed active key incorrectly")
	}
	expires, revoked, _ = licenseState(t, s, 43)
	if expires != now.Add(20*24*time.Hour).Unix() || revoked {
		t.Fatal("global add did not renew the cancelled subscription")
	}
	checkCode(t, s, req(cancelled.Key, "renewed-pc"), true, now, "OK")
	if texts := noticesFor(t, s, 42); len(texts) != 1 || texts[0] != "Вам начислено 20 дополнительных дней доступа в Mythra" {
		t.Fatal(texts)
	}
	if texts := noticesFor(t, s, 43); len(texts) != 2 || texts[1] != "Вам начислено 20 дополнительных дней доступа в Mythra" {
		t.Fatal(texts)
	}
}

func TestAdminGetExportsLicenseAndEquipmentWithoutResettingTrial(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	l, err := s.Issue(context.Background(), 42, now)
	if err != nil {
		t.Fatal(err)
	}
	r := req(l.Key, "pc")
	checkCode(t, s, r, true, now, "OK")
	applyCommand(t, s, 1, "/get 42", now)
	texts := noticesFor(t, s, AdminID)
	var info licenseInfo
	if len(texts) != 1 {
		t.Fatal(texts)
	}
	if err = json.Unmarshal([]byte(texts[0]), &info); err != nil {
		t.Fatal(err)
	}
	if info.Key != l.Key || info.ID != l.ID || !info.Bound || len(info.Hardware) != 1 || info.Hardware[0] != r.Hardware[0] || info.TelegramID != 42 {
		t.Fatal("incomplete user export")
	}
	applyCommand(t, s, 2, "/add 999 20", now)
	if texts = noticesFor(t, s, AdminID); texts[len(texts)-1] != "Пользователь не найден" {
		t.Fatal(texts)
	}
	var count int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM licenses`).Scan(&count); err != nil || count != 1 {
		t.Fatal("missing user received a new trial")
	}
}

func TestInvalidCommandsAndUnauthorizedActorsCannotMutate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now()
	l, err := s.Issue(ctx, 42, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ApplyAdmin(ctx, 1, 42, "/public_add 20", now); err == nil {
		t.Fatal("non-admin accepted")
	}
	for index, text := range []string{"/add 42 0", "/add 42 -1", "/set 42 -1", "/set 42 3651", "/reset_hwid", "/public_add -1", "/get 0", "/get abc", "/add 42 2 extra", "/add@another_bot 42 20"} {
		applyCommand(t, s, int64(index+10), text, now)
	}
	expires, revoked, _ := licenseState(t, s, 42)
	if expires != l.ExpiresAt || revoked || len(noticesFor(t, s, 42)) != 0 {
		t.Fatal("invalid command changed license")
	}
	if _, err = s.nextNotice(ctx, now.Add(-time.Hour)); err != sql.ErrNoRows {
		t.Fatal("future notice delivered early", err)
	}
}

func TestAdminMigrationKeepsExistingKeyAndHardware(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/old.sqlite"
	enc := make([]byte, 32)
	lookup := []byte("0123456789abcdef0123456789abcdef")
	old, err := Open(path, enc, lookup)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	l, err := old.Issue(ctx, 42, now)
	if err != nil {
		t.Fatal(err)
	}
	checkCode(t, old, req(l.Key, "pc"), true, now, "OK")
	for _, statement := range []string{`DROP TABLE admin_updates`, `DROP TABLE bot_outbox`, `PRAGMA user_version=1`} {
		if _, err = old.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err = old.Close(); err != nil {
		t.Fatal(err)
	}
	migrated, err := Open(path, enc, lookup)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	actual, err := migrated.Issue(ctx, 42, now)
	if err != nil || actual != l {
		t.Fatal("migration changed existing license", err)
	}
	checkCode(t, migrated, req(l.Key, "pc"), false, now, "OK")
	applyCommand(t, migrated, 1, "/add 42 20", now)
	checkCode(t, migrated, req(l.Key, "other-pc"), true, now, "OTHER_PC")
}
