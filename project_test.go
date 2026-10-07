package plink

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

func TestProjectCatalogItem(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	if _, err := database.CreateOfferWithHomepageLink(db.OfferInput{
		Title: "Plink", ItemType: "project", Active: true, Description: "Self-hosted link shortener", Category: "Go",
	}, "plink-project", "https://github.com/srmdn/plink"); err != nil {
		t.Fatal(err)
	}

	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest("GET", "/", nil))
	body := home.Body.String()
	if !strings.Contains(body, `href="https://github.com/srmdn/plink"`) {
		t.Fatal("home project card does not link to the project URL")
	}
	if strings.Contains(body, `href="/plink-project"`) {
		t.Fatal("project card should not use the Plink slug")
	}
	if !strings.Contains(body, "Portofolio") || !strings.Contains(body, "Proyek") {
		t.Fatal("home project section missing")
	}
	if strings.Contains(body, `data-favorite-card="plink-project"`) || strings.Contains(body, `data-favorite="plink-project"`) {
		t.Fatal("project card should not offer a favorite action")
	}

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest("GET", "/offers?view=projects", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "Plink") {
		t.Fatalf("projects view missing the item: %d", list.Code)
	}

	catalog := httptest.NewRecorder()
	handler.ServeHTTP(catalog, httptest.NewRequest("GET", "/offers", nil))
	if strings.Contains(catalog.Body.String(), "Self-hosted link shortener") {
		t.Fatal("project should be excluded from the main catalog grid")
	}
	search := httptest.NewRecorder()
	handler.ServeHTTP(search, httptest.NewRequest("GET", "/offers?q=link+shortener", nil))
	if !strings.Contains(search.Body.String(), "Plink") {
		t.Fatal("project should still be found through search")
	}
}
