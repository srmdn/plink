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

func TestHomeOfferSectionTitleFollowsItemTypes(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	render := func() string {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		return w.Body.String()
	}

	if _, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Referral one", ItemType: "referral", Active: true}, "ref-one", "https://example.com/ref"); err != nil {
		t.Fatal(err)
	}
	referralsOnly := render()
	if !strings.Contains(referralsOnly, "<h2>Offers</h2>") {
		t.Fatal("a referral-only catalog should be titled Offers")
	}
	if !strings.Contains(referralsOnly, `<span class="offer-badge">Offer</span>`) {
		t.Fatal("referral cards should carry the public Offer badge")
	}

	if _, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Product one", ItemType: "product", Active: true}, "prod-one", "https://example.com/prod"); err != nil {
		t.Fatal(err)
	}
	if mixed := render(); !strings.Contains(mixed, "<h2>Products &amp; offers</h2>") {
		t.Fatal("a catalog with products and referrals should be titled Products & offers")
	}
}
