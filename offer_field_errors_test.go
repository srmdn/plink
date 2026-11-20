package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testOfferForm(fields map[string]string) url.Values {
	form := url.Values{
		"title":                  {"Produk uji"},
		"item_type":              {"product"},
		"home_slug":              {"produk-uji"},
		"home_url":               {"https://example.com/order"},
		"active":                 {"1"},
		"product_fields_present": {"1"},
		"product_page":           {"1"},
		"product_kind":           {"physical"},
		"product_currency":       {"IDR"},
		"product_price":          {"1250"},
		"product_details":        {"Isi produk"},
		"product_fulfillment":    {"Dikirim setelah konfirmasi"},
	}
	for key, value := range fields {
		form.Set(key, value)
	}
	return form
}

func postOfferForm(t *testing.T, handler http.Handler, cookies []*http.Cookie, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
		if cookie.Name == "plink_csrf" {
			req.Header.Set("X-CSRF-Token", cookie.Value)
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// The FieldErrors map itself is asserted in internal/server, where the form
// model lives. These tests cover the operator-visible half: the form re-renders,
// the banner still explains the failure, and the submitted value is preserved.
func TestOfferFormCurrencyErrorRerendersWithFieldFeedback(t *testing.T) {
	database, handler, cfg := uploadTestServer(t)
	rec := postOfferForm(t, handler, loginAs(t, handler, cfg), "/admin/offers", testOfferForm(map[string]string{"product_currency": "ID"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("invalid currency = %d, want the form re-rendered", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "the currency must be three letters") {
		t.Fatal("currency error message is missing from the form banner")
	}
	if !strings.Contains(body, `value="ID"`) {
		t.Fatal("submitted currency was not preserved")
	}
	if offers, err := database.ListOffers(); err != nil || len(offers) != 0 {
		t.Fatalf("invalid currency created %d offers: %v", len(offers), err)
	}
}

func TestOfferFormPriceErrorPreservesInput(t *testing.T) {
	_, handler, cfg := uploadTestServer(t)
	rec := postOfferForm(t, handler, loginAs(t, handler, cfg), "/admin/offers", testOfferForm(map[string]string{"product_price": "abc"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("invalid price = %d, want the form re-rendered", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "the price must be a positive number") {
		t.Fatal("price error message is missing from the form banner")
	}
	if !strings.Contains(body, `value="abc"`) {
		t.Fatal("submitted price was not preserved")
	}
}

func TestOfferFormWhatsAppNumberErrorPreservesInput(t *testing.T) {
	_, handler, cfg := uploadTestServer(t)
	form := testOfferForm(map[string]string{
		"whatsapp_fields_present": "1",
		"whatsapp_enabled":        "1",
		"whatsapp_number":         "08123456789",
	})
	rec := postOfferForm(t, handler, loginAs(t, handler, cfg), "/admin/offers", form)

	if rec.Code != http.StatusOK {
		t.Fatalf("invalid WhatsApp number = %d, want the form re-rendered", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "the WhatsApp number must be 8–15 digits") {
		t.Fatal("WhatsApp number error message is missing from the form banner")
	}
	if !strings.Contains(body, `value="08123456789"`) {
		t.Fatal("submitted WhatsApp number was not preserved")
	}
}

func TestOfferFormValidSubmitCreatesOffer(t *testing.T) {
	database, handler, cfg := uploadTestServer(t)
	rec := postOfferForm(t, handler, loginAs(t, handler, cfg), "/admin/offers", testOfferForm(nil))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("valid offer = %d: %s", rec.Code, rec.Body.String())
	}
	offers, err := database.ListOffers()
	if err != nil || len(offers) != 1 {
		t.Fatalf("valid offer created %d offers: %v", len(offers), err)
	}
}
