package plink

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLinksListPagination covers the 25-per-page list: page one ends after 25
// rows, the page links carry the page number, and an out-of-range page lands
// on the last page instead of an empty list.
func TestLinksListPagination(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	for i := 1; i <= 30; i++ {
		if _, err := database.CreateLink(fmt.Sprintf("page-link-%02d", i), fmt.Sprintf("https://example.com/%d", i), "", "Tools"); err != nil {
			t.Fatal(err)
		}
	}
	cookies := loginAs(t, handler, cfg)

	get := func(target string) string {
		t.Helper()
		req := httptest.NewRequest("GET", target, nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", target, w.Code)
		}
		return w.Body.String()
	}

	first := get("/admin")
	if got := strings.Count(first, "slug-edit-trigger"); got != 25 {
		t.Fatalf("first page should render 25 link rows, got %d", got)
	}
	if !strings.Contains(first, "Page 1 of 2") {
		t.Fatal("first page should report its position")
	}
	if !strings.Contains(first, `href="/admin?page=2"`) {
		t.Fatal("first page should link to the second page")
	}

	second := get("/admin?page=2")
	if got := strings.Count(second, "slug-edit-trigger"); got != 5 {
		t.Fatalf("second page should render the remaining 5 rows, got %d", got)
	}
	if !strings.Contains(second, "Page 2 of 2") {
		t.Fatal("second page should report its position")
	}
	if !strings.Contains(second, `value="2"`) {
		t.Fatal("second page should keep its number for row actions")
	}

	clamped := get("/admin?page=9")
	if got := strings.Count(clamped, "slug-edit-trigger"); got != 5 {
		t.Fatalf("out-of-range page should clamp to the last page, got %d rows", got)
	}
}
