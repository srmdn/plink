package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestUnlistedMigrationPreservesExistingLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v18.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`CREATE TABLE _migrations(version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range migrations[:18] {
		if _, err := conn.Exec(migration); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(`INSERT INTO _migrations VALUES(?)`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(`INSERT INTO links(id,slug,url,featured,created_at,updated_at) VALUES(1,'existing','https://example.com',1,1,1); INSERT INTO clicks(link_id,clicked_at) VALUES(1,1)`); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	database, err := Init(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	link, err := database.GetLinkByID(1)
	if err != nil || link == nil || link.Unlisted || link.Clicks != 1 || !link.Featured {
		t.Fatalf("existing link changed: %#v, %v", link, err)
	}
	links, err := database.ListPublicLinks()
	if err != nil || len(links) != 1 || links[0].Slug != "existing" {
		t.Fatalf("existing link no longer public: %#v, %v", links, err)
	}
}
