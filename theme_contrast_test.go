package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestLowContrastPaletteStillSaves locks the non-blocking contract: a palette
// below the WCAG AA ratio is stored as entered; the warning is advisory.
func TestLowContrastPaletteStillSaves(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cookies := adminLogin(t, handler, cfg)
	form := url.Values{
		"site_name":         {"Contrast brand"},
		"featured_limit":    {"3"},
		"public_accent":     {"#f4f1e9"},
		"public_background": {"#f4f1e9"},
		"admin_accent":      {"#ffffff"},
		"admin_background":  {"#ffffff"},
		"csrf_token":        {cookieNamed(cookies, "plink_csrf").Value},
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, adminRequest("POST", "/admin/settings", form, cookies))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("low contrast save = %d, want 303", response.Code)
	}
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.PublicAccent != "#f4f1e9" || settings.PublicBackground != "#f4f1e9" ||
		settings.AdminAccent != "#ffffff" || settings.AdminBackground != "#ffffff" {
		t.Fatalf("low contrast palette was not saved: %#v", settings)
	}
}
