package plink

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestThemeServesLightAndSystemDarkBlocks locks the P2b contract: the light
// block keeps its historical output, and a second block follows the visitor's
// system dark mode with the fixed palette while the accent stays operator-owned.
func TestThemeServesLightAndSystemDarkBlocks(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.PublicAccent, settings.PublicBackground = "#123456", "#f4f1e9"
	settings.AdminAccent, settings.AdminBackground = "#654321", "#faf8f4"
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	getTheme := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/css") {
			t.Fatalf("%s theme response invalid: %d", path, response.Code)
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s Cache-Control = %q", path, response.Header().Get("Cache-Control"))
		}
		return response
	}

	light := ":root{--accent:#123456;--bg:#f4f1e9;--paper:#f4f1e9}"
	darkValues := "--paper:#161a19;--bg:#161a19;--surface:#202623;--line:#39413d;--ink:#e9ece9;--muted:#9aa49e"
	dark := "@media (prefers-color-scheme:dark){:root{" + darkValues + ";--accent:#123456}}"
	for _, tc := range []struct {
		path, light, dark string
	}{
		{"/js/theme.css", light, dark},
		{"/js/theme.css?scope=admin", ":root{--accent:#654321;--bg:#faf8f4;--paper:#faf8f4}", "@media (prefers-color-scheme:dark){:root{" + darkValues + ";--accent:#654321}}"},
	} {
		body := getTheme(tc.path).Body.String()
		if body != tc.light+tc.dark {
			t.Fatalf("%s theme body = %q", tc.path, body)
		}
		// The system block must come after the light block so it can override it.
		if strings.Index(body, "@media") < strings.Index(body, ":root") {
			t.Fatalf("%s dark block precedes the light block", tc.path)
		}
	}
}
