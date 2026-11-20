package server

import (
	"net/http/httptest"
	"net/url"
	"testing"
)

func offerValidationError(t *testing.T, form url.Values) error {
	t.Helper()
	r := httptest.NewRequest("POST", "/admin/offers", nil)
	r.Form = form
	r.PostForm = form
	_, err := parseOfferInput(r)
	if err == nil {
		t.Fatal("form should have failed validation")
	}
	return err
}

func validOfferForm() url.Values {
	return url.Values{
		"title":                  {"Produk uji"},
		"item_type":              {"product"},
		"product_fields_present": {"1"},
		"product_page":           {"1"},
		"product_kind":           {"physical"},
		"product_currency":       {"IDR"},
		"product_price":          {"1250"},
		"product_details":        {"Isi produk"},
		"product_fulfillment":    {"Dikirim setelah konfirmasi"},
	}
}

func TestOfferErrorDataMapsFields(t *testing.T) {
	cases := []struct {
		name  string
		field string
		form  url.Values
	}{
		{
			name:  "currency",
			field: "product_currency",
			form:  url.Values{"title": {"P"}, "item_type": {"product"}, "product_fields_present": {"1"}, "product_currency": {"ID"}},
		},
		{
			name:  "price",
			field: "product_price",
			form:  url.Values{"title": {"P"}, "item_type": {"product"}, "product_fields_present": {"1"}, "product_price": {"abc"}},
		},
		{
			name:  "whatsapp number",
			field: "whatsapp_number",
			form:  url.Values{"title": {"P"}, "item_type": {"product"}, "product_fields_present": {"1"}, "whatsapp_fields_present": {"1"}, "whatsapp_enabled": {"1"}, "whatsapp_number": {"08123456789"}},
		},
		{
			name:  "whatsapp message",
			field: "whatsapp_message",
			form:  url.Values{"title": {"P"}, "item_type": {"product"}, "product_fields_present": {"1"}, "whatsapp_fields_present": {"1"}, "whatsapp_message": {string(make([]byte, 501))}},
		},
		{
			name:  "product details",
			field: "product_details",
			form:  url.Values{"title": {"P"}, "item_type": {"product"}, "product_fields_present": {"1"}, "product_details": {string(make([]byte, 5001))}},
		},
		{
			name:  "product fulfillment",
			field: "product_fulfillment",
			form:  url.Values{"title": {"P"}, "item_type": {"product"}, "product_fields_present": {"1"}, "product_fulfillment": {string(make([]byte, 1001))}},
		},
		{
			name:  "product kind",
			field: "product_kind",
			form:  url.Values{"title": {"P"}, "item_type": {"product"}, "product_fields_present": {"1"}, "product_kind": {"unknown"}},
		},
		{
			name:  "product page required for WhatsApp",
			field: "product_page",
			form:  url.Values{"title": {"P"}, "item_type": {"product"}, "product_fields_present": {"1"}, "whatsapp_fields_present": {"1"}, "whatsapp_enabled": {"1"}, "whatsapp_number": {"628123456789"}},
		},
		{
			name:  "title",
			field: "title",
			form:  url.Values{"title": {""}, "item_type": {"referral"}},
		},
		{
			name:  "fallback URL",
			field: "fallback_url",
			form:  url.Values{"title": {"P"}, "item_type": {"referral"}, "ended_behavior": {"redirect"}},
		},
		{
			name:  "notice source URL",
			field: "notice_source_url",
			form:  url.Values{"title": {"P"}, "item_type": {"referral"}, "notice_source_url": {"not-a-url"}},
		},
		{
			name:  "start date",
			field: "starts_on",
			form:  url.Values{"title": {"P"}, "item_type": {"referral"}, "starts_on": {"2026-13-01"}},
		},
		{
			name:  "end date",
			field: "ends_on",
			form:  url.Values{"title": {"P"}, "item_type": {"referral"}, "starts_on": {"2026-11-10"}, "ends_on": {"2026-11-01"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := offerValidationError(t, tc.form)
			message, fields := offerErrorData(err)
			if fields[tc.field] == "" {
				t.Fatalf("FieldErrors[%q] is empty; got %#v", tc.field, fields)
			}
			if message == "" || message != fields[tc.field] {
				t.Fatalf("banner %q does not match field message %q", message, fields[tc.field])
			}
		})
	}
}

func TestOfferErrorDataValidInputHasNoFieldErrors(t *testing.T) {
	r := httptest.NewRequest("POST", "/admin/offers", nil)
	r.Form = validOfferForm()
	r.PostForm = r.Form
	if _, err := parseOfferInput(r); err != nil {
		t.Fatalf("valid form rejected: %v", err)
	}
	message, fields := offerErrorData(nil)
	if message != "" || fields != nil {
		t.Fatalf("valid form produced banner %q and fields %#v", message, fields)
	}
}
