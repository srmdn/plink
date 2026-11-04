package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

func TestProductPageOptInAndLifecycle(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	old, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Old product", ItemType: "product", Active: true}, "old-product", "https://example.com/old")
	if err != nil {
		t.Fatal(err)
	}
	price := int64(7900000)
	input := db.OfferInput{Title: "Digital guide", Description: "Short introduction", ItemType: "product", Active: true, ButtonLabel: "Pesan", Product: &db.ProductDetails{Enabled: true, Kind: "digital", PriceMinor: &price, Currency: "IDR", Details: "A guide <script>alert(1)</script>", Fulfillment: "Delivered after confirmation"}}
	product, err := database.CreateOfferWithHomepageLink(input, "digital-guide", "https://example.com/order")
	if err != nil {
		t.Fatal(err)
	}
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	pageURL := "/offers/item/" + strconv.FormatInt(product.ID, 10)
	catalog := get("/offers").Body.String()
	if !strings.Contains(catalog, `href="`+pageURL+`"`) || !strings.Contains(catalog, `href="/old-product"`) || !strings.Contains(catalog, "Rp79.000") {
		t.Fatal("product cards do not respect the opted-in detail page and legacy redirect")
	}
	page := get(pageURL)
	for _, value := range []string{"Digital guide", "Rp79.000", "Delivered after confirmation", `href="/digital-guide"`, "&lt;script&gt;"} {
		if page.Code != 200 || !strings.Contains(page.Body.String(), value) {
			t.Fatalf("product page missing %q: %d", value, page.Code)
		}
	}
	if strings.Contains(page.Body.String(), "<script>alert(1)</script>") {
		t.Fatal("product text was not escaped")
	}
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM clicks").Scan(&count); err != nil || count != 0 {
		t.Fatalf("browsing recorded clicks: %d, %v", count, err)
	}
	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest("HEAD", "/digital-guide", nil))
	if head.Code != http.StatusFound || head.Header().Get("Location") != "https://example.com/order" {
		t.Fatal("existing short slug no longer redirects to its original destination")
	}
	if got := get("/offers/item/" + strconv.FormatInt(old.ID, 10)).Code; got != 404 {
		t.Fatalf("legacy product detail = %d", got)
	}
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.CatalogEnabled = false
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	if got := get(pageURL).Code; got != 404 {
		t.Fatalf("detail with catalog disabled = %d", got)
	}
	settings.CatalogEnabled = true
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	input.Active = false
	if err := database.UpdateOffer(product.ID, input); err != nil {
		t.Fatal(err)
	}
	if got := get(pageURL).Code; got != 404 {
		t.Fatalf("hidden product detail = %d", got)
	}
	input.Active, input.ProgramStatus = true, "ended"
	if err := database.UpdateOffer(product.ID, input); err != nil {
		t.Fatal(err)
	}
	if got := get(pageURL).Code; got != 404 {
		t.Fatalf("ended product detail = %d", got)
	}
}

func TestProductAdminValidationAndLegacyUpdate(t *testing.T) {
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
	form := url.Values{"title": {"Local product"}, "item_type": {"product"}, "home_slug": {"local-product"}, "home_url": {"https://example.com/order"}, "active": {"1"}, "product_fields_present": {"1"}, "product_page": {"1"}, "product_kind": {"physical"}, "product_currency": {"IDR"}, "product_price": {"12,50"}, "product_details": {"Useful details"}, "product_fulfillment": {"Delivery by arrangement"}}
	form.Set("product_price", "abc")
	bad := post("/admin/offers", form)
	if bad.Code != 200 || !strings.Contains(bad.Body.String(), `value="abc"`) || !strings.Contains(bad.Body.String(), "harga harus") {
		t.Fatal("invalid price should report the error and preserve the input")
	}
	form.Set("product_price", "12,50")
	response := post("/admin/offers", form)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("create product = %d: %s", response.Code, response.Body.String())
	}
	link, err := database.GetLinkBySlug("local-product")
	if err != nil || link == nil {
		t.Fatal("product slug missing")
	}
	product, err := database.GetOfferByID(link.OfferID)
	if err != nil || product.PriceMinor == nil || *product.PriceMinor != 1250 || !product.ProductPage {
		t.Fatalf("product fields = %#v, %v", product, err)
	}
	edit := httptest.NewRequest("GET", "/admin/offers/"+strconv.FormatInt(product.ID, 10)+"?error=example", nil)
	for _, cookie := range cookies {
		edit.AddCookie(cookie)
	}
	editResponse := httptest.NewRecorder()
	handler.ServeHTTP(editResponse, edit)
	if !strings.Contains(editResponse.Body.String(), `name="product_price" type="text" inputmode="decimal" value="12.50"`) {
		t.Fatal("existing price should remain populated after an attach error")
	}
	// Older form submissions do not contain the new fields and must preserve them.
	legacy := url.Values{"title": {"Updated product"}, "item_type": {"product"}, "active": {"1"}}
	if response := post("/admin/offers/"+strconv.FormatInt(product.ID, 10), legacy); response.Code != http.StatusSeeOther {
		t.Fatalf("legacy update = %d", response.Code)
	}
	product, err = database.GetOfferByID(product.ID)
	if err != nil || !product.ProductPage || product.PriceMinor == nil || *product.PriceMinor != 1250 || product.ProductDetails != "Useful details" {
		t.Fatalf("legacy update erased product information: %#v %v", product, err)
	}
}
