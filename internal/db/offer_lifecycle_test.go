package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLifecycleMigrationPreservesLegacyFallbackAndHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`CREATE TABLE _migrations (version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range migrations[:7] {
		if _, err := conn.Exec(migration); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(`INSERT INTO _migrations VALUES (?)`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(`INSERT INTO offers (title, fallback_url, created_at, updated_at) VALUES ('Legacy fallback', 'https://provider.example/current', 1, 1), ('Legacy no fallback', '', 1, 1);
		INSERT INTO links (slug, url, offer_id, offer_homepage, created_at, updated_at) VALUES ('legacy', 'https://provider.example/?ref=old', 1, 1, 1, 1);
		INSERT INTO clicks (link_id, clicked_at) VALUES (1, 1)`); err != nil {
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
	if offer.ProgramStatus != "active" || offer.EndedBehavior != "redirect" || offer.Clicks != 1 || offer.HomeSlug != "legacy" {
		t.Fatalf("legacy offer changed: %#v", offer)
	}
	other, err := database.GetOfferByID(2)
	if err != nil {
		t.Fatal(err)
	}
	if other.EndedBehavior != "notice" {
		t.Fatal("offer without fallback must default to notice")
	}
	if err := migrate(database.DB); err != nil {
		t.Fatalf("migration not repeatable: %v", err)
	}
}
