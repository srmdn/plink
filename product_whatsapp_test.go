package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/srmdn/plink/internal/db"
)

func TestProductWhatsAppOrderingAndClickAccounting(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cfg.PublicURL = "https://shop.example"
	input := db.OfferInput{Title: "Buku & panduan", ItemType: "product", Active: true, Product: &db.ProductDetails{Enabled: true, Currency: "IDR", WhatsApp: &db.ProductWhatsApp{Enabled: true, Number: "628123456789", Message: "Halo, apakah tersedia?"}}}
	product, err := database.CreateOfferWithHomepageLink(input, "buku", "https://example.com/original")
	if err != nil {
		t.Fatal(err)
	}
	path := "/offers/item/" + strconv.FormatInt(product.ID, 10)
	request := func(method, target, agent string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		req.Header.Set("User-Agent", agent)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	page := request("GET", path, "browser")
	if page.Code != 200 || !strings.Contains(page.Body.String(), `href="`+path+`/whatsapp"`) || !strings.Contains(page.Body.String(), "Order via WhatsApp") {
		t.Fatal("product page does not expose the WhatsApp action")
	}
	for _, tc := range []struct{ method, agent string }{{"HEAD", "browser"}, {"GET", "WhatsApp/2.0"}, {"GET", "browser"}} {
		response := request(tc.method, path+"/whatsapp", tc.agent)
		destination, err := url.Parse(response.Header().Get("Location"))
		if response.Code != http.StatusFound || err != nil || destination.Scheme != "https" || destination.Host != "wa.me" || destination.Path != "/628123456789" {
			t.Fatalf("WhatsApp redirect: %d %s", response.Code, response.Header().Get("Location"))
		}
		message := destination.Query().Get("text")
		if !strings.Contains(message, "Halo, apakah tersedia?") || !strings.Contains(message, "Buku & panduan") || !strings.Contains(message, "https://shop.example"+path) {
			t.Fatalf("WhatsApp draft = %q", message)
		}
		if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Robots-Tag") != "noindex" {
			t.Fatal("redirect must not be cached or indexed")
		}
	}
	updated, err := database.GetOfferByID(product.ID)
	if err != nil || updated.WhatsAppClicks != 1 || updated.Clicks != 0 || updated.HomeURL != "https://example.com/original" {
		t.Fatalf("click accounting or old destination changed: %#v %v", updated, err)
	}
	if got := request("HEAD", "/buku", "browser").Header().Get("Location"); got != "https://example.com/original" {
		t.Fatalf("original slug destination changed: %q", got)
	}
	input.Product.WhatsApp.Enabled = false
	if err := database.UpdateOffer(product.ID, input); err != nil {
		t.Fatal(err)
	}
	if got := request("GET", path+"/whatsapp", "browser").Code; got != http.StatusNotFound {
		t.Fatalf("disabled WhatsApp action = %d", got)
	}
	updated, err = database.GetOfferByID(product.ID)
	if err != nil || updated.WhatsAppClicks != 1 {
		t.Fatal("disabling WhatsApp erased click history")
	}
	input.Product.WhatsApp.Enabled = true
	input.Active = false
	if err := database.UpdateOffer(product.ID, input); err != nil {
		t.Fatal(err)
	}
	if got := request("GET", path+"/whatsapp", "browser").Code; got != http.StatusNotFound {
		t.Fatalf("hidden product WhatsApp action = %d", got)
	}
	input.Active, input.ProgramStatus = true, "ended"
	if err := database.UpdateOffer(product.ID, input); err != nil {
		t.Fatal(err)
	}
	if got := request("GET", path+"/whatsapp", "browser").Code; got != http.StatusNotFound {
		t.Fatalf("ended product WhatsApp action = %d", got)
	}
}

func TestWhatsAppStatisticsUsesPeriodAndCategory(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	first, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Digital", Category: "Digital", ItemType: "product", Active: true}, "digital", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	second, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Physical", Category: "Physical", ItemType: "product", Active: true}, "physical", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	for _, click := range []struct{ id, at int64 }{{first.ID, now}, {first.ID, now - 40*86400}, {second.ID, now}} {
		if err := database.RecordProductWhatsAppClick(click.id, click.at); err != nil {
			t.Fatal(err)
		}
	}
	start, end := now-7*86400, now+1
	total, err := database.CountProductWhatsAppClicks(&start, &end, "Digital")
	if err != nil || total != 1 {
		t.Fatalf("filtered WhatsApp clicks = %d, %v", total, err)
	}
	total, err = database.CountProductWhatsAppClicks(nil, nil, "")
	if err != nil || total != 3 {
		t.Fatalf("all-time WhatsApp clicks = %d, %v", total, err)
	}
	req := httptest.NewRequest("GET", "/admin/analytics/dashboard?period=7d&category=Digital", nil)
	for _, cookie := range loginAs(t, handler, cfg) {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Klik WhatsApp") || !strings.Contains(w.Body.String(), `class="stat-value">1</div><div class="stat-card-foot">Tombol produk`) {
		t.Fatal("statistics page does not show the filtered WhatsApp total")
	}
}

