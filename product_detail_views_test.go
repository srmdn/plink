package plink

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/srmdn/plink/internal/db"
)

// waitForRowCount polls a COUNT(*) query because detail views are inserted
// from a background goroutine, like redirect clicks.
func waitForRowCount(t *testing.T, database *db.DB, want int, query string, args ...any) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var got int
		if err := database.QueryRow(query, args...).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("row count = %d, want %d: %s", got, want, query)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestProductDetailViewsAreRecordedForOperators(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	product, err := database.CreateOfferWithHomepageLink(db.OfferInput{
		Title: "Tracked product", ItemType: "product", Active: true,
		Product: &db.ProductDetails{Enabled: true, Currency: "IDR"},
	}, "tracked-product", "https://example.com/order")
	if err != nil {
		t.Fatal(err)
	}
	path := "/offers/item/tracked-product"
	get := func(target, agent string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("GET", target, nil)
		request.Header.Set("User-Agent", agent)
		request.Header.Set("Referer", "https://social.example/post?private=value#fragment")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	for i := 0; i < 2; i++ {
		if code := get(path, "Mozilla/5.0").Code; code != http.StatusOK {
			t.Fatalf("detail page = %d", code)
		}
	}
	viewsQuery := `SELECT COUNT(*) FROM product_views WHERE offer_id = ?`
	waitForRowCount(t, database, 2, viewsQuery, product.ID)

	var referrer, agent string
	if err := database.QueryRow(`SELECT referrer, user_agent FROM product_views WHERE offer_id = ? ORDER BY id LIMIT 1`, product.ID).Scan(&referrer, &agent); err != nil {
		t.Fatal(err)
	}
	if referrer != "https://social.example/post" || agent != "Mozilla/5.0" {
		t.Fatalf("stored view metadata = %q %q", referrer, agent)
	}

	// Preview bots and the numeric compatibility alias must not add a view.
	if code := get(path, "Twitterbot/1.0").Code; code != http.StatusOK {
		t.Fatalf("preview bot detail page = %d", code)
	}
	numericURL := "/offers/item/" + strconv.FormatInt(product.ID, 10)
	if response := get(numericURL, "Mozilla/5.0"); response.Code != http.StatusMovedPermanently || response.Header().Get("Location") != path {
		t.Fatalf("numeric detail URL = %d %q", response.Code, response.Header().Get("Location"))
	}
	time.Sleep(200 * time.Millisecond)
	waitForRowCount(t, database, 2, viewsQuery, product.ID)

	// The admin view model carries the all-time total for the operator.
	stored, err := database.GetOfferByID(product.ID)
	if err != nil || stored == nil || stored.DetailViews != 2 {
		t.Fatalf("admin detail views = %#v, %v", stored, err)
	}
	if stored.Clicks != 0 || stored.WhatsAppClicks != 0 {
		t.Fatalf("detail views leaked into other counters: %#v", stored)
	}
	admin := httptest.NewRequest("GET", "/admin/offers/"+strconv.FormatInt(product.ID, 10), nil)
	for _, cookie := range loginAs(t, handler, cfg) {
		admin.AddCookie(cookie)
	}
	adminResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminResponse, admin)
	if adminResponse.Code != http.StatusOK || !strings.Contains(adminResponse.Body.String(), "Tracked product") {
		t.Fatalf("admin offer page = %d", adminResponse.Code)
	}
}

func TestProductWhatsAppNumericAliasRecordsOneClick(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	product, err := database.CreateOfferWithHomepageLink(db.OfferInput{
		Title: "Chat product", ItemType: "product", Active: true,
		Product: &db.ProductDetails{Enabled: true, Currency: "IDR", WhatsApp: &db.ProductWhatsApp{Enabled: true, Number: "628123456789"}},
	}, "chat-product", "https://example.com/original")
	if err != nil {
		t.Fatal(err)
	}
	get := func(target string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", target, nil))
		return response
	}
	canonical := "/offers/item/chat-product/whatsapp"
	alias := "/offers/item/" + strconv.FormatInt(product.ID, 10) + "/whatsapp"
	if response := get(alias); response.Code != http.StatusMovedPermanently || response.Header().Get("Location") != canonical {
		t.Fatalf("numeric WhatsApp URL = %d %q, want 301 to %s", response.Code, response.Header().Get("Location"), canonical)
	}
	if stored, err := database.GetOfferByID(product.ID); err != nil || stored.WhatsAppClicks != 0 {
		t.Fatalf("the numeric alias recorded a click: %#v, %v", stored, err)
	}
	if response := get(canonical); response.Code != http.StatusFound || !strings.HasPrefix(response.Header().Get("Location"), "https://wa.me/628123456789?") {
		t.Fatalf("canonical WhatsApp URL = %d %q", response.Code, response.Header().Get("Location"))
	}
	stored, err := database.GetOfferByID(product.ID)
	if err != nil || stored.WhatsAppClicks != 1 {
		t.Fatalf("WhatsApp clicks = %#v, %v", stored, err)
	}
}

func TestProductDetailSlugResolution(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	get := func(target string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", target, nil))
		return response
	}
	productInput := func(title string, enabled bool) db.OfferInput {
		return db.OfferInput{Title: title, ItemType: "product", Active: true,
			Product: &db.ProductDetails{Enabled: enabled, Currency: "IDR"}}
	}
	referral, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Referral", ItemType: "referral", Active: true}, "plain-referral", "https://example.com/referral")
	if err != nil {
		t.Fatal(err)
	}
	if code := get("/offers/item/plain-referral").Code; code != http.StatusNotFound {
		t.Fatalf("referral detail slug = %d", code)
	}
	if code := get("/offers/item/" + strconv.FormatInt(referral.ID, 10)).Code; code != http.StatusNotFound {
		t.Fatalf("referral numeric detail = %d", code)
	}
	disabled, err := database.CreateOfferWithHomepageLink(productInput("No detail page", false), "no-detail-page", "https://example.com/no-detail")
	if err != nil {
		t.Fatal(err)
	}
	if code := get("/offers/item/no-detail-page").Code; code != http.StatusNotFound {
		t.Fatalf("disabled detail slug = %d", code)
	}
	hidden, err := database.CreateOfferWithHomepageLink(productInput("Hidden product", true), "hidden-product", "https://example.com/hidden")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.ToggleOffer(hidden.ID); err != nil {
		t.Fatal(err)
	}
	if code := get("/offers/item/hidden-product").Code; code != http.StatusNotFound {
		t.Fatalf("hidden detail slug = %d", code)
	}
	// A slug made only of digits is still a slug, not an internal ID.
	numericSlug, err := database.CreateOfferWithHomepageLink(productInput("Numeric slug", true), "20260101", "https://example.com/numeric")
	if err != nil {
		t.Fatal(err)
	}
	if code := get("/offers/item/20260101").Code; code != http.StatusOK {
		t.Fatalf("numeric slug detail = %d", code)
	}
	if code := get("/offers/item/20260101/whatsapp").Code; code != http.StatusNotFound {
		t.Fatalf("numeric slug without WhatsApp = %d", code)
	}
	for _, target := range []string{"/offers/item/unknown-slug", "/offers/item/999999", "/offers/item/0", "/offers/item/" + strconv.FormatInt(disabled.ID, 10)} {
		if code := get(target).Code; code != http.StatusNotFound {
			t.Fatalf("%s = %d, want 404", target, code)
		}
	}
	if numericSlug.ID == 0 {
		t.Fatal("numeric slug offer was not created")
	}
}
