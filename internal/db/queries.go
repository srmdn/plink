package db

import (
	"database/sql"
	"time"
)

type Link struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Active      bool   `json:"active"`
	Clicks      int64  `json:"clicks"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type DailyClicks struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

type Referrer struct {
	Source string `json:"source"`
	Clicks int64  `json:"clicks"`
}

type Analytics struct {
	TotalClicks int64         `json:"total_clicks"`
	Daily       []DailyClicks `json:"daily"`
	Referrers   []Referrer    `json:"referrers"`
}

type PublicLink struct {
	ID          int64
	Slug        string
	Description string
	Category    string
}

func (db *DB) ListPublicLinks() ([]PublicLink, error) {
	rows, err := db.Query(`
		SELECT id, slug, description, category
		FROM links
		WHERE active = 1
		ORDER BY category, slug
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []PublicLink
	for rows.Next() {
		var l PublicLink
		if err := rows.Scan(&l.ID, &l.Slug, &l.Description, &l.Category); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (db *DB) ListLinks() ([]Link, error) {
	rows, err := db.Query(`
		SELECT l.id, l.slug, l.url, l.description, l.category, l.active, l.created_at, l.updated_at,
		       COUNT(c.id) AS clicks
		FROM links l
		LEFT JOIN clicks c ON c.link_id = l.id
		GROUP BY l.id
		ORDER BY l.active DESC, l.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.Slug, &l.URL, &l.Description, &l.Category, &l.Active, &l.CreatedAt, &l.UpdatedAt, &l.Clicks); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (db *DB) GetLinkBySlug(slug string) (*Link, error) {
	var l Link
	err := db.QueryRow(
		`SELECT id, slug, url, description, created_at, updated_at FROM links WHERE slug = ? AND active = 1`, slug,
	).Scan(&l.ID, &l.Slug, &l.URL, &l.Description, &l.CreatedAt, &l.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &l, err
}

func (db *DB) ToggleLink(id int64) error {
	_, err := db.Exec(`UPDATE links SET active = NOT active, updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

func (db *DB) CreateLink(slug, url, description, category string) (*Link, error) {
	now := time.Now().Unix()
	res, err := db.Exec(
		`INSERT INTO links (slug, url, description, category, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		slug, url, description, category, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Link{ID: id, Slug: slug, URL: url, Description: description, Category: category, CreatedAt: now, UpdatedAt: now}, nil
}

func (db *DB) UpdateLink(id int64, slug, url, description, category string) error {
	now := time.Now().Unix()
	_, err := db.Exec(
		`UPDATE links SET slug = ?, url = ?, description = ?, category = ?, updated_at = ? WHERE id = ?`,
		slug, url, description, category, now, id,
	)
	return err
}

func (db *DB) DeleteLink(id int64) error {
	_, err := db.Exec(`DELETE FROM links WHERE id = ?`, id)
	return err
}

func (db *DB) RecordClick(linkID int64, referrer, userAgent string) error {
	_, err := db.Exec(
		`INSERT INTO clicks (link_id, clicked_at, referrer, user_agent) VALUES (?, ?, ?, ?)`,
		linkID, time.Now().Unix(), referrer, userAgent,
	)
	return err
}

func (db *DB) GetAnalytics(linkID int64) (*Analytics, error) {
	var total int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM clicks WHERE link_id = ?`, linkID).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT date(clicked_at, 'unixepoch') AS day, COUNT(*) AS cnt
		FROM clicks
		WHERE link_id = ? AND clicked_at >= ?
		GROUP BY day
		ORDER BY day ASC
	`, linkID, time.Now().AddDate(0, 0, -30).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var daily []DailyClicks
	for rows.Next() {
		var d DailyClicks
		if err := rows.Scan(&d.Date, &d.Clicks); err != nil {
			return nil, err
		}
		daily = append(daily, d)
	}
	rows.Close()

	refRows, err := db.Query(`
		SELECT COALESCE(NULLIF(referrer, ''), 'direct') AS src, COUNT(*) AS cnt
		FROM clicks
		WHERE link_id = ?
		GROUP BY src
		ORDER BY cnt DESC
		LIMIT 10
	`, linkID)
	if err != nil {
		return nil, err
	}
	defer refRows.Close()

	var referrers []Referrer
	for refRows.Next() {
		var r Referrer
		if err := refRows.Scan(&r.Source, &r.Clicks); err != nil {
			return nil, err
		}
		referrers = append(referrers, r)
	}

	return &Analytics{
		TotalClicks: total,
		Daily:       daily,
		Referrers:   referrers,
	}, refRows.Err()
}
