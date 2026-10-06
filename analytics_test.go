package plink

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
	"github.com/srmdn/plink/internal/server"
)

func analyticsTestHandler(t *testing.T, cfg *config.Config) http.Handler {
	t.Helper()
	database, err := db.Init(filepath.Join(t.TempDir(), "analytics.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return server.New(cfg, database, webFS)
}

func getPage(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestAnalyticsIsOptIn(t *testing.T) {
	base := &config.Config{AdminPath: "admin", AdminPassword: "test-password", Timezone: "Asia/Jakarta", Production: true}

	off := getPage(t, analyticsTestHandler(t, base), "/")
	if body := off.Body.String(); strings.Contains(body, "data-website-id") || strings.Contains(body, "analytics.example.com") {
		t.Fatal("default instance rendered a third-party analytics script")
	}
	if csp := off.Header().Get("Content-Security-Policy"); strings.Contains(csp, "connect-src") || !strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("unexpected default CSP: %s", csp)
	}

	on := &config.Config{
		AdminPath:          "admin",
		AdminPassword:      "test-password",
		Timezone:           "Asia/Jakarta",
		Production:         true,
		AnalyticsScriptURL: "https://analytics.example.com/script.js",
		AnalyticsWebsiteID: "site-123",
	}
	handler := analyticsTestHandler(t, on)
	for _, path := range []string{"/", "/offers", "/links"} {
		body := getPage(t, handler, path).Body.String()
		if !strings.Contains(body, `src="https://analytics.example.com/script.js"`) || !strings.Contains(body, `data-website-id="site-123"`) {
			t.Fatalf("configured analytics tag missing on %s", path)
		}
	}
	csp := getPage(t, handler, "/").Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline' https://analytics.example.com") {
		t.Fatalf("CSP did not allow the analytics script origin: %s", csp)
	}
	if !strings.Contains(csp, "connect-src 'self' https://analytics.example.com") {
		t.Fatalf("CSP did not allow the analytics connect origin: %s", csp)
	}

	admin := getPage(t, handler, "/admin/login").Body.String()
	if strings.Contains(admin, "data-website-id") {
		t.Fatal("admin pages should not load visitor analytics")
	}
}

func TestAnalyticsEscapesConfiguredValues(t *testing.T) {
	cfg := &config.Config{
		AdminPath:          "admin",
		AdminPassword:      "test-password",
		Timezone:           "Asia/Jakarta",
		AnalyticsScriptURL: `https://analytics.example.com/script.js`,
		AnalyticsWebsiteID: `"><script>alert(1)</script>`,
	}
	body := getPage(t, analyticsTestHandler(t, cfg), "/").Body.String()
	if strings.Contains(body, `<script>alert(1)</script>`) {
		t.Fatal("analytics website id was not escaped")
	}
}
