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
		conn.Close()
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

	// v4: add homepage curation metadata
	`ALTER TABLE links ADD COLUMN featured INTEGER NOT NULL DEFAULT 0;
	 ALTER TABLE links ADD COLUMN priority INTEGER NOT NULL DEFAULT 0`,

	// v5: add promotion grouping metadata without changing public slugs
	`ALTER TABLE links ADD COLUMN provider TEXT NOT NULL DEFAULT '';
	 ALTER TABLE links ADD COLUMN channel TEXT NOT NULL DEFAULT '';
	 ALTER TABLE links ADD COLUMN campaign TEXT NOT NULL DEFAULT '';
	 CREATE INDEX IF NOT EXISTS idx_links_provider ON links(provider);
	 CREATE INDEX IF NOT EXISTS idx_links_channel ON links(channel)`,

	// v6: offers are public cards; links remain the redirectable channel variants.
	`CREATE TABLE IF NOT EXISTS offers (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		title        TEXT NOT NULL,
		provider     TEXT NOT NULL DEFAULT '',
		description  TEXT NOT NULL DEFAULT '',
		category     TEXT NOT NULL DEFAULT '',
		image_url    TEXT NOT NULL DEFAULT '',
		button_label TEXT NOT NULL DEFAULT 'Lihat promo',
		fallback_url TEXT NOT NULL DEFAULT '',
		starts_on    TEXT NOT NULL DEFAULT '',
		ends_on      TEXT NOT NULL DEFAULT '',
		active       INTEGER NOT NULL DEFAULT 1,
		featured     INTEGER NOT NULL DEFAULT 0,
		priority     INTEGER NOT NULL DEFAULT 0,
		created_at   INTEGER NOT NULL,
		updated_at   INTEGER NOT NULL
	);
	ALTER TABLE links ADD COLUMN offer_id INTEGER REFERENCES offers(id) ON DELETE SET NULL;
	ALTER TABLE links ADD COLUMN offer_homepage INTEGER NOT NULL DEFAULT 0;
	CREATE INDEX IF NOT EXISTS idx_links_offer_id ON links(offer_id);
	CREATE INDEX IF NOT EXISTS idx_offers_active_priority ON offers(active, featured, priority);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_links_offer_homepage
		ON links(offer_id) WHERE offer_id IS NOT NULL AND offer_homepage = 1`,

	// v7: persist public storefront preferences separately from offer content.
	`CREATE TABLE IF NOT EXISTS site_settings (
		id                   INTEGER PRIMARY KEY CHECK (id = 1),
		site_name            TEXT NOT NULL DEFAULT '',
		site_desc            TEXT NOT NULL DEFAULT '',
		affiliate_disclosure TEXT NOT NULL DEFAULT '',
		hero_enabled         INTEGER NOT NULL DEFAULT 1,
		featured_limit       INTEGER NOT NULL DEFAULT 3,
		updated_at           INTEGER NOT NULL
	)`,

	// v8: program lifecycle and notice views, separate from redirect clicks.
	`ALTER TABLE offers ADD COLUMN program_status TEXT NOT NULL DEFAULT 'active' CHECK (program_status IN ('active', 'paused', 'ended'));
	ALTER TABLE offers ADD COLUMN ended_behavior TEXT NOT NULL DEFAULT 'notice' CHECK (ended_behavior IN ('notice', 'redirect'));
	ALTER TABLE offers ADD COLUMN notice_message TEXT NOT NULL DEFAULT '';
	ALTER TABLE offers ADD COLUMN notice_source_url TEXT NOT NULL DEFAULT '';
	ALTER TABLE offers ADD COLUMN status_changed_on TEXT NOT NULL DEFAULT '';
	ALTER TABLE offers ADD COLUMN verified_on TEXT NOT NULL DEFAULT '';
	UPDATE offers SET ended_behavior = 'redirect' WHERE fallback_url != '';
	CREATE TABLE notice_views (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		link_id INTEGER NOT NULL REFERENCES links(id) ON DELETE CASCADE,
		viewed_at INTEGER NOT NULL,
		program_status TEXT NOT NULL,
		referrer TEXT NOT NULL DEFAULT '',
		user_agent TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX idx_notice_views_link_id ON notice_views(link_id);`,
	// v9: catalog types and independently configurable public/admin palettes.
	`ALTER TABLE offers ADD COLUMN item_type TEXT NOT NULL DEFAULT 'referral' CHECK (item_type IN ('product','referral','service','resource'));
	ALTER TABLE site_settings ADD COLUMN public_accent TEXT NOT NULL DEFAULT '#c43424';
	ALTER TABLE site_settings ADD COLUMN public_background TEXT NOT NULL DEFAULT '#f4f1e9';
	ALTER TABLE site_settings ADD COLUMN admin_accent TEXT NOT NULL DEFAULT '#c43424';
	ALTER TABLE site_settings ADD COLUMN admin_background TEXT NOT NULL DEFAULT '#f4f1e9';`,
	// v10: configurable brand image and favicon, without adding asset storage.
	`ALTER TABLE site_settings ADD COLUMN logo_url TEXT NOT NULL DEFAULT '';
	ALTER TABLE site_settings ADD COLUMN favicon_url TEXT NOT NULL DEFAULT '';`,
	// v11: public SEO and social sharing defaults.
	`ALTER TABLE site_settings ADD COLUMN seo_title TEXT NOT NULL DEFAULT '';
	ALTER TABLE site_settings ADD COLUMN seo_description TEXT NOT NULL DEFAULT '';
	ALTER TABLE site_settings ADD COLUMN share_image_url TEXT NOT NULL DEFAULT '';`,
	// v12: editable public address and storefront copy.
	`ALTER TABLE site_settings ADD COLUMN public_origin TEXT NOT NULL DEFAULT '';
ALTER TABLE site_settings ADD COLUMN hero_eyebrow TEXT NOT NULL DEFAULT 'Pilihan Said';
ALTER TABLE site_settings ADD COLUMN hero_title TEXT NOT NULL DEFAULT 'Temuan bagus.
Buat kebutuhan lo.';
ALTER TABLE site_settings ADD COLUMN hero_description TEXT NOT NULL DEFAULT 'Produk, referral, resource, dan jasa dalam satu tempat.';
ALTER TABLE site_settings ADD COLUMN service_title TEXT NOT NULL DEFAULT 'Jasa Said';
ALTER TABLE site_settings ADD COLUMN service_description TEXT NOT NULL DEFAULT 'Bantuan website, WordPress, dan VPS. Buka detail jasa untuk membahas kebutuhan lo.';`,
	// v13: automatic catalog previews on existing short URLs.
	`ALTER TABLE site_settings ADD COLUMN item_previews INTEGER NOT NULL DEFAULT 1 CHECK (item_previews IN (0, 1));`,
}

func migrate(conn *sql.DB) error {
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS _migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return err
	}

	for i, sql := range migrations {
		version := i + 1
		var count int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM _migrations WHERE version = ?`, version).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		tx, err := conn.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration v%d: %w", version, err)
		}
		if _, err := tx.Exec(`INSERT INTO _migrations (version) VALUES (?)`, version); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration v%d record: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration v%d commit: %w", version, err)
		}
	}
	return nil
}
