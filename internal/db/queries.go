package db

import (
	"database/sql"
	"sort"
	"strings"
	"time"
)

type Link struct {
	ID          int64  `json:"id"`
	OfferID     int64  `json:"offer_id,omitempty"`
	OfferHome   bool   `json:"offer_homepage,omitempty"`
	OfferEndsOn string `json:"offer_ends_on,omitempty"`
	FallbackURL string `json:"fallback_url,omitempty"`
	Slug        string `json:"slug"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Provider    string `json:"provider"`
	Channel     string `json:"channel"`
	Campaign    string `json:"campaign"`
	Active      bool   `json:"active"`
	Featured    bool   `json:"featured"`
	Priority    int    `json:"priority"`
	Clicks      int64  `json:"clicks"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// Offer is one public card or promotion. Its links are independent redirect
// records so clicks can still be attributed to a particular channel.
type Offer struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Provider    string `json:"provider"`
	Description string `json:"description"`
	Category    string `json:"category"`
	ImageURL    string `json:"image_url"`
	ButtonLabel string `json:"button_label"`
	FallbackURL string `json:"fallback_url"`
	StartsOn    string `json:"starts_on"`
	EndsOn      string `json:"ends_on"`
	Active      bool   `json:"active"`
	Featured    bool   `json:"featured"`
	Priority    int    `json:"priority"`
	HomeSlug    string `json:"home_slug"`
	HomeURL     string `json:"home_url"`
	HomeActive  bool   `json:"home_active"`
	LinkCount   int    `json:"link_count"`
	Clicks      int64  `json:"clicks"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type OfferInput struct {
	Title       string
	Provider    string
	Description string
	Category    string
	ImageURL    string
	ButtonLabel string
	FallbackURL string
	StartsOn    string
	EndsOn      string
	Active      bool
	Featured    bool
	Priority    int
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
	Featured  bool
	Priority  int
	Provider  string
	Channel   string
	Campaign  string
	OfferID   int64
	OfferHome bool
}

type PublicLink struct {
	ID          int64
	Slug        string
	Description string
	Category    string
	Featured    bool
	Priority    int
}

func (db *DB) ListOffers() ([]Offer, error) {
	rows, err := db.Query(`
		SELECT o.id, o.title, o.provider, o.description, o.category, o.image_url,
		       o.button_label, o.fallback_url, o.starts_on, o.ends_on,
		       o.active, o.featured, o.priority,
		       COALESCE(h.slug, ''), COALESCE(h.url, ''), COALESCE(h.active, 0),
		       COUNT(DISTINCT l.id), COUNT(c.id), o.created_at, o.updated_at
		FROM offers o
		LEFT JOIN links l ON l.offer_id = o.id
		LEFT JOIN clicks c ON c.link_id = l.id
		LEFT JOIN links h ON h.offer_id = o.id AND h.offer_homepage = 1
		GROUP BY o.id
		ORDER BY o.active DESC, o.featured DESC, o.priority DESC, o.updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offers []Offer
	for rows.Next() {
		var offer Offer
		if err := rows.Scan(&offer.ID, &offer.Title, &offer.Provider, &offer.Description, &offer.Category, &offer.ImageURL,
			&offer.ButtonLabel, &offer.FallbackURL, &offer.StartsOn, &offer.EndsOn,
			&offer.Active, &offer.Featured, &offer.Priority, &offer.HomeSlug, &offer.HomeURL, &offer.HomeActive,
			&offer.LinkCount, &offer.Clicks, &offer.CreatedAt, &offer.UpdatedAt); err != nil {
			return nil, err
		}
		offers = append(offers, offer)
	}
	return offers, rows.Err()
}

func (db *DB) GetOfferByID(id int64) (*Offer, error) {
	offers, err := db.ListOffers()
	if err != nil {
		return nil, err
	}
	for i := range offers {
		if offers[i].ID == id {
			return &offers[i], nil
		}
	}
	return nil, nil
}

