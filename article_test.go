package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

func TestArticleCatalogItem(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	if _, err := database.CreateOfferWithHomepageLink(db.OfferInput{
		Title: "Cara pilih VPS", ItemType: "article", Active: true, Description: "Panduan singkat",
	}, "cara-pilih-vps", "https://blog.example/cara-pilih-vps"); err != nil {
		t.Fatal(err)
	}

	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest("GET", "/", nil))
	body := home.Body.String()
	if !strings.Contains(body, `href="https://blog.example/cara-pilih-vps"`) {
		t.Fatal("home article card does not link to the article URL")
	}
	if strings.Contains(body, `href="/cara-pilih-vps"`) {
		t.Fatal("article card should not use the Plink slug")
	}
	if !strings.Contains(body, "Artikel") {
		t.Fatal("home article section missing")
	}

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest("GET", "/offers?view=articles", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "Cara pilih VPS") {
		t.Fatalf("articles view missing the item: %d", list.Code)
	}
}

func TestSettingsProfileAndSocialLinks(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	login := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {cfg.AdminPassword}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(login, req)
	cookies := login.Result().Cookies()

	form := url.Values{
		"site_name":        {"Brand"},
		"featured_limit":   {"2"},
		"avatar_url":       {"/media/avatar.png"},
		"social_instagram": {"https://instagram.com/said"},
		"social_email":     {"me@example.com"},
	}
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
	if w := post(); w.Code != http.StatusSeeOther {
		t.Fatalf("save profile: %d %s", w.Code, w.Body.String())
	}
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.AvatarURL != "/media/avatar.png" {
		t.Fatalf("avatar not saved: %q", settings.AvatarURL)
	}
	if !strings.Contains(settings.SocialLinks, "instagram") || !strings.Contains(settings.SocialLinks, "mailto:me@example.com") {
		t.Fatalf("social links not saved: %q", settings.SocialLinks)
	}

	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest("GET", "/", nil))
	body := home.Body.String()
	if !strings.Contains(body, `src="/media/avatar.png"`) {
		t.Fatal("avatar not rendered in the public header")
	}
	if !strings.Contains(body, `href="https://instagram.com/said"`) || !strings.Contains(body, `href="mailto:me@example.com"`) {
		t.Fatal("social links not rendered in the public header")
	}

	form.Set("social_instagram", "javascript:alert(1)")
	if w := post(); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Tautan sosial tidak valid") {
		t.Fatalf("invalid social accepted: %d", w.Code)
	}
	saved, _ := database.GetSiteSettings()
	if saved.SocialLinks != settings.SocialLinks {
		t.Fatal("invalid social link modified saved values")
	}
}