func TestProductWhatsAppPublicFeatureSwitchesPreserveSettings(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	input := db.OfferInput{Title: "Produk", ItemType: "product", Active: true, Product: &db.ProductDetails{Enabled: true, Currency: "IDR", WhatsApp: &db.ProductWhatsApp{Enabled: true, Number: "628123456789", Message: "Halo"}}}
	product, err := database.CreateOfferWithHomepageLink(input, "produk-switch", "https://example.com/original")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	path := "/offers/item/" + strconv.FormatInt(product.ID, 10)
	request := func(target string) int {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("HEAD", target, nil))
		return w.Code
	}
	for _, tc := range []struct {
		name       string
		storefront bool
		catalog    bool
		want       int
	}{
		{"both enabled", true, true, http.StatusOK},
		{"catalog disabled", true, false, http.StatusNotFound},
		{"storefront disabled", false, true, http.StatusNotFound},
		{"both restored", true, true, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings.StorefrontEnabled, settings.CatalogEnabled = tc.storefront, tc.catalog
			if err := database.SaveSiteSettings(settings); err != nil {
				t.Fatal(err)
			}
			if got := request(path); got != tc.want {
				t.Fatalf("product page = %d, want %d", got, tc.want)
			}
			wantWhatsApp := http.StatusNotFound
			if tc.want == http.StatusOK {
				wantWhatsApp = http.StatusFound
			}
			if got := request(path + "/whatsapp"); got != wantWhatsApp {
				t.Fatalf("WhatsApp action = %d, want %d", got, wantWhatsApp)
			}
			if got := request("/produk-switch"); got != http.StatusFound {
				t.Fatalf("short link = %d, want redirect", got)
			}
		})
	}
	stored, err := database.GetOfferByID(product.ID)
	if err != nil || stored == nil || !stored.ProductPage || !stored.WhatsAppEnabled || stored.WhatsAppNumber != "628123456789" || stored.WhatsAppMessage != "Halo" || stored.HomeURL != "https://example.com/original" {
		t.Fatalf("feature switches changed product settings: %#v, %v", stored, err)
	}
}

func TestProductWhatsAppAdminCreationAndValidation(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cookies := loginAs(t, handler, cfg)
	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
			if cookie.Name == "plink_csrf" {
				req.Header.Set("X-CSRF-Token", cookie.Value)
			}
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	form := url.Values{"title": {"Produk uji"}, "item_type": {"product"}, "home_slug": {"produk-uji"}, "active": {"1"}, "product_fields_present": {"1"}, "product_page": {"1"}, "whatsapp_fields_present": {"1"}, "whatsapp_enabled": {"1"}, "whatsapp_number": {"08123456789"}, "whatsapp_message": {"Halo & salam"}}
	bad := post("/admin/offers", form)
	if bad.Code != 200 || !strings.Contains(bad.Body.String(), "kode negara") || !strings.Contains(bad.Body.String(), "08123456789") || !strings.Contains(bad.Body.String(), "Halo &amp; salam") {
		t.Fatal("invalid number should explain the error and preserve input")
	}
	form.Set("whatsapp_number", "+62 (812) 3456-789")
	form.Del("product_page")
	if result := post("/admin/offers", form); result.Code != 200 || !strings.Contains(result.Body.String(), "aktifkan halaman detail") {
		t.Fatal("WhatsApp should require an enabled product detail page")
	}
	form.Set("product_page", "1")
	if result := post("/admin/offers", form); result.Code != http.StatusSeeOther {
		t.Fatalf("create = %d: %s", result.Code, result.Body.String())
	}
	link, err := database.GetLinkBySlug("produk-uji")
	if err != nil || link == nil || link.URL != "https://wa.me/628123456789" {
		t.Fatalf("automatic short-link destination: %#v %v", link, err)
	}
	product, err := database.GetOfferByID(link.OfferID)
	if err != nil || !product.WhatsAppEnabled || product.WhatsAppNumber != "628123456789" || product.WhatsAppMessage != "Halo & salam" {
		t.Fatalf("saved WhatsApp settings = %#v %v", product, err)
	}
	// A previously opened product editor does not have the new WhatsApp fields.
	legacy := url.Values{"title": {"Produk uji"}, "item_type": {"product"}, "active": {"1"}, "product_fields_present": {"1"}, "product_page": {"1"}, "product_currency": {"IDR"}}
	if result := post("/admin/offers/"+strconv.FormatInt(product.ID, 10), legacy); result.Code != http.StatusSeeOther {
		t.Fatalf("legacy update = %d", result.Code)
	}
	product, err = database.GetOfferByID(product.ID)
	if err != nil || !product.WhatsAppEnabled || product.WhatsAppNumber != "628123456789" || product.WhatsAppMessage != "Halo & salam" {
		t.Fatal("legacy product editor erased WhatsApp settings")
	}
}
