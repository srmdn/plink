package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestContactSettingsMigrationDefaultsAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v17.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(`CREATE TABLE _migrations(version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range migrations[:17] {
		if _, err = conn.Exec(migration); err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(`INSERT INTO _migrations VALUES(?)`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = conn.Exec(`INSERT INTO site_settings(id, site_name, featured_limit, updated_at) VALUES(1,'Legacy',2,1)`); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	database, err := Init(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SiteName != "Legacy" || settings.FeaturedLimit != 2 {
		t.Fatal("v18 migration changed existing settings")
	}
	if settings.ContactTitle != "Hubungi saya" || !settings.ContactEnabled || settings.ContactLinks != "" {
		t.Fatalf("v18 migration did not apply contact defaults: %+v", settings)
	}

	settings.ContactLinks = `[{"platform":"whatsapp","url":"https://wa.me/628123"}]`
	settings.ContactTitle = "Chat saya"
	settings.ContactNote = "Balas cepat ya"
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	saved, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContactTitle != "Chat saya" || saved.ContactNote != "Balas cepat ya" || saved.ContactLinks == "" {
		t.Fatalf("contact settings not persisted: %+v", saved)
	}
}
