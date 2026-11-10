package plink

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

func TestNativeArticlePagePublishingAndSitemap(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	article, err := database.CreateArticle(db.ArticleInput{
		Slug:    "hello-world",
		Title:   "Hello World",
		Excerpt: "A short note",
		BodyMD:  "## Heading\n\nSome **bold** text and a [bad link](javascript:alert(1)).",
		Status:  "draft",
	})
	if err != nil {
		t.Fatal(err)
	}

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}

	if got := get("/a/hello-world"); got.Code != http.StatusNotFound {
		t.Fatalf("draft article = %d, want 404", got.Code)
	}
	if sitemap := get("/sitemap.xml").Body.String(); strings.Contains(sitemap, "/a/hello-world") {
		t.Fatal("draft article must not appear in the sitemap")
	}

	article.Status = "published"
	if err := database.UpdateArticle(article.ID, db.ArticleInput{
		Slug: article.Slug, Title: article.Title, Excerpt: article.Excerpt,
		BodyMD: article.BodyMD, Status: "published",
	}); err != nil {
		t.Fatal(err)
	}

	page := get("/a/hello-world")
	if page.Code != http.StatusOK {
		t.Fatalf("published article = %d: %s", page.Code, page.Body.String())
	}
	body := page.Body.String()
	for _, want := range []string{"Hello World", "<h2", "<strong>bold</strong>", `rel="canonical"`, "og:title", "A short note"} {
		if !strings.Contains(body, want) {
			t.Fatalf("article page missing %q", want)
		}
	}
	if strings.Contains(body, "javascript:") {
		t.Fatal("article page kept an active link scheme")
	}

	if sitemap := get("/sitemap.xml").Body.String(); !strings.Contains(sitemap, "/a/hello-world") {
		t.Fatal("published article is missing from the sitemap")
	}
}

func TestArticleAdminRequiresLogin(t *testing.T) {
	_, handler, _ := lifecycleTestServer(t)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/admin/articles", nil))
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/admin/login" {
		t.Fatalf("unauthenticated articles = %d, location %q", w.Code, w.Header().Get("Location"))
	}
}
