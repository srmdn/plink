package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBrandingSettingsAndFavicon(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	login := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {cfg.AdminPassword}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(login, req)
	cookies := login.Result().Cookies()
	form := url.Values{"site_name": {"Brand"}, "featured_limit": {"2"}, "public_accent": {"#123456"}, "logo_url": {"https://example.com/logo.png"}, "favicon_url": {"https://example.com/favicon.png"}}
	post := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
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
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	if w := post(); w.Code != http.StatusSeeOther {
		t.Fatalf("save branding: %d", w.Code)
	}
	settings, err := database.GetSiteSettings()
	if err != nil || settings.LogoURL != form.Get("logo_url") || settings.FaviconURL != form.Get("favicon_url") {
		t.Fatal("branding not saved")
	}
	if body := get("/").Body.String(); !strings.Contains(body, `src="https://example.com/logo.png"`) || !strings.Contains(body, `/favicon.svg?v=`) {
		t.Fatal("public branding not rendered")
	}
	favicon := get("/favicon.svg")
	if favicon.Code != 302 || favicon.Header().Get("Location") != form.Get("favicon_url") || favicon.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("custom favicon response incorrect")
	}
	for _, invalid := range []string{"javascript:alert(1)", "data:image/svg+xml,unsafe", "//example.com/a.png", "/favicon.svg", "/foo/../favicon.ico", "/a\\b.png"} {
		form.Set("logo_url", invalid)
		if w := post(); w.Code != 200 || !strings.Contains(w.Body.String(), "Logo, favicon, dan avatar harus") {
			t.Fatalf("accepted invalid asset %q", invalid)
		}
		saved, _ := database.GetSiteSettings()
		if saved.LogoURL != settings.LogoURL {
			t.Fatal("invalid asset modified saved branding")
		}
	}
	form.Del("logo_url")
	form.Del("favicon_url")
	if w := post(); w.Code != 303 {
		t.Fatal("legacy settings rejected")
	}
	saved, _ := database.GetSiteSettings()
	if saved.LogoURL != settings.LogoURL || saved.FaviconURL != settings.FaviconURL {
		t.Fatal("legacy form reset branding")
	}
	form.Set("logo_url", settings.LogoURL)
	form.Set("favicon_url", "")
	post()
	if get("/favicon.svg").Header().Get("Location") != settings.LogoURL {
		t.Fatal("favicon did not fall back to logo")
	}
	form.Set("logo_url", "")
	post()
	favicon = get("/favicon.svg")
	if favicon.Code != 200 || !strings.Contains(favicon.Header().Get("Content-Type"), "image/svg+xml") || !strings.Contains(favicon.Body.String(), `fill="#123456"`) {
		t.Fatal("default favicon ignores palette")
	}
	form.Set("favicon_url", "https://example.com/favicon.svg")
	post()
	favicon = get("https://example.com/favicon.svg")
	if favicon.Code != 200 {
		t.Fatal("self-referencing absolute favicon caused redirect loop")
	}
}
