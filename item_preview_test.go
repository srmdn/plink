package plink

import (
	"bytes"
	"image/png"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/srmdn/plink/internal/db"
)

func TestItemPreviewsInheritCatalogWithoutTrackingCrawlers(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cfg.PublicURL = "https://go.example.com"
	input := db.OfferInput{Title: "VPS <pilihan>", Description: "RAM & storage", Provider: "Provider", Active: true}
	offer, err := database.CreateOfferWithHomepageLink(input, "vps", "https://provider.example/?ref=one")
	if err != nil {
		t.Fatal(err)
	}
	channel, err := database.CreateOfferLink(offer.ID, "vps-threads", "https://provider.example/?ref=two", "threads", "")
	if err != nil {
		t.Fatal(err)
	}
	get := func(path, agent, method string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("User-Agent", agent)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, agent := range []string{"facebookexternalhit/1.1", "meta-externalfetcher/1.1", "Twitterbot/1.0", "WhatsApp/2.0", "Discordbot/2.0", "LinkedInBot/1.0", "Slackbot-LinkExpanding 1.0", "TelegramBot"} {
		for _, slug := range []string{"vps", "vps-threads"} {
			w := get("/"+slug+"?next=https://untrusted.example", agent, "GET")
			if w.Code != 200 || w.Header().Get("Location") != "" {
				t.Fatalf("%s %s: %d", agent, slug, w.Code)
			}
			for _, wanted := range []string{`property="og:title" content="VPS &lt;pilihan&gt;"`, `property="og:description" content="RAM &amp; storage"`, `property="og:url" content="https://go.example.com/` + slug + `"`, `/og-image.png?slug=` + slug, `content="noindex, follow"`} {
				if !strings.Contains(w.Body.String(), wanted) {
					t.Fatalf("missing %s in %s", wanted, w.Body.String())
				}
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Vary") != "User-Agent" {
				t.Fatal("unsafe cache")
			}
		}
	}
	var clicks, notices int
	database.QueryRow("SELECT COUNT(*) FROM clicks").Scan(&clicks)
	database.QueryRow("SELECT COUNT(*) FROM notice_views").Scan(&notices)
	if clicks != 0 || notices != 0 {
		t.Fatal("crawler tracked")
	}
	human := get("/vps-threads", "Mozilla/5.0", "GET")
	if human.Code != 302 || human.Header().Get("Location") != channel.URL {
		t.Fatal("human redirect changed")
	}
	deadline := time.Now().Add(time.Second)
	for {
		var tracked int
		database.QueryRow("SELECT COUNT(*) FROM clicks WHERE link_id=?", channel.ID).Scan(&tracked)
		if tracked == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("human click not tracked")
		}
		time.Sleep(10 * time.Millisecond)
	}
	first := get("/og-image.png?slug=vps", "", "GET")
	second := get("/og-image.png?slug=vps-threads", "", "GET")
	if first.Code != 200 || !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatal("channel images differ")
	}
	img, err := png.Decode(bytes.NewReader(first.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 {
		t.Fatal("invalid item image")
	}
	input.Title = "Updated VPS"
	input.ImageURL = "https://cdn.example/item.png"
	if err := database.UpdateOffer(offer.ID, input); err != nil {
		t.Fatal(err)
	}
	w := get("/vps-threads", "Twitterbot", "GET")
	if !strings.Contains(w.Body.String(), `property="og:image" content="https://cdn.example/item.png"`) || !strings.Contains(w.Body.String(), "Updated VPS") {
		t.Fatal("item edits not inherited")
	}
	if bytes.Equal(first.Body.Bytes(), get("/og-image.png?slug=vps", "", "GET").Body.Bytes()) {
		t.Fatal("stale generated image")
	}
	for _, agent := range []string{"Mozilla/5.0", "Googlebot/2.1", "Mozilla Instagram", "bingbot"} {
		w := get("/vps-threads", agent, "HEAD")
		if w.Code != 302 || w.Header().Get("Location") != channel.URL {
			t.Fatal("browser/search crawler redirect changed")
		}
	}
	for _, status := range []string{"ended", "paused", "upcoming", "expired"} {
		input.ProgramStatus = status
		input.StartsOn, input.EndsOn = "", ""
		switch status {
		case "upcoming":
			input.ProgramStatus = "active"
			input.StartsOn = time.Now().AddDate(0, 0, 2).Format("2006-01-02")
		case "expired":
			input.ProgramStatus = "active"
			input.EndsOn = time.Now().AddDate(0, 0, -2).Format("2006-01-02")
		}
		input.EndedBehavior = "redirect"
		input.FallbackURL = "https://provider.example/current"
		if err := database.UpdateOffer(offer.ID, input); err != nil {
			t.Fatal(err)
		}
		w := get("/vps", "Discordbot", "GET")
		if w.Code != 200 || strings.Contains(w.Body.String(), `property="og:image" content="https://cdn.example/item.png"`) || !strings.Contains(w.Body.String(), "Program ") {
			t.Fatal("stale lifecycle preview")
		}
	}
	database.QueryRow("SELECT COUNT(*) FROM notice_views").Scan(&notices)
	if notices != 0 {
		t.Fatal("status crawler counted")
	}
	database.Exec("UPDATE links SET active=0 WHERE slug='vps'")
	if get("/vps", "Twitterbot", "GET").Code != 404 || get("/og-image.png?slug=vps", "", "GET").Code != 404 {
		t.Fatal("disabled link exposed")
	}
	if get("/og-image.png?slug=missing", "", "GET").Code != 404 {
		t.Fatal("missing item image")
	}
}

func TestItemPreviewSettingsTogglePreservesLegacySaves(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	_, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Item", Active: true}, "item", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := database.GetSiteSettings()
	if !settings.ItemPreviews {
		t.Fatal("preview not default enabled")
	}
	login := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {cfg.AdminPassword}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(login, req)
	cookies := login.Result().Cookies()
	post := func(values url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			req.AddCookie(c)
			if c.Name == "plink_csrf" {
				req.Header.Set("X-CSRF-Token", c.Value)
			}
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	values := url.Values{"site_name": {"Test"}, "featured_limit": {"3"}, "item_previews_present": {"1"}}
	if w := post(values); w.Code != 303 {
		t.Fatalf("toggle save: %d %s", w.Code, w.Body.String())
	}
	settings, _ = database.GetSiteSettings()
	if settings.ItemPreviews {
		t.Fatal("toggle ignored")
	}
	delete(values, "item_previews_present")
	if post(values).Code != 303 {
		t.Fatal("legacy save failed")
	}
	settings, _ = database.GetSiteSettings()
	if settings.ItemPreviews {
		t.Fatal("legacy save reset preference")
	}
	req = httptest.NewRequest("GET", "/item", nil)
	req.Header.Set("User-Agent", "Twitterbot")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 302 {
		t.Fatal("disabled mode still previews")
	}
	var clicks int
	database.QueryRow("SELECT COUNT(*) FROM clicks").Scan(&clicks)
	if clicks != 0 {
		t.Fatal("disabled preview bot tracked")
	}
	values.Set("item_previews_present", "1")
	values.Set("item_previews", "1")
	if post(values).Code != 303 {
		t.Fatal("enable failed")
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal("enable ignored")
	}
}
