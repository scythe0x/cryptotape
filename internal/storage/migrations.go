package storage

import "database/sql"

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version >= 1 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`ALTER TABLE rules ADD COLUMN color TEXT NOT NULL DEFAULT '#a78bfa'`,
		`ALTER TABLE rules ADD COLUMN volume REAL NOT NULL DEFAULT 1`,
		`ALTER TABLE rules ADD COLUMN min_quote_volume REAL NOT NULL DEFAULT 80000000`,
		`ALTER TABLE rules ADD COLUMN terminals TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE alerts ADD COLUMN color TEXT NOT NULL DEFAULT '#a78bfa'`,
		`ALTER TABLE alerts ADD COLUMN volume REAL NOT NULL DEFAULT 1`,
		`ALTER TABLE alerts ADD COLUMN terminals TEXT NOT NULL DEFAULT '[]'`,
		`UPDATE rules SET terminals=CASE WHEN open_terminal=1 THEN '["vataga"]' ELSE '[]' END, enabled=1,
   min_quote_volume=COALESCE((SELECT json_extract(payload,'$.minQuoteVolume') FROM settings WHERE id=1),80000000)`,
		`UPDATE alerts SET terminals=CASE WHEN open_terminal=1 THEN '["vataga"]' ELSE '[]' END`,
		`PRAGMA user_version=1`,
	}
	for _, statement := range statements {
		if _, err = tx.Exec(statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}
