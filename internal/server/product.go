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
			return product, offerFieldErrorf("whatsapp_message", "the WhatsApp message must be 500 characters or fewer")
		}
		if product.WhatsApp.Enabled || product.WhatsApp.Number != "" {
			number, err := normalizeWhatsAppNumber(product.WhatsApp.Number)
			if err != nil {
				return product, &offerFieldError{Field: "whatsapp_number", Message: err.Error()}
			}
			product.WhatsApp.Number = number
		}
	}
	if product.Currency == "" {
		product.Currency = "IDR"
	}
	if product.WhatsApp != nil && product.WhatsApp.Enabled && !product.Enabled {
		return product, offerFieldErrorf("product_page", "enable the product detail page to use the WhatsApp button")
	}
	if product.Kind != "" && product.Kind != "physical" && product.Kind != "digital" {
		return product, offerFieldErrorf("product_kind", "invalid product type")
	}
	if !productCurrencyPattern.MatchString(product.Currency) {
		return product, offerFieldErrorf("product_currency", "the currency must be three letters, for example IDR")
	}
	if len(product.Details) > 5000 {
		return product, offerFieldErrorf("product_details", "product details must be 5000 characters or fewer")
	}
	if len(product.Fulfillment) > 1000 {
		return product, offerFieldErrorf("product_fulfillment", "shipping information must be 1000 characters or fewer")
	}
	price, err := parseProductPrice(r.FormValue("product_price"))
	if err != nil {
		return product, &offerFieldError{Field: "product_price", Message: err.Error()}
	}
	product.PriceMinor = price
	return product, nil
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
	// ContactItemAction is a ready-to-use URL that starts a conversation about
	// this item on the first active public contact channel. It is empty when the
	// contact feature is off or no channel is configured, so the template hides
	// the secondary button.
	ContactItemAction string
	// ContactItemLabel is the short English label for that secondary button.
	ContactItemLabel string
}

// Label defaults are locked by the P2b contract.
const (
	productContactItemLabel         = "Ask about this item"
	productContactItemWhatsAppLabel = "Ask about this item via WhatsApp"
	productContactItemGreeting      = "Hi, I'd like to ask about this item."
)

// productContactItem builds the item-aware contact action for a product detail
// page. Channels come from buildPublicProfile, the same ordered list the public
// floating contact bubble renders, so the first entry matches what a visitor
// would see there. Only a mailto: and WhatsApp destinations receive prefilled
// context: every other channel is returned unchanged because there is no safe
// prefill mechanism.
func productContactItem(settings db.SiteSettings, offer *db.Offer, baseURL string) (string, string) {
	label := productContactItemLabel
	if !settings.ContactEnabled || offer == nil {
		return "", label
	}
	profile := buildPublicProfile(settings)
	if len(profile.Contact) == 0 {
		return "", label
	}
	channel := profile.Contact[0]
	raw := strings.TrimSpace(channel.URL)
	if channel.Key == "email" && !strings.Contains(raw, ":") {
		// Legacy rows may store a bare address; the settings form normalizes new
		// ones to mailto:. Prefilling a subject needs a real mailto: URL.
		raw = "mailto:" + raw
	}
	target, err := url.Parse(raw)
	if err != nil {
		return "", label
	}
	switch target.Scheme {
	case "mailto":
		query := target.Query()
		query.Set("subject", offer.Title)
		target.RawQuery = encodeMailtoQuery(query)
		return target.String(), label
	case "http", "https":
		switch strings.ToLower(target.Hostname()) {
		case "wa.me", "api.whatsapp.com":
			pageURL := strings.TrimRight(baseURL, "/") + productDetailPath(offer.HomeSlug)
			message := productContactItemGreeting + "\n\nItem: " + offer.Title + "\n" + pageURL
			query := target.Query()
			query.Set("text", message)
			target.RawQuery = query.Encode()
			return target.String(), productContactItemWhatsAppLabel
		}
		return target.String(), label
	case "":
		// Legacy rows may hold a scheme-less host or path; keep it as-is.
		return target.String(), label
	default:
		// Reject executable schemes (for example javascript: or data:); they can
		// only come from manual database writes and must never reach an href.
		return "", label
	}
}

// encodeMailtoQuery encodes a mailto query with %20 for spaces. RFC 6068 treats
// "+" as a literal plus, not a space, so url.Values.Encode alone is not safe
// for mail clients.
func encodeMailtoQuery(values url.Values) string {
	return strings.ReplaceAll(values.Encode(), "+", "%20")
}

