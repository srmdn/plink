package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestProductPageMigrationKeepsLegacyCardsAndClicks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v19.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`CREATE TABLE _migrations(version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range migrations[:19] {
		if _, err := conn.Exec(migration); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(`INSERT INTO _migrations VALUES(?)`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(`INSERT INTO offers(id,title,item_type,created_at,updated_at) VALUES(1,'Existing product','product',1,1);
		INSERT INTO links(id,slug,url,offer_id,offer_homepage,created_at,updated_at) VALUES(1,'existing-product','https://example.com/old',1,1,1,1);
		INSERT INTO clicks(link_id,clicked_at) VALUES(1,1)`); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	database, err := Init(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	product, err := database.GetOfferByID(1)
	if err != nil || product == nil || product.ProductPage || product.PriceMinor != nil || product.HomeSlug != "existing-product" || product.HomeURL != "https://example.com/old" || product.Clicks != 1 {
		t.Fatalf("legacy product changed: %#v, %v", product, err)
	}
}
