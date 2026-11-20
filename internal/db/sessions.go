package db

import (
	"database/sql"
	"errors"
	"time"
)

// Session is one persisted admin login. TokenHash and CSRFHash are SHA-256
// hex digests: the raw tokens only ever live in cookies.
type Session struct {
	TokenHash string
	CSRFHash  string
	ExpiresAt time.Time
}

// CreateSession stores a new session that expires at the given instant.
func (db *DB) CreateSession(tokenHash, csrfHash string, expiresAt time.Time) error {
	_, err := db.Exec(`
		INSERT INTO sessions (token_hash, csrf_hash, expires_at) VALUES (?, ?, ?)
	`, tokenHash, csrfHash, expiresAt.Unix())
	return err
}

// SessionValid reports whether a session exists and has not expired.
func (db *DB) SessionValid(tokenHash string, now time.Time) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM sessions WHERE token_hash = ? AND expires_at > ?
	`, tokenHash, now.Unix()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SessionCSRFHash returns the CSRF token hash bound to a live session. found is
// false when the session does not exist or has already expired.
func (db *DB) SessionCSRFHash(tokenHash string, now time.Time) (hash string, found bool, err error) {
	err = db.QueryRow(`
		SELECT csrf_hash FROM sessions WHERE token_hash = ? AND expires_at > ?
	`, tokenHash, now.Unix()).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return hash, true, nil
}

// UpdateSessionCSRF rebinds a live session to a new CSRF token hash.
func (db *DB) UpdateSessionCSRF(tokenHash, csrfHash string, now time.Time) error {
	_, err := db.Exec(`
		UPDATE sessions SET csrf_hash = ? WHERE token_hash = ? AND expires_at > ?
	`, csrfHash, tokenHash, now.Unix())
	return err
}

// DeleteSession removes one session, which is what logout does.
func (db *DB) DeleteSession(tokenHash string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteExpiredSessions drops sessions whose TTL has passed and returns how
// many rows were removed.
func (db *DB) DeleteExpiredSessions(now time.Time) (int64, error) {
	result, err := db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now.Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
