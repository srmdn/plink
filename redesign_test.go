package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

func TestUnifiedCatalogServicesFiltersAndUnavailableFavorites(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	for _, input := range []db.OfferInput{
		{Title: "Setup VPS", ItemType: "service", Category: "Website", Active: true},
		{Title: "Hosting referral", ItemType: "referral", Category: "Website", Active: true},
		{Title: "Ended program", ItemType: "referral", Category: "Website", Active: true, ProgramStatus: "ended"},
	} {
		slug := strings.ReplaceAll(strings.ToLower(input.Title), " ", "-")
		if _, err := database.CreateOfferWithHomepageLink(input, slug, "https://example.com"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.CreateLink("resource-guide", "https://example.com/guide", "Useful resource", "Tools"); err != nil {
		t.Fatal(err)
	}
	get := func(path string) string {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 200 {
			t.Fatalf("%s: %d", path, r.Code)
		}
		return r.Body.String()
	}
	services := get("/offers?view=services")
	if !strings.Contains(services, `href="/setup-vps"`) || strings.Contains(services, `href="/hosting-referral"`) {
		t.Fatal("services catalog includes the wrong types")
	}
	search := get("/offers?q=Useful")
	if !strings.Contains(search, `href="/resource-guide"`) {
		t.Fatal("search omitted standalone resources")
	}
	scoped := get("/offers?category=Website&q=Useful")
	if strings.Contains(scoped, `href="/resource-guide"`) {
		t.Fatal("search escaped the selected category")
	}
	favorites := get("/offers?view=favorites")
	if !strings.Contains(favorites, `data-favorite-card="setup-vps"`) || strings.Contains(favorites, `data-favorite-card="ended-program"`) {
		t.Fatal("favorites contain unavailable items")
	}
	var clicks, notices int
	if err := database.QueryRow("SELECT COUNT(*) FROM clicks").Scan(&clicks); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow("SELECT COUNT(*) FROM notice_views").Scan(&notices); err != nil {
		t.Fatal(err)
	}
	if clicks != 0 || notices != 0 {
		t.Fatal("catalog browsing recorded redirect/notice analytics")
	}
}

func TestPaletteSettingsRejectInjectionAndKeepScopesSeparate(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	login := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {cfg.AdminPassword}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(login, request)
	cookies := login.Result().Cookies()
	form := url.Values{"site_name": {"Example"}, "featured_limit": {"2"}, "hero_enabled": {"1"}, "public_accent": {"#123456"}, "public_background": {"#ffffff"}, "admin_accent": {"#654321"}, "admin_background": {"#f4f1e9"}}
	post := func(csrf bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range cookies {
			r.AddCookie(cookie)
			if csrf && cookie.Name == "plink_csrf" {
				r.Header.Set("X-CSRF-Token", cookie.Value)
			}
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if response := post(false); response.Code != http.StatusForbidden {
		t.Fatal("palette write bypassed CSRF")
	}
	if response := post(true); response.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", response.Code, response.Body.String())
	}
	getTheme := func(path string) string {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/css") {
			t.Fatal("theme response invalid")
		}
		return w.Body.String()
	}
	if !strings.Contains(getTheme("/js/theme.css"), "--accent:#123456") || !strings.Contains(getTheme("/js/theme.css?scope=admin"), "--accent:#654321") {
		t.Fatal("theme scopes leaked")
	}
	form.Set("public_accent", "#123456;}body{display:none}")
	if response := post(true); response.Code != 200 || !strings.Contains(response.Body.String(), "Warna harus") {
		t.Fatal("invalid CSS accepted")
	}
	settings, err := database.GetSiteSettings()
	if err != nil || settings.PublicAccent != "#123456" {
		t.Fatal("invalid input changed saved settings")
	}
	form.Del("public_accent")
	form.Del("public_background")
	form.Del("admin_accent")
	form.Del("admin_background")
	if response := post(true); response.Code != http.StatusSeeOther {
		t.Fatal("legacy settings form rejected")
	}
	settings, _ = database.GetSiteSettings()
	if settings.PublicAccent != "#123456" || settings.AdminAccent != "#654321" {
		t.Fatal("legacy settings form reset custom palette")
	}
}
