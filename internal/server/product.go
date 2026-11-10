package server

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/srmdn/plink/internal/db"
)

var productPricePattern = regexp.MustCompile(`^[0-9]{1,12}(?:[.,][0-9]{1,2})?$`)
var productCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
var whatsappNumberPattern = regexp.MustCompile(`^[1-9][0-9]{7,14}$`)

// Prices are stored in hundredths of the selected currency, never as floats.
func parseProductPrice(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if !productPricePattern.MatchString(value) {
		return nil, fmt.Errorf("the price must be a positive number with at most two decimals")
	}
	parts := strings.SplitN(strings.ReplaceAll(value, ",", "."), ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid price")
	}
	var fraction int64
	if len(parts) == 2 {
		digits := parts[1]
		if len(digits) == 1 {
			digits += "0"
		}
		fraction, _ = strconv.ParseInt(digits, 10, 64)
	}
	minor := whole*100 + fraction
	return &minor, nil
}

func productPriceInput(value *int64) string {
	if value == nil {
		return ""
	}
	if *value%100 == 0 {
		return strconv.FormatInt(*value/100, 10)
	}
	return fmt.Sprintf("%d.%02d", *value/100, *value%100)
}

func productPriceLabel(value *int64, currency string) string {
	if value == nil {
		return ""
	}
	if currency == "IDR" {
		digits := strconv.FormatInt(*value/100, 10)
		for i := len(digits) - 3; i > 0; i -= 3 {
			digits = digits[:i] + "." + digits[i:]
		}
		if *value%100 != 0 {
			return fmt.Sprintf("Rp%s,%02d", digits, *value%100)
		}
		return "Rp" + digits
	}
	return strings.TrimSpace(currency) + " " + productPriceInput(value)
}

func parseProductDetails(r *http.Request) (*db.ProductDetails, error) {
	if r.FormValue("product_fields_present") != "1" || r.FormValue("item_type") != "product" {
		return nil, nil
	}
	product := &db.ProductDetails{
		Enabled:     r.FormValue("product_page") == "1",
		Kind:        strings.TrimSpace(r.FormValue("product_kind")),
		Currency:    strings.ToUpper(strings.TrimSpace(r.FormValue("product_currency"))),
		Details:     strings.TrimSpace(r.FormValue("product_details")),
		Fulfillment: strings.TrimSpace(r.FormValue("product_fulfillment")),
	}
	if r.FormValue("whatsapp_fields_present") == "1" {
		product.WhatsApp = &db.ProductWhatsApp{
			Enabled: r.FormValue("whatsapp_enabled") == "1",
			Number:  strings.TrimSpace(r.FormValue("whatsapp_number")),
			Message: strings.TrimSpace(r.FormValue("whatsapp_message")),
		}
		if len(product.WhatsApp.Message) > 500 {
			return product, fmt.Errorf("pesan WhatsApp maksimal 500 karakter")
		}
		if product.WhatsApp.Enabled || product.WhatsApp.Number != "" {
			number, err := normalizeWhatsAppNumber(product.WhatsApp.Number)
			if err != nil {
				return product, err
			}
			product.WhatsApp.Number = number
		}
	}
	if product.Currency == "" {
		product.Currency = "IDR"
	}
	if product.WhatsApp != nil && product.WhatsApp.Enabled && !product.Enabled {
		return product, fmt.Errorf("enable the product detail page to use the WhatsApp button")
	}
	if product.Kind != "" && product.Kind != "physical" && product.Kind != "digital" {
		return product, fmt.Errorf("invalid product type")
	}
	if !productCurrencyPattern.MatchString(product.Currency) {
		return product, fmt.Errorf("the currency must be three letters, for example IDR")
	}
	if len(product.Details) > 5000 || len(product.Fulfillment) > 1000 {
		return product, fmt.Errorf("product details or shipping information are too long")
	}
	var err error
	product.PriceMinor, err = parseProductPrice(r.FormValue("product_price"))
	return product, err
}

func normalizeWhatsAppNumber(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "+") {
		raw = raw[1:]
	}
	var number strings.Builder
	for _, c := range raw {
		if c >= '0' && c <= '9' {
			number.WriteRune(c)
		} else if c != ' ' && c != '-' && c != '(' && c != ')' {
			return "", fmt.Errorf("the WhatsApp number must use a country code and digits")
		}
	}
	if !whatsappNumberPattern.MatchString(number.String()) {
		return "", fmt.Errorf("the WhatsApp number must be 8–15 digits including the country code, without a leading 0")
	}
	return number.String(), nil
}

type productPageData struct {
	Settings                            db.SiteSettings
	SEO                                 pageSEO
	Offer                               *db.Offer
	SiteName, SiteDesc, AdminPath, View string
	IsLoggedIn                          bool
}

func (s *Server) handleProductPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if !settings.StorefrontEnabled || !settings.CatalogEnabled {
		http.NotFound(w, r)
		return
	}
	offer, err := s.db.GetOfferByID(id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	if offer == nil || offer.ItemType != "product" || !offer.ProductPage || !isOfferAvailable(*offer, today) {
		http.NotFound(w, r)
		return
	}
	seo := s.itemSEO(settings, r, offer, "active", offer.HomeSlug)
	seo.URL = s.publicURL(r) + "/offers/item/" + strconv.FormatInt(id, 10)
	seo.NoIndex = false
	w.Header().Add("Vary", "Cookie")
	s.renderTemplate(w, "product-page", productPageData{Settings: settings, SEO: seo, Offer: offer, SiteName: settings.SiteName, SiteDesc: settings.SiteDesc, AdminPath: s.cfg.AdminPath, View: "categories", IsLoggedIn: s.isLoggedIn(r)})
}

func (s *Server) handleProductWhatsApp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	offer, err := s.db.GetOfferByID(id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	if !settings.StorefrontEnabled || !settings.CatalogEnabled || offer == nil || offer.ItemType != "product" || !offer.ProductPage || !offer.WhatsAppEnabled || !isOfferAvailable(*offer, today) || !whatsappNumberPattern.MatchString(offer.WhatsAppNumber) {
		http.NotFound(w, r)
		return
	}
	message := strings.TrimSpace(offer.WhatsAppMessage)
	if message == "" {
		message = "Hi, I am interested in this product."
	}
	message += "\n\nProduct: " + offer.Title + "\n" + s.publicURL(r) + "/offers/item/" + strconv.FormatInt(id, 10)
	destination := "https://wa.me/" + offer.WhatsAppNumber + "?" + url.Values{"text": {message}}.Encode()
	if r.Method == http.MethodGet && !isSocialPreviewBot(r.UserAgent()) {
		if err := s.db.RecordProductWhatsAppClick(id, time.Now().Unix()); err != nil {
			log.Printf("record product WhatsApp click: %v", err)
		}
	}
	http.Redirect(w, r, destination, http.StatusFound)
}