func (s *Server) handleProductPage(w http.ResponseWriter, r *http.Request) {
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if !settings.StorefrontEnabled || !settings.CatalogEnabled {
		http.NotFound(w, r)
		return
	}
	offer, legacyID, err := s.productOfferForSegment(r.PathValue("slug"))
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	if offer == nil || offer.ItemType != "product" || !offer.ProductPage || !isOfferAvailable(*offer, today) {
		http.NotFound(w, r)
		return
	}
	// The numeric form is only a compatibility alias: visitors and crawlers are
	// sent to the canonical card slug without recording anything.
	if legacyID {
		target := productDetailPath(offer.HomeSlug)
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}
	seo := s.itemSEO(settings, r, offer, "active", offer.HomeSlug)
	seo.URL = s.publicURL(r) + productDetailPath(offer.HomeSlug)
	seo.Nonce = cspNonceFrom(r)
	seo.NoIndex = false
	contactAction, contactLabel := productContactItem(settings, offer, s.publicURL(r))
	w.Header().Add("Vary", "Cookie")
	if r.Method == http.MethodGet && !isSocialPreviewBot(r.UserAgent()) {
		referrer, userAgent, viewedAt := sanitizeReferrer(r.Referer()), sanitizeUserAgent(r.UserAgent()), time.Now().Unix()
		go func() {
			if err := s.db.RecordProductView(offer.ID, referrer, userAgent, viewedAt); err != nil {
				log.Printf("record product view: %v", err)
			}
		}()
	}
	s.renderTemplate(w, "product-page", productPageData{Settings: settings, SEO: seo, Offer: offer, SiteName: settings.SiteName, SiteDesc: settings.SiteDesc, AdminPath: s.cfg.AdminPath, View: "categories", IsLoggedIn: s.isLoggedIn(r), ContactItemAction: contactAction, ContactItemLabel: contactLabel})
}

// productDetailPath is the canonical public path of a product detail page.
func productDetailPath(slug string) string {
	return "/offers/item/" + url.PathEscape(slug)
}

// productOfferForSegment resolves one /offers/item/{segment} path value. A card
// slug wins over a numeric ID, so a slug made only of digits keeps working; an
// all-digit segment with no matching slug is the legacy /offers/item/{id}
// form, reported through legacyID.
func (s *Server) productOfferForSegment(segment string) (offer *db.Offer, legacyID bool, err error) {
	offer, err = s.db.GetOfferByHomeSlug(segment)
	if err != nil || offer != nil {
		return offer, false, err
	}
	if !isNumericSegment(segment) {
		return nil, false, nil
	}
	id, err := strconv.ParseInt(segment, 10, 64)
	if err != nil || id <= 0 {
		return nil, false, nil
	}
	offer, err = s.db.GetOfferByID(id)
	if err != nil {
		return nil, false, err
	}
	return offer, offer != nil, nil
}

func isNumericSegment(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// productWhatsAppAvailable is the single availability rule for the contact
// action, shared by the canonical slug URL and its numeric alias.
func productWhatsAppAvailable(offer *db.Offer, today string) bool {
	return offer != nil && offer.ItemType == "product" && offer.ProductPage && offer.WhatsAppEnabled &&
		isOfferAvailable(*offer, today) && whatsappNumberPattern.MatchString(offer.WhatsAppNumber)
}

func (s *Server) handleProductWhatsApp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	offer, legacyID, err := s.productOfferForSegment(r.PathValue("slug"))
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	if !settings.StorefrontEnabled || !settings.CatalogEnabled || !productWhatsAppAvailable(offer, today) {
		http.NotFound(w, r)
		return
	}
	// The alias never records a click; the canonical slug endpoint below owns
	// the single recorded click.
	if legacyID {
		http.Redirect(w, r, productDetailPath(offer.HomeSlug)+"/whatsapp", http.StatusMovedPermanently)
		return
	}
	message := strings.TrimSpace(offer.WhatsAppMessage)
	if message == "" {
		message = "Hi, I am interested in this product."
	}
	message += "\n\nProduct: " + offer.Title + "\n" + s.publicURL(r) + productDetailPath(offer.HomeSlug)
	destination := "https://wa.me/" + offer.WhatsAppNumber + "?" + url.Values{"text": {message}}.Encode()
	if r.Method == http.MethodGet && !isSocialPreviewBot(r.UserAgent()) {
		if err := s.db.RecordProductWhatsAppClick(offer.ID, time.Now().Unix()); err != nil {
			log.Printf("record product WhatsApp click: %v", err)
		}
	}
	http.Redirect(w, r, destination, http.StatusFound)
}
