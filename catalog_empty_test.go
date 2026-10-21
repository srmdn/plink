package plink

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmptyCatalogHidesSearchAndFilters(t *testing.T) {
	_, handler, _ := lifecycleTestServer(t)
	get := func(path string) string {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		return response.Body.String()
	}

	empty := get("/offers?view=articles")
	if strings.Contains(empty, `class="store-search"`) || strings.Contains(empty, `class="catalog-filters"`) {
		t.Fatal("empty articles view should not render search or filters")
	}
	if !strings.Contains(empty, "Belum ada artikel yang ditambahkan.") {
		t.Fatal("empty articles view is missing its specific copy")
	}

	noMatch := get("/offers?q=no-such-item")
	if !strings.Contains(noMatch, `class="store-search"`) || !strings.Contains(noMatch, `class="catalog-filters"`) {
		t.Fatal("search with no results should keep search and filters")
	}
	if !strings.Contains(noMatch, "Coba kata lain atau ubah filternya.") {
		t.Fatal("search with no results is missing its copy")
	}
	if strings.Contains(noMatch, "buat lo") || !strings.Contains(noMatch, "untuk Anda") {
		t.Fatal("public affiliate fallback should use the formal register")
	}
}
