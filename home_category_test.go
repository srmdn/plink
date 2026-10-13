package plink

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

func TestHomeCategoryChipsExcludeArticleAndProject(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)

	offers := []struct {
		title, itemType, category, slug, url string
	}{
		{"Produk", "referral", "VPS", "produk", "https://example.com/p"},
		{"Tulisan", "article", "Refleksi", "tulisan", "https://example.com/a"},
		{"Proyek", "project", "Arsip", "proyek", "https://example.com/pr"},
	}
	for _, o := range offers {
		if _, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: o.title, ItemType: o.itemType, Category: o.category, Active: true}, o.slug, o.url); err != nil {
			t.Fatal(err)
		}
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	body := w.Body.String()

	chips := ""
	if i := strings.Index(body, `class="store-categories"`); i >= 0 {
		if j := strings.Index(body[i:], "</nav>"); j >= 0 {
			chips = body[i : i+j]
		}
	}
	if chips == "" {
		t.Fatal("no category chips rendered on the home page")
	}
	if !strings.Contains(chips, ">VPS<") {
		t.Fatal("product category chip missing from the home page")
	}
	for _, hidden := range []string{"Refleksi", "Arsip"} {
		if strings.Contains(chips, hidden) {
			t.Fatalf("article/project category %q should not appear as a home chip", hidden)
		}
	}
}
