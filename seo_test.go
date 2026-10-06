package plink

import (
	"bytes"
	"encoding/xml"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPublicSEOAndShareImage(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cfg.PublicURL = "https://go.example.com"
	settings, _ := database.GetSiteSettings()
	settings.SiteName = "Said & tools"
	settings.SiteDesc = "Pilihan produk dan jasa"
	settings.SEOTitle = "Pilihan <Said>"
	settings.SEODescription = "Tools & jasa untuk website"
	settings.PublicAccent = "#ABCDEF"
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	home := get("/")
	for _, expected := range []string{`<title>Pilihan &lt;Said&gt;</title>`, `name="description" content="Tools &amp; jasa untuk website"`, `property="og:title"`, `property="og:image"`, `name="twitter:card" content="summary_large_image"`, cfg.PublicURL + "/og-image.png?", `rel="canonical"`} {
		if !strings.Contains(home.Body.String(), expected) {
			t.Fatalf("missing metadata %s", expected)
		}
	}
	for _, path := range []string{"/offers?view=favorites", "/offers?q=website", "/offers?category=AI"} {
		if !strings.Contains(get(path).Body.String(), `content="noindex, follow"`) {
			t.Fatalf("indexable private/filter page %s", path)
		}
	}
	services := get("/offers?view=services")
	if !strings.Contains(services.Body.String(), `Jasa · Said &amp; tools`) || strings.Contains(services.Body.String(), `content="noindex, follow"`) {
		t.Fatal("service SEO incorrect")
	}
	if !strings.Contains(get("/links").Body.String(), `href="`+cfg.PublicURL+`/links"`) {
		t.Fatal("resource canonical incorrect")
	}
	image := get("/og-image.png?view=services")
	if image.Code != 200 || image.Header().Get("Content-Type") != "image/png" {
		t.Fatal("OG image response incorrect")
	}
	if cached := get("/og-image.png?view=services&arbitrary=value"); !bytes.Equal(cached.Body.Bytes(), image.Body.Bytes()) {
		t.Fatal("same OG variant changed with irrelevant query")
	}
	decoded, err := png.Decode(image.Body)
	if err != nil || decoded.Bounds().Dx() != 1200 || decoded.Bounds().Dy() != 630 {
		t.Fatal("invalid OG PNG")
	}
	if strings.Contains(get("/robots.txt").Body.String(), "Disallow: /\n") || !strings.Contains(get("/robots.txt").Body.String(), "Disallow: /admin") {
		t.Fatal("robots incorrect")
	}
	sitemap := get("/sitemap.xml")
	var parsed struct {
		URLs []struct {
			Location string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(sitemap.Body.Bytes(), &parsed); err != nil || len(parsed.URLs) != 4 {
		t.Fatal("invalid sitemap")
	}
	for _, entry := range parsed.URLs {
		if strings.Contains(entry.Location, "admin") || strings.Contains(entry.Location, "favorites") {
			t.Fatal("private URL in sitemap")
		}
	}
	if get("/admin/login").Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatal("admin may be indexed")
	}
	settings.ShareImageURL = "/js/custom.png"
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(get("/").Body.String(), cfg.PublicURL+"/js/custom.png") {
		t.Fatal("custom image not absolute")
	}
}

func TestSEOSettingsPreserveAndValidate(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	login := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {cfg.AdminPassword}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(login, request)
	cookies := login.Result().Cookies()
	post := func(values url.Values) *httptest.ResponseRecorder {
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
	form := url.Values{"site_name": {"Said"}, "featured_limit": {"3"}, "seo_title": {"Pilihan Said"}, "seo_description": {"Produk dan jasa"}, "share_image_url": {"https://example.com/share.png"}}
	if post(form).Code != http.StatusSeeOther {
		t.Fatal("SEO save failed")
	}
	saved, _ := database.GetSiteSettings()
	if saved.SEOTitle != form.Get("seo_title") || saved.ShareImageURL != form.Get("share_image_url") {
		t.Fatal("SEO not persisted")
	}
	for _, bad := range []string{"javascript:alert(1)", "//example.com/share.png", "http://example.com/share.png", "/og-image.png"} {
		form.Set("share_image_url", bad)
		if post(form).Code != 200 {
			t.Fatal("expected validation page")
		}
		after, _ := database.GetSiteSettings()
		if after.ShareImageURL != saved.ShareImageURL {
			t.Fatal("invalid image saved")
		}
	}
	form.Set("public_origin", "https://example.com/path")
	if post(form).Code != 200 {
		t.Fatal("invalid origin not rejected")
	}
	invalid, _ := database.GetSiteSettings()
	if invalid.PublicOrigin != "" {
		t.Fatal("invalid origin saved")
	}
	form.Set("share_image_url", saved.ShareImageURL)
	form.Set("public_origin", "https://NEW.example.com/")
	if post(form).Code != 303 {
		t.Fatal("valid origin rejected")
	}
	normalized, _ := database.GetSiteSettings()
	if normalized.PublicOrigin != "https://new.example.com" {
		t.Fatal("origin not normalized")
	}
	form.Del("public_origin")
	form.Del("seo_title")
	form.Del("seo_description")
	form.Del("share_image_url")
	if post(form).Code != 303 {
		t.Fatal("legacy save failed")
	}
	after, _ := database.GetSiteSettings()
	if after.SEOTitle != saved.SEOTitle || after.ShareImageURL != saved.ShareImageURL {
		t.Fatal("legacy save cleared SEO")
	}
	form.Set("share_image_url", "")
	form.Set("seo_title", "")
	form.Set("seo_description", "")
	if post(form).Code != 303 {
		t.Fatal("cannot reset SEO")
	}
}

func TestSettingsDrivePublicCopyDomainAndCanonical(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cfg.PublicURL = "https://old.example.com"
	settings, _ := database.GetSiteSettings()
	settings.SiteName = "Brand"
	settings.PublicOrigin = "https://new.example.com"
	settings.HeroEyebrow = "Pilihan tim"
	settings.HeroTitle = "Temuan tim\nUntuk kebutuhanmu"
	settings.HeroDescription = "Produk pilihan tim"
	settings.ServiceTitle = "Layanan kami"
	settings.ServiceDescription = "Bantuan aplikasi"
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	home := get("/").Body.String()
	for _, expected := range []string{settings.HeroTitle, settings.HeroEyebrow, settings.HeroDescription, settings.ServiceTitle, "https://new.example.com/og-image.png"} {
		if !strings.Contains(home, expected) {
			t.Fatalf("missing editable content %q", expected)
		}
	}
	services := get("/offers?view=services").Body.String()
	if !strings.Contains(services, "Layanan kami") || !strings.Contains(services, "Bantuan aplikasi") {
		t.Fatal("service copy not applied")
	}
	if strings.Contains(get("/offers?page=999").Body.String(), `/offers?page=999`) {
		t.Fatal("unclamped canonical")
	}
	if !strings.Contains(get("/sitemap.xml").Body.String(), "https://new.example.com/") {
		t.Fatal("sitemap ignores saved origin")
	}
	if !strings.Contains(get("/robots.txt").Body.String(), "https://new.example.com/sitemap.xml") {
		t.Fatal("robots ignores saved origin")
	}
	settings.PublicOrigin = ""
	database.SaveSiteSettings(settings)
	if !strings.Contains(get("/").Body.String(), "https://old.example.com/og-image.png") {
		t.Fatal("environment fallback lost")
	}
}
