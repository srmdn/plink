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

func migrate(conn *sql.DB) error {
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS links (
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

		CREATE INDEX IF NOT EXISTS idx_clicks_link_id   ON clicks(link_id);
		CREATE INDEX IF NOT EXISTS idx_clicks_clicked_at ON clicks(clicked_at);
	`)
	return err
}
