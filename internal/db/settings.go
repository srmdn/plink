package db

import (
	"database/sql"
	"errors"
	"time"
)

type SiteSettings struct {
	ItemPreviews        bool
	PublicOrigin        string
	HeroEyebrow         string
	HeroTitle           string
	HeroDescription     string
	ServiceTitle        string
	ServiceDescription  string
	SEOTitle            string
	SEODescription      string
	ShareImageURL       string
	LogoURL             string
	FaviconURL          string
	AvatarURL           string
	SocialLinks         string
	SiteName            string
	SiteDesc            string
	AffiliateDisclosure string
	HeroEnabled         bool
	FeaturedLimit       int
	Configured          bool
	PublicAccent        string
	PublicBackground    string
	AdminAccent         string
	AdminBackground     string
}

func (db *DB) GetSiteSettings() (SiteSettings, error) {
	settings := SiteSettings{ItemPreviews: true, PublicOrigin: "", HeroEyebrow: "Rekomendasi saya", HeroTitle: "Barang bagus,\nuntuk kebutuhan Anda.", HeroDescription: "Kumpulan barang, kelas, dan jasa yang saya pakai dan rekomendasikan.", ServiceTitle: "Jasa", ServiceDescription: "Butuh membuat website, membereskan WordPress, atau mengurus VPS? Ceritakan kebutuhan Anda.", HeroEnabled: true, FeaturedLimit: 3, PublicAccent: "#c43424", PublicBackground: "#f4f1e9", AdminAccent: "#c43424", AdminBackground: "#f4f1e9"}
	err := db.QueryRow(`
		SELECT site_name, site_desc, affiliate_disclosure, hero_enabled, featured_limit, public_accent, public_background, admin_accent, admin_background, logo_url, favicon_url, avatar_url, social_links, seo_title, seo_description, share_image_url, public_origin, hero_eyebrow, hero_title, hero_description, service_title, service_description, item_previews
		FROM site_settings WHERE id = 1
	`).Scan(&settings.SiteName, &settings.SiteDesc, &settings.AffiliateDisclosure, &settings.HeroEnabled, &settings.FeaturedLimit, &settings.PublicAccent, &settings.PublicBackground, &settings.AdminAccent, &settings.AdminBackground, &settings.LogoURL, &settings.FaviconURL, &settings.AvatarURL, &settings.SocialLinks, &settings.SEOTitle, &settings.SEODescription, &settings.ShareImageURL, &settings.PublicOrigin, &settings.HeroEyebrow, &settings.HeroTitle, &settings.HeroDescription, &settings.ServiceTitle, &settings.ServiceDescription, &settings.ItemPreviews)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	settings.Configured = err == nil
	return settings, err
}

func (db *DB) SaveSiteSettings(settings SiteSettings) error {
	_, err := db.Exec(`
		INSERT INTO site_settings (id, site_name, site_desc, affiliate_disclosure, hero_enabled, featured_limit, public_accent, public_background, admin_accent, admin_background, logo_url, favicon_url, avatar_url, social_links, seo_title, seo_description, share_image_url, public_origin, hero_eyebrow, hero_title, hero_description, service_title, service_description, item_previews, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			site_name = excluded.site_name,
			site_desc = excluded.site_desc,
			affiliate_disclosure = excluded.affiliate_disclosure,
			hero_enabled = excluded.hero_enabled,
			featured_limit = excluded.featured_limit,
			public_accent = excluded.public_accent, public_background = excluded.public_background,
			admin_accent = excluded.admin_accent, admin_background = excluded.admin_background,
			logo_url = excluded.logo_url, favicon_url = excluded.favicon_url,
			avatar_url = excluded.avatar_url, social_links = excluded.social_links,
			seo_title = excluded.seo_title, seo_description = excluded.seo_description, share_image_url = excluded.share_image_url,
			public_origin = excluded.public_origin,
			hero_eyebrow = excluded.hero_eyebrow,
			hero_title = excluded.hero_title,
			hero_description = excluded.hero_description,
			service_title = excluded.service_title,
			service_description = excluded.service_description,
			item_previews = excluded.item_previews,
			updated_at = excluded.updated_at
	`, settings.SiteName, settings.SiteDesc, settings.AffiliateDisclosure, settings.HeroEnabled, settings.FeaturedLimit, settings.PublicAccent, settings.PublicBackground, settings.AdminAccent, settings.AdminBackground, settings.LogoURL, settings.FaviconURL, settings.AvatarURL, settings.SocialLinks, settings.SEOTitle, settings.SEODescription, settings.ShareImageURL, settings.PublicOrigin, settings.HeroEyebrow, settings.HeroTitle, settings.HeroDescription, settings.ServiceTitle, settings.ServiceDescription, settings.ItemPreviews, time.Now().Unix())
	return err
}
