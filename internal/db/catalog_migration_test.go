package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCatalogMigrationPreservesExistingContentAndHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v8.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(`CREATE TABLE _migrations(version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range migrations[:8] {
		if _, err = conn.Exec(migration); err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(`INSERT INTO _migrations VALUES(?)`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = conn.Exec(`INSERT INTO offers(id,title,created_at,updated_at) VALUES(1,'Existing referral',1,1);
 INSERT INTO links(id,slug,url,offer_id,offer_homepage,created_at,updated_at) VALUES(1,'shared','https://example.com',1,1,1,1);
 INSERT INTO clicks(link_id,clicked_at) VALUES(1,1);
 INSERT INTO site_settings(id,site_name,featured_limit,updated_at) VALUES(1,'Existing identity',2,1)`); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	database, err := Init(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	offer, err := database.GetOfferByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if offer.ItemType != "referral" || offer.HomeSlug != "shared" || offer.Clicks != 1 {
		t.Fatal("catalog migration changed existing referral/history")
	}
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !settings.ItemPreviews || settings.LogoURL != "" || settings.FaviconURL != "" || settings.SiteName != "Existing identity" || settings.FeaturedLimit != 2 || settings.PublicAccent != "#c43424" || settings.AdminBackground != "#f4f1e9" {
		t.Fatal("catalog migration changed existing settings")
	}
	if _, err = database.Exec(`UPDATE offers SET item_type='unknown' WHERE id=1`); err == nil {
		t.Fatal("invalid item type accepted")
	}
}
