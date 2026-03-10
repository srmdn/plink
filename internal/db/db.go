package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Init(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	conn.SetMaxOpenConns(1)

	if err := migrate(conn); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{conn}, nil
}

var migrations = []string{
	// v1: initial schema
	`CREATE TABLE IF NOT EXISTS links (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		slug        TEXT    UNIQUE NOT NULL,
		url         TEXT    NOT NULL,
		description TEXT    NOT NULL DEFAULT '',
		created_at  INTEGER NOT NULL,
		updated_at  INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS clicks (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		link_id    INTEGER NOT NULL REFERENCES links(id) ON DELETE CASCADE,
		clicked_at INTEGER NOT NULL,
		referrer   TEXT    NOT NULL DEFAULT '',
		user_agent TEXT    NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_clicks_link_id    ON clicks(link_id);
	CREATE INDEX IF NOT EXISTS idx_clicks_clicked_at ON clicks(clicked_at);`,

	// v2: add category to links
	`ALTER TABLE links ADD COLUMN category TEXT NOT NULL DEFAULT ''`,

	// v3: add active flag to links
	`ALTER TABLE links ADD COLUMN active INTEGER NOT NULL DEFAULT 1`,
}

func migrate(conn *sql.DB) error {
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS _migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return err
	}

	for i, sql := range migrations {
		version := i + 1
		var count int
		conn.QueryRow(`SELECT COUNT(*) FROM _migrations WHERE version = ?`, version).Scan(&count)
		if count > 0 {
			continue
		}
		if _, err := conn.Exec(sql); err != nil {
			return fmt.Errorf("migration v%d: %w", version, err)
		}
		if _, err := conn.Exec(`INSERT INTO _migrations (version) VALUES (?)`, version); err != nil {
			return fmt.Errorf("migration v%d record: %w", version, err)
		}
	}
	return nil
}
