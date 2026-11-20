package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestSettingsCacheInvalidatedOnSave primes the cached settings, saves through
// the admin form, and expects every public read path to use the new values.
func TestSettingsCacheInvalidatedOnSave(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cookies := adminLogin(t, handler, cfg)
	get := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		return response
	}

	if body := get("/").Body.String(); !strings.Contains(body, "<title>"+cfg.SiteName+"</title>") {
		t.Fatal("initial public page does not use the configured name")
	}

	form := url.Values{
		"site_name":      {"Cached brand"},
		"featured_limit": {"3"},
		"public_accent":  {"#123456"},
		"logo_url":       {"https://example.com/logo.png"},
		"csrf_token":     {cookieNamed(cookies, "plink_csrf").Value},
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, adminRequest("POST", "/admin/settings", form, cookies))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save settings = %d, want 303", response.Code)
	}

	body := get("/").Body.String()
	if !strings.Contains(body, "<title>Cached brand</title>") {
		t.Fatal("public page still renders the pre-save site name")
	}
	if !strings.Contains(body, `src="https://example.com/logo.png"`) {
		t.Fatal("branding template func still renders the pre-save logo")
	}
	if theme := get("/js/theme.css").Body.String(); !strings.Contains(theme, "#123456") {
		t.Fatal("theme.css still renders the pre-save accent")
	}

	// A write that bypasses the settings handler must invalidate as well, so a
	// future writer cannot leave a stale render behind.
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.SiteName = "Direct write"
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	if body := get("/").Body.String(); !strings.Contains(body, "<title>Direct write</title>") {
		t.Fatal("direct database write left a stale settings cache")
	}
}
