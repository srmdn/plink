package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestFeatureFlagsMigrationDefaultsOnAndPreserves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v16.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(`CREATE TABLE _migrations(version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range migrations[:16] {
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
		t.Fatal("v17 migration changed existing settings")
	}
	if !settings.StorefrontEnabled || !settings.CatalogEnabled || !settings.ServicesEnabled || !settings.ArticlesEnabled || !settings.ProjectsEnabled || !settings.ResourcesEnabled || !settings.ProfileEnabled || !settings.SupportEnabled || !settings.FavoritesEnabled {
		t.Fatal("v17 migration did not default public features on")
	}
	if settings.FeatureMode() != "full" {
		t.Fatalf("fresh install should be full mode, got %q", settings.FeatureMode())
	}

	settings.StorefrontEnabled = false
	settings.CatalogEnabled = false
	settings.ServicesEnabled = false
	settings.ArticlesEnabled = false
	settings.ProjectsEnabled = false
	settings.ResourcesEnabled = false
	settings.ProfileEnabled = false
	settings.SupportEnabled = false
	settings.FavoritesEnabled = false
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	saved, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if saved.StorefrontEnabled || saved.CatalogEnabled || saved.ProfileEnabled || saved.FeatureMode() != "shortener" {
		t.Fatalf("feature flags not persisted: %+v", saved)
	}
}