func (db *DB) ListOfferLinks(offerID int64) ([]Link, error) {
	rows, err := db.Query(`
		SELECT l.id, l.slug, l.url, l.description, l.category, l.provider, l.channel, l.campaign,
		       l.active, l.featured, l.priority, l.created_at, l.updated_at,
		       COALESCE(l.offer_id, 0), l.offer_homepage, COALESCE(o.ends_on, ''), COALESCE(o.fallback_url, ''),
		       COUNT(c.id)
		FROM links l
		LEFT JOIN offers o ON o.id = l.offer_id
		LEFT JOIN clicks c ON c.link_id = l.id
		WHERE l.offer_id = ?
		GROUP BY l.id
		ORDER BY l.offer_homepage DESC, l.channel, l.created_at
	`, offerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var link Link
		if err := rows.Scan(&link.ID, &link.Slug, &link.URL, &link.Description, &link.Category, &link.Provider, &link.Channel, &link.Campaign,
			&link.Active, &link.Featured, &link.Priority, &link.CreatedAt, &link.UpdatedAt,
			&link.OfferID, &link.OfferHome, &link.OfferEndsOn, &link.FallbackURL, &link.Clicks); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (db *DB) CreateOfferWithHomepageLink(input OfferInput, slug, url string) (*Offer, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	if strings.TrimSpace(input.ButtonLabel) == "" {
		input.ButtonLabel = "Lihat promo"
	}
	result, err := tx.Exec(`
		INSERT INTO offers (title, provider, description, category, image_url, button_label, fallback_url,
		                    starts_on, ends_on, active, featured, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.Title, input.Provider, input.Description, input.Category, input.ImageURL, input.ButtonLabel, input.FallbackURL,
		input.StartsOn, input.EndsOn, input.Active, input.Featured, input.Priority, now, now)
	if err != nil {
		return nil, err
	}
	offerID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`
		INSERT INTO links (slug, url, description, category, provider, channel, campaign, featured, priority,
		                   offer_id, offer_homepage, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'homepage', '', 0, 0, ?, 1, ?, ?)
	`, slug, url, input.Description, input.Category, input.Provider, offerID, now, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return db.GetOfferByID(offerID)
}

func (db *DB) UpdateOffer(id int64, input OfferInput) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	if strings.TrimSpace(input.ButtonLabel) == "" {
		input.ButtonLabel = "Lihat promo"
	}
	if _, err := tx.Exec(`
		UPDATE offers
		SET title = ?, provider = ?, description = ?, category = ?, image_url = ?, button_label = ?, fallback_url = ?,
		    starts_on = ?, ends_on = ?, active = ?, featured = ?, priority = ?, updated_at = ?
		WHERE id = ?
	`, input.Title, input.Provider, input.Description, input.Category, input.ImageURL, input.ButtonLabel, input.FallbackURL,
		input.StartsOn, input.EndsOn, input.Active, input.Featured, input.Priority, now, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE links
		SET description = ?, category = ?, provider = ?, updated_at = ?
		WHERE offer_id = ?
	`, input.Description, input.Category, input.Provider, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) CreateOfferLink(offerID int64, slug, url, channel string) (*Link, error) {
	offer, err := db.GetOfferByID(offerID)
	if err != nil || offer == nil {
		return nil, err
	}
	return db.CreateLinkWithOptions(slug, url, "", offer.Category, LinkOptions{
		Provider: offer.Provider,
		Channel:  channel,
		OfferID:  offerID,
	})
}

func (db *DB) SetOfferHomepageLink(offerID, linkID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE links SET offer_homepage = 0 WHERE offer_id = ?`, offerID); err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE links SET offer_homepage = 1, updated_at = ? WHERE id = ? AND offer_id = ? AND active = 1`, time.Now().Unix(), linkID, offerID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (db *DB) ToggleOffer(id int64) error {
	_, err := db.Exec(`UPDATE offers SET active = NOT active, updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

func (db *DB) ListPublicLinks() ([]PublicLink, error) {
	rows, err := db.Query(`
		SELECT id, slug, description, category, featured, priority
		FROM links
		WHERE active = 1 AND offer_id IS NULL
		ORDER BY featured DESC, priority DESC, category, slug
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
		WHERE active = 1 AND featured = 1 AND offer_id IS NULL
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
		SELECT l.id, l.slug, l.url, l.description, l.category, l.provider, l.channel, l.campaign,
		       l.active, l.featured, l.priority,
		       l.created_at, l.updated_at, COALESCE(l.offer_id, 0), l.offer_homepage,
		       COALESCE(o.ends_on, ''), COALESCE(o.fallback_url, ''),
		       COUNT(c.id) AS clicks
		FROM links l
		LEFT JOIN offers o ON o.id = l.offer_id
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
		if err := rows.Scan(&l.ID, &l.Slug, &l.URL, &l.Description, &l.Category, &l.Provider, &l.Channel, &l.Campaign, &l.Active, &l.Featured, &l.Priority, &l.CreatedAt, &l.UpdatedAt, &l.OfferID, &l.OfferHome, &l.OfferEndsOn, &l.FallbackURL, &l.Clicks); err != nil {
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
		`SELECT l.id, l.slug, l.url, l.description, l.category, l.provider, l.channel, l.campaign,
		        l.active, l.featured, l.priority, l.created_at, l.updated_at,
		        COALESCE(l.offer_id, 0), l.offer_homepage, COALESCE(o.ends_on, ''), COALESCE(o.fallback_url, '')
		 FROM links l LEFT JOIN offers o ON o.id = l.offer_id
		 WHERE l.slug = ? AND l.active = 1`, slug,
	).Scan(&l.ID, &l.Slug, &l.URL, &l.Description, &l.Category, &l.Provider, &l.Channel, &l.Campaign, &l.Active, &l.Featured, &l.Priority, &l.CreatedAt, &l.UpdatedAt, &l.OfferID, &l.OfferHome, &l.OfferEndsOn, &l.FallbackURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &l, err
}

func (db *DB) GetLinkByID(id int64) (*Link, error) {
	var l Link
	err := db.QueryRow(`
		SELECT l.id, l.slug, l.url, l.description, l.category, l.provider, l.channel, l.campaign,
		       l.active, l.featured, l.priority,
		       l.created_at, l.updated_at, COALESCE(l.offer_id, 0), l.offer_homepage,
		       COALESCE(o.ends_on, ''), COALESCE(o.fallback_url, ''), COUNT(c.id) AS clicks
		FROM links l
		LEFT JOIN offers o ON o.id = l.offer_id
		LEFT JOIN clicks c ON c.link_id = l.id
		WHERE l.id = ?
		GROUP BY l.id
	`, id).Scan(&l.ID, &l.Slug, &l.URL, &l.Description, &l.Category, &l.Provider, &l.Channel, &l.Campaign, &l.Active, &l.Featured, &l.Priority, &l.CreatedAt, &l.UpdatedAt, &l.OfferID, &l.OfferHome, &l.OfferEndsOn, &l.FallbackURL, &l.Clicks)
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
		`INSERT INTO links (slug, url, description, category, provider, channel, campaign, featured, priority, offer_id, offer_homepage, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?)`,
		slug, url, description, category, options.Provider, options.Channel, options.Campaign, options.Featured, options.Priority, options.OfferID, options.OfferHome, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Link{
		ID: id, Slug: slug, URL: url, Description: description, Category: category,
		Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign,
		OfferID: options.OfferID, OfferHome: options.OfferHome,
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
		 SET slug = ?, url = ?, description = ?, category = ?, provider = ?, channel = ?, campaign = ?,
		     featured = ?, priority = ?, updated_at = ?
		 WHERE id = ?`,
		slug, url, description, category, options.Provider, options.Channel, options.Campaign, options.Featured, options.Priority, now, id,
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

// GetReferrersBetween returns the top traffic sources for one link in the
// half-open click range [start, end). Callers should derive the boundaries in
// their reporting timezone before querying.
func (db *DB) GetReferrersBetween(linkID, start, end int64) ([]Referrer, error) {
	rows, err := db.Query(`
		SELECT referrer
		FROM clicks
		WHERE link_id = ? AND clicked_at >= ? AND clicked_at < ?
	`, linkID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make(map[string]*referrerAggregate)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		addReferrer(groups, raw, linkID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return buildReferrers(groups, 10), nil
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

// GetOverviewAnalyticsBetween returns aggregate analytics for the half-open
// click range [start, end). Callers should derive the boundaries in their
// reporting timezone before querying.
func (db *DB) GetOverviewAnalyticsBetween(start, end int64) (*OverviewAnalytics, error) {
	var total int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM clicks WHERE clicked_at >= ? AND clicked_at < ?`, start, end).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT referrer, link_id
		FROM clicks
		WHERE clicked_at >= ? AND clicked_at < ?
	`, start, end)
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
		Referrers:   buildSourceSummaries(groups, 10),
	}, nil
}

// GetOverviewAnalyticsForLinks returns aggregate analytics for a selected set
// of links. A nil range means all time; otherwise the half-open range
// [start, end) is used. This keeps overview filters scoped to the same links
// shown in the dashboard instead of mixing category-specific totals with
// site-wide traffic sources.
func (db *DB) GetOverviewAnalyticsForLinks(linkIDs []int64, start, end *int64) (*OverviewAnalytics, error) {
	if len(linkIDs) == 0 {
		return &OverviewAnalytics{}, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(linkIDs)), ",")
	where := "link_id IN (" + placeholders + ")"
	args := make([]any, 0, len(linkIDs)+2)
	for _, id := range linkIDs {
		args = append(args, id)
	}
	if start != nil && end != nil {
		where += " AND clicked_at >= ? AND clicked_at < ?"
		args = append(args, *start, *end)
	}

	var total int64
	if err := db.QueryRow("SELECT COUNT(*) FROM clicks WHERE "+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT referrer, link_id FROM clicks WHERE "+where, args...)
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
		Referrers:   buildSourceSummaries(groups, 10),
	}, nil
}

func analyticsLocation(location *time.Location) *time.Location {
	if location == nil {
		return time.UTC
	}
	return location
}
