package plink

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMediaLibraryRouteRequiresAuth checks that the media page is registered
// behind the admin session, not on the public catch-all.
func TestMediaLibraryRouteRequiresAuth(t *testing.T) {
	_, handler, cfg := lifecycleTestServer(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/"+cfg.AdminPath+"/media", nil))
	if response.Code != http.StatusFound {
		t.Fatalf("unauthenticated media page = %d, want 302", response.Code)
	}
	if location := response.Header().Get("Location"); location != "/"+cfg.AdminPath+"/login" {
		t.Fatalf("redirect = %q", location)
	}

	cookies := adminLogin(t, handler, cfg)
	page := adminGet(t, handler, cookies, "/"+cfg.AdminPath+"/media")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Media library") {
		t.Fatalf("media page = %d, body = %s", page.Code, page.Body.String())
	}
}
