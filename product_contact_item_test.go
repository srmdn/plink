package plink

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

// saveContactChannels stores contact rows exactly as the settings form would and
// returns the matching settings update helper.
func saveContactChannels(t *testing.T, database *db.DB, enabled bool, rows ...string) {
	t.Helper()
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContactEnabled = enabled
	settings.ContactLinks = ""
	if len(rows) > 0 {
		settings.ContactLinks = "[" + strings.Join(rows, ",") + "]"
	}
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
}

// TestProductPageItemContactAction covers the item-aware contact button: the
// prefilled channel URL reaches the page for a visitor, and the existing primary
// action keeps its destination and label.
func TestProductPageItemContactAction(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cfg.PublicURL = "https://shop.example"
	input := db.OfferInput{Title: "Buku panduan", ItemType: "product", Active: true, ButtonLabel: "Contact", Product: &db.ProductDetails{Enabled: true, Currency: "IDR"}}
	if _, err := database.CreateOfferWithHomepageLink(input, "buku", "https://example.com/order"); err != nil {
		t.Fatal(err)
	}
	get := func(path string) string {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s = %d", path, response.Code)
		}
		return response.Body.String()
	}
	pageURL := "/offers/item/buku"
	emailRow := `{"platform":"email","label":"","url":"mailto:hi@example.com"}`
	whatsAppRow := `{"platform":"whatsapp","label":"","url":"https://wa.me/628123456789"}`

	// (a) Email channel: the subject carries the item title, URL-encoded.
	saveContactChannels(t, database, true, emailRow)
	page := get(pageURL)
	if !strings.Contains(page, `mailto:hi@example.com?subject=Buku%20panduan`) {
		t.Fatal("product page is missing the item-aware email action")
	}
	if !strings.Contains(page, "Ask about this item") {
		t.Fatal("product page is missing the item contact label")
	}
	// (d) The primary button is untouched.
	if !strings.Contains(page, `href="/buku"`) || !strings.Contains(page, "Contact") {
		t.Fatal("primary product action changed")
	}

	// (b) WhatsApp channel: greeting, item title, and detail URL are prefilled.
	saveContactChannels(t, database, true, whatsAppRow)
	page = get(pageURL)
	match := regexp.MustCompile(`href="(https://wa\.me/[^"]+)"`).FindStringSubmatch(page)
	if match == nil {
		t.Fatal("product page is missing the item-aware WhatsApp action")
	}
	// html/template escapes "+" as &#43; inside the attribute; a browser decodes
	// it back before opening WhatsApp, so compare the decoded value.
	action, err := url.Parse(html.UnescapeString(match[1]))
	if err != nil {
		t.Fatal(err)
	}
	if action.Host != "wa.me" || action.Path != "/628123456789" {
		t.Fatalf("WhatsApp contact action = %q", action)
	}
	message := action.Query().Get("text")
	for _, value := range []string{"I'd like to ask about this item.", "Item: Buku panduan", "https://shop.example" + pageURL} {
		if !strings.Contains(message, value) {
			t.Fatalf("WhatsApp contact text is missing %q: %q", value, message)
		}
	}
	if !strings.Contains(page, "Ask about this item via WhatsApp") {
		t.Fatal("product page is missing the WhatsApp item contact label")
	}
	if !strings.Contains(page, `href="/buku"`) || !strings.Contains(page, "Contact") {
		t.Fatal("primary product action changed with WhatsApp contact configured")
	}

	// (c) No channel, or the feature switched off, hides the secondary action.
	for _, tc := range []struct {
		name    string
		enabled bool
		rows    []string
	}{
		{"no channels", true, nil},
		{"feature off", false, []string{emailRow}},
	} {
		saveContactChannels(t, database, tc.enabled, tc.rows...)
		page = get(pageURL)
		if strings.Contains(page, "Ask about this item") || strings.Contains(page, "mailto:hi@example.com") {
			t.Fatalf("%s: secondary contact action rendered", tc.name)
		}
		if !strings.Contains(page, `href="/buku"`) || !strings.Contains(page, "Contact") {
			t.Fatalf("%s: primary product action changed", tc.name)
		}
	}

	// The first active channel matches the public contact bubble order.
	saveContactChannels(t, database, true, emailRow, whatsAppRow)
	page = get(pageURL)
	// The floating bubble lists every channel, but only the item action carries a
	// prefilled subject, so exactly one subject-bearing link proves which channel
	// won.
	if strings.Count(page, "?subject=") != 1 || !strings.Contains(page, "mailto:hi@example.com?subject=Buku%20panduan") {
		t.Fatal("secondary contact action did not use the first active channel")
	}
}

// TestProductPageWhatsAppPrimaryActionUnchanged keeps the WhatsApp product
// button independent from the new secondary contact button.
func TestProductPageWhatsAppPrimaryActionUnchanged(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	input := db.OfferInput{Title: "Buku panduan", ItemType: "product", Active: true, Product: &db.ProductDetails{Enabled: true, Currency: "IDR", WhatsApp: &db.ProductWhatsApp{Enabled: true, Number: "628123456789", Message: "Halo"}}}
	if _, err := database.CreateOfferWithHomepageLink(input, "buku-wa", "https://example.com/order"); err != nil {
		t.Fatal(err)
	}
	saveContactChannels(t, database, true, `{"platform":"email","label":"","url":"mailto:hi@example.com"}`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/offers/item/buku-wa", nil))
	page := response.Body.String()
	for _, value := range []string{`href="/offers/item/buku-wa/whatsapp"`, "Order via WhatsApp", "mailto:hi@example.com?subject=Buku%20panduan"} {
		if !strings.Contains(page, value) {
			t.Fatalf("product page is missing %q", value)
		}
	}
	if strings.Contains(page, "<a class=\"product-action\" href=\"/buku-wa\"") {
		t.Fatal("primary WhatsApp action was replaced by the slug destination")
	}
}
