package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPublicFeatureToggles(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)

	// Shortener mode: everything public off, redirects stay core.
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.SiteName = "Solo Link"
	settings.StorefrontEnabled = false
	settings.CatalogEnabled = false
	settings.ServicesEnabled = false
	settings.ArticlesEnabled = false
	settings.ProjectsEnabled = false
	settings.ResourcesEnabled = false
	settings.ProfileEnabled = false
	settings.SupportEnabled = false
	settings.FavoritesEnabled = false
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	if root := get("/"); root.Code != http.StatusOK {
		t.Fatalf("shortener root status = %d", root.Code)
	} else if strings.Contains(root.Body.String(), "Produk &amp; referral") {
		t.Fatal("catalog rendered in shortener mode")
	}
	for _, path := range []string{"/offers", "/links", "/offers?view=services", "/offers?view=projects", "/offers?view=articles", "/offers?view=favorites"} {
		if code := get(path).Code; code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404 in shortener mode", path, code)
		}
	}
	if sitemap := get("/sitemap.xml").Body.String(); strings.Contains(sitemap, "view=") || strings.Contains(sitemap, "/links") {
		t.Fatal("sitemap exposes disabled features in shortener mode")
	}

	// Admin form: preset modes drive the flags.
	login := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {cfg.AdminPassword}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(login, request)
	cookies := login.Result().Cookies()
	postSettings := func(values url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			r.AddCookie(c)
			if c.Name == "plink_csrf" {
				r.Header.Set("X-CSRF-Token", c.Value)
			}
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	form := url.Values{"site_name": {"Solo Link"}, "featured_limit": {"3"}, "feature_mode": {"biolink"}}
	if w := postSettings(form); w.Code != http.StatusSeeOther {
		t.Fatalf("biolink save status = %d: %s", w.Code, w.Body.String())
	}
	saved, _ := database.GetSiteSettings()
	if !saved.StorefrontEnabled || !saved.ProfileEnabled || !saved.SupportEnabled || !saved.ResourcesEnabled {
		t.Fatalf("biolink preset did not enable profile links: %+v", saved)
	}
	if saved.CatalogEnabled || saved.ServicesEnabled || saved.ArticlesEnabled || saved.ProjectsEnabled || saved.FavoritesEnabled {
		t.Fatalf("biolink preset left catalog features on: %+v", saved)
	}
	if get("/offers").Code != http.StatusNotFound {
		t.Fatal("catalog reachable in biolink mode")
	}
	if get("/").Code != http.StatusOK {
		t.Fatal("biolink home not reachable")
	}

	form.Set("feature_mode", "full")
	if w := postSettings(form); w.Code != http.StatusSeeOther {
		t.Fatalf("full save status = %d", w.Code)
	}
	if get("/offers").Code != http.StatusOK {
		t.Fatal("catalog not restored in full mode")
	}

	// A legacy submission without feature_mode preserves the saved flags.
	form.Del("feature_mode")
	if w := postSettings(form); w.Code != http.StatusSeeOther {
		t.Fatalf("legacy save status = %d", w.Code)
	}
	after, _ := database.GetSiteSettings()
	if !after.StorefrontEnabled || !after.CatalogEnabled || !after.ProfileEnabled {
		t.Fatalf("legacy save cleared feature flags: %+v", after)
	}
}
