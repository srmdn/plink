package db

import (
	"database/sql"
	"sort"
	"time"
)

type Link struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Active      bool   `json:"active"`
	Featured    bool   `json:"featured"`
	Priority    int    `json:"priority"`
	Clicks      int64  `json:"clicks"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type DailyClicks struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

type Referrer struct {
	Source  string           `json:"source"`
	Clicks  int64            `json:"clicks"`
	Details []ReferrerDetail `json:"details,omitempty"`
}

type ReferrerDetail struct {
	Source string `json:"source"`
	Clicks int64  `json:"clicks"`
}

type SourceSummary struct {
	Source    string           `json:"source"`
	Clicks    int64            `json:"clicks"`
	LinkCount int64            `json:"link_count"`
	Details   []ReferrerDetail `json:"details,omitempty"`
}

type Analytics struct {
	TotalClicks int64         `json:"total_clicks"`
	LastClickAt int64         `json:"last_click_at"`
	Daily       []DailyClicks `json:"daily"`
	Referrers   []Referrer    `json:"referrers"`
}

type OverviewAnalytics struct {
	TotalClicks int64           `json:"total_clicks"`
	Last30d     int64           `json:"last_30d"`
	Referrers   []SourceSummary `json:"referrers"`
}

type LinkOptions struct {
	Featured bool
	Priority int
}

type PublicLink struct {
	ID          int64
	Slug        string
	Description string
	Category    string
	Featured    bool
	Priority    int
}

func (db *DB) ListPublicLinks() ([]PublicLink, error) {
	rows, err := db.Query(`
		SELECT id, slug, description, category, featured, priority
		FROM links
		WHERE active = 1
		ORDER BY priority DESC, category, slug
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []PublicLink
	for rows.Next() {
		var l PublicLink
		if err := rows.Scan(&l.ID, &l.Slug, &l.Description, &l.Category, &l.Featured, &l.Priority); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (db *DB) ListFeaturedLinks(limit int) ([]PublicLink, error) {
	rows, err := db.Query(`
		SELECT id, slug, description, category, featured, priority
		FROM links
		WHERE active = 1 AND featured = 1
		ORDER BY priority DESC, category, slug
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []PublicLink
	for rows.Next() {
		var l PublicLink
		if err := rows.Scan(&l.ID, &l.Slug, &l.Description, &l.Category, &l.Featured, &l.Priority); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (db *DB) ListLinks() ([]Link, error) {
	rows, err := db.Query(`
		SELECT l.id, l.slug, l.url, l.description, l.category, l.active, l.featured, l.priority,
		       l.created_at, l.updated_at,
		       COUNT(c.id) AS clicks
		FROM links l
		LEFT JOIN clicks c ON c.link_id = l.id
		GROUP BY l.id
		ORDER BY l.active DESC, l.featured DESC, l.priority DESC, l.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.Slug, &l.URL, &l.Description, &l.Category, &l.Active, &l.Featured, &l.Priority, &l.CreatedAt, &l.UpdatedAt, &l.Clicks); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// ClickCountsBetween returns per-link click totals for the half-open Unix
// timestamp range [start, end).
func (db *DB) ClickCountsBetween(start, end int64) (map[int64]int64, error) {
	counts := make(map[int64]int64)
	if end <= start {
		return counts, nil
	}

	rows, err := db.Query(`
		SELECT link_id, COUNT(*)
		FROM clicks
		WHERE clicked_at >= ? AND clicked_at < ?
		GROUP BY link_id
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var linkID, clicks int64
		if err := rows.Scan(&linkID, &clicks); err != nil {
			return nil, err
		}
		counts[linkID] = clicks
	}
	return counts, rows.Err()
}

func (db *DB) GetLinkBySlug(slug string) (*Link, error) {
	var l Link
	err := db.QueryRow(
		`SELECT id, slug, url, description, category, active, featured, priority, created_at, updated_at
		 FROM links WHERE slug = ? AND active = 1`, slug,
	).Scan(&l.ID, &l.Slug, &l.URL, &l.Description, &l.Category, &l.Active, &l.Featured, &l.Priority, &l.CreatedAt, &l.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &l, err
}

func (db *DB) GetLinkByID(id int64) (*Link, error) {
	var l Link
	err := db.QueryRow(`
		SELECT l.id, l.slug, l.url, l.description, l.category, l.active, l.featured, l.priority,
		       l.created_at, l.updated_at, COUNT(c.id) AS clicks
		FROM links l
		LEFT JOIN clicks c ON c.link_id = l.id
		WHERE l.id = ?
		GROUP BY l.id
	`, id).Scan(&l.ID, &l.Slug, &l.URL, &l.Description, &l.Category, &l.Active, &l.Featured, &l.Priority, &l.CreatedAt, &l.UpdatedAt, &l.Clicks)
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
	return db.CreateLinkWithOptions(slug, url, description, category, LinkOptions{})
}

func (db *DB) CreateLinkWithOptions(slug, url, description, category string, options LinkOptions) (*Link, error) {
	now := time.Now().Unix()
	res, err := db.Exec(
		`INSERT INTO links (slug, url, description, category, featured, priority, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		slug, url, description, category, options.Featured, options.Priority, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Link{
		ID: id, Slug: slug, URL: url, Description: description, Category: category,
		Featured: options.Featured, Priority: options.Priority, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (db *DB) UpdateLink(id int64, slug, url, description, category string) error {
	now := time.Now().Unix()
	_, err := db.Exec(
		`UPDATE links SET slug = ?, url = ?, description = ?, category = ?, updated_at = ? WHERE id = ?`,
		slug, url, description, category, now, id,
	)
	return err
}

func (db *DB) UpdateLinkWithOptions(id int64, slug, url, description, category string, options LinkOptions) error {
	now := time.Now().Unix()
	_, err := db.Exec(
		`UPDATE links
		 SET slug = ?, url = ?, description = ?, category = ?, featured = ?, priority = ?, updated_at = ?
		 WHERE id = ?`,
		slug, url, description, category, options.Featured, options.Priority, now, id,
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
	return db.GetAnalyticsInLocation(linkID, time.UTC)
}

func (db *DB) GetAnalyticsInLocation(linkID int64, location *time.Location) (*Analytics, error) {
	location = analyticsLocation(location)
	var total, lastClickAt sql.NullInt64
	if err := db.QueryRow(`SELECT COUNT(*), MAX(clicked_at) FROM clicks WHERE link_id = ?`, linkID).Scan(&total, &lastClickAt); err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT clicked_at
		FROM clicks
		WHERE link_id = ? AND clicked_at >= ?
		ORDER BY clicked_at ASC
	`, linkID, time.Now().In(location).AddDate(0, 0, -30).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dailyCounts := make(map[string]int64)
	for rows.Next() {
		var clickedAt int64
		if err := rows.Scan(&clickedAt); err != nil {
			return nil, err
		}
		day := time.Unix(clickedAt, 0).In(location).Format("2006-01-02")
		dailyCounts[day]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	daily := make([]DailyClicks, 0, len(dailyCounts))
	for day, clicks := range dailyCounts {
		daily = append(daily, DailyClicks{Date: day, Clicks: clicks})
	}
	sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })

	refRows, err := db.Query(`SELECT referrer FROM clicks WHERE link_id = ?`, linkID)
	if err != nil {
		return nil, err
	}
	defer refRows.Close()

	groups := make(map[string]*referrerAggregate)
	for refRows.Next() {
		var raw string
		if err := refRows.Scan(&raw); err != nil {
			return nil, err
		}
		addReferrer(groups, raw, linkID)
	}
	if err := refRows.Err(); err != nil {
		return nil, err
	}

	return &Analytics{
		TotalClicks: total.Int64,
		LastClickAt: lastClickAt.Int64,
		Daily:       daily,
		Referrers:   buildReferrers(groups, 10),
	}, nil
}

func (db *DB) GetOverviewAnalytics() (*OverviewAnalytics, error) {
	return db.GetOverviewAnalyticsInLocation(time.UTC)
}

func (db *DB) GetOverviewAnalyticsInLocation(location *time.Location) (*OverviewAnalytics, error) {
	location = analyticsLocation(location)
	var total, last30d int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM clicks`).Scan(&total); err != nil {
		return nil, err
	}
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM clicks WHERE clicked_at >= ?`,
		time.Now().In(location).AddDate(0, 0, -30).Unix(),
	).Scan(&last30d); err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT referrer, link_id FROM clicks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make(map[string]*referrerAggregate)
	for rows.Next() {
		var raw string
		var linkID int64
		if err := rows.Scan(&raw, &linkID); err != nil {
			return nil, err
		}
		addReferrer(groups, raw, linkID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &OverviewAnalytics{
		TotalClicks: total,
		Last30d:     last30d,
		Referrers:   buildSourceSummaries(groups, 10),
	}, nil
}

func analyticsLocation(location *time.Location) *time.Location {
	if location == nil {
		return time.UTC
	}
	return location
}
