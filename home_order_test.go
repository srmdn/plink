package plink

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

func TestHomePresentsProjectsAndWritingBeforeRecommendations(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	for _, item := range []struct{ title, slug, kind string }{
		{"Project example", "example-project", "project"},
		{"Article example", "example-article", "article"},
		{"Referral example", "example-referral", "referral"},
	} {
		_, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: item.title, ItemType: item.kind, Active: true, Featured: true}, item.slug, "https://example.com/"+item.slug)
		if err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	body := w.Body.String()
	project, article, referral := strings.Index(body, "Project example"), strings.Index(body, "Article example"), strings.Index(body, "Referral example")
	if project < 0 || article < 0 || referral < 0 || project >= article || article >= referral {
		t.Fatalf("expected project then article then referral, got positions %d, %d, %d", project, article, referral)
	}
}
