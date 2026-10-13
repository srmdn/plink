package plink

import (
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestStaticAssetsVersioned(t *testing.T) {
	_, handler, _ := lifecycleTestServer(t)

	get := func(path string) string {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w.Body.String()
	}

	home := get("/")
	for _, want := range []string{`/js/css/storefront.css?v=`, `/js/storefront.js?v=`} {
		if !strings.Contains(home, want) {
			t.Fatalf("home page missing versioned asset %q", want)
		}
	}

	login := get("/admin/login")
	for _, want := range []string{`/js/css/admin.css?v=`, `/js/htmx.min.js?v=`} {
		if !strings.Contains(login, want) {
			t.Fatalf("admin page missing versioned asset %q", want)
		}
	}

	if !regexp.MustCompile(`storefront\.css\?v=[0-9a-f]{12}`).MatchString(home) {
		t.Fatal("asset version is not a 12-char hex hash")
	}
}
