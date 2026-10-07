package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestProjectItemTypeMigrationPreservesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v15.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(`CREATE TABLE _migrations(version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range migrations[:15] {
		if _, err = conn.Exec(migration); err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(`INSERT INTO _migrations VALUES(?)`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = conn.Exec(`INSERT INTO offers(id,title,item_type,created_at,updated_at) VALUES(1,'Existing article','article',1,1);
 INSERT INTO links(id,slug,url,offer_id,offer_homepage,created_at,updated_at) VALUES(1,'tulisan','https://example.com',1,1,1,1);
 INSERT INTO clicks(link_id,clicked_at) VALUES(1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(`INSERT INTO offers(id,title,item_type,created_at,updated_at) VALUES(2,'Too early','project',1,1)`); err == nil {
		t.Fatal("v15 schema unexpectedly accepted the project item type")
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
	if offer.ItemType != "article" || offer.HomeSlug != "tulisan" || offer.Clicks != 1 {
		t.Fatal("v16 migration changed the existing article or its history")
	}

	project, err := database.CreateOfferWithHomepageLink(OfferInput{Title: "Portfolio", ItemType: "project", Active: true}, "portfolio", "https://example.com/project")
	if err != nil {
		t.Fatalf("create project offer: %v", err)
	}
	if project.ItemType != "project" || project.HomeURL != "https://example.com/project" {
		t.Fatalf("project offer not stored: %+v", project)
	}
	if _, err = database.Exec(`UPDATE offers SET item_type='unknown' WHERE id=1`); err == nil {
		t.Fatal("invalid item type accepted after v16")
	}
}
