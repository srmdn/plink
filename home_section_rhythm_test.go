package plink

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

// TestHomeSectionLeadAndList asserts the Projects section renders one lead
// card and keeps the remaining items as compact rows, instead of repeating the
// two-column card grid.
func TestHomeSectionLeadAndList(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	projects := []struct {
		title    string
		slug     string
		url      string
		priority int
	}{
		{"Curlog", "curlog", "https://curlog.example/", 3},
		{"Islami.click", "islami-click", "https://islami.example/", 2},
		{"Sermdn", "sermdn", "https://srmdn.example/", 1},
	}
	for _, project := range projects {
		if _, err := database.CreateOfferWithHomepageLink(db.OfferInput{
			Title: project.title, ItemType: "project", Active: true, Provider: "Said",
			Description: project.title + " notes", Priority: project.priority,
		}, project.slug, project.url); err != nil {
			t.Fatal(err)
		}
	}

	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest("GET", "/", nil))
	body := home.Body.String()

	if got := strings.Count(body, `<article class="lead-card`); got != 1 {
		t.Fatalf("home should render one lead card, got %d", got)
	}
	if got := strings.Count(body, `<article class="lead-row">`); got != 2 {
		t.Fatalf("home should render two compact rows, got %d", got)
	}

	// Priority order puts Curlog first, so it leads and the other two follow
	// as rows that keep their link and initials.
	lead := body[strings.Index(body, `<article class="lead-card`):strings.Index(body, `class="lead-rows"`)]
	if !strings.Contains(lead, "Curlog") || !strings.Contains(lead, "https://curlog.example/") {
		t.Fatal("the first project should be the lead card")
	}
	rows := body[strings.Index(body, `class="lead-rows"`):]
	if !strings.Contains(rows, "https://islami.example/") || !strings.Contains(rows, "https://srmdn.example/") {
		t.Fatal("remaining projects should keep their links in the rows")
	}
	if !strings.Contains(rows, ">IC<") {
		t.Fatal("a row thumbnail should show the title initials")
	}
	if strings.Contains(rows, `class="offer-card`) {
		t.Fatal("rows should not fall back to the grid card")
	}
}

// TestHomeSectionSingleItemHasNoRows keeps the section compact when there is
// only one item: the lead stands alone.
func TestHomeSectionSingleItemHasNoRows(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	if _, err := database.CreateOfferWithHomepageLink(db.OfferInput{
		Title: "Plink", ItemType: "project", Active: true,
	}, "plink-project", "https://github.com/srmdn/plink"); err != nil {
		t.Fatal(err)
	}

	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest("GET", "/", nil))
	body := home.Body.String()

	if got := strings.Count(body, `<article class="lead-card`); got != 1 {
		t.Fatalf("home should render one lead card, got %d", got)
	}
	if strings.Contains(body, `class="lead-rows"`) {
		t.Fatal("a single item should not render an empty row list")
	}
}

// TestHomeArticleLeadShowsDateAndReadTime covers the one line of context the
// article rows and lead use instead of the provider label.
func TestHomeArticleLeadShowsDateAndReadTime(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	if _, err := database.CreateArticle(db.ArticleInput{
		Slug:        "why-digital-freedom",
		Title:       "Why I built Digital Freedom",
		Excerpt:     "Owning the name, the file, and the link.",
		BodyMD:      strings.Repeat("word ", 400),
		Status:      "published",
		PublishedAt: "2026-11-10",
	}); err != nil {
		t.Fatal(err)
	}

	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest("GET", "/", nil))
	body := home.Body.String()

	if !strings.Contains(body, "Nov 10, 2026 · 2 min read") {
		t.Fatal("article lead should show the publication date and read time")
	}
	if !strings.Contains(body, `href="/a/why-digital-freedom"`) {
		t.Fatal("native article lead should link to the article page")
	}
}
