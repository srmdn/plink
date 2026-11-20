package server

import (
	"net/url"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

// contacts builds a SiteSettings value with the contact feature on and the given
// stored channel rows, mirroring what the settings form saves.
func contactItemSettings(rows ...string) db.SiteSettings {
	return db.SiteSettings{ContactEnabled: true, ContactLinks: "[" + strings.Join(rows, ",") + "]"}
}

func contactRow(platform, rawURL string) string {
	return `{"platform":"` + platform + `","label":"","url":"` + rawURL + `"}`
}

func TestProductContactItemEmail(t *testing.T) {
	offer := &db.Offer{Title: "Buku & panduan 100%", HomeSlug: "buku"}
	action, label := productContactItem(contactItemSettings(contactRow("email", "mailto:hi@example.com")), offer, "https://shop.example")
	if label != productContactItemLabel {
		t.Fatalf("label = %q", label)
	}
	if !strings.HasPrefix(action, "mailto:hi@example.com?") {
		t.Fatalf("action = %q", action)
	}
	parsed, err := url.Parse(action)
	if err != nil {
		t.Fatal(err)
	}
	if subject := parsed.Query().Get("subject"); subject != offer.Title {
		t.Fatalf("subject = %q, want %q", subject, offer.Title)
	}
	// RFC 6068 mail clients read "+" literally, so spaces must be %20.
	if strings.Contains(action, "+") || !strings.Contains(action, "subject=Buku%20%26%20panduan%20100%25") {
		t.Fatalf("subject is not mailto-encoded: %q", action)
	}
}

func TestProductContactItemEmailKeepsExistingQuery(t *testing.T) {
	offer := &db.Offer{Title: "Guide", HomeSlug: "guide"}
	action, _ := productContactItem(contactItemSettings(contactRow("email", "mailto:hi@example.com?subject=Old&body=Hello%20there")), offer, "https://shop.example")
	parsed, err := url.Parse(action)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("subject") != "Guide" || query.Get("body") != "Hello there" {
		t.Fatalf("mailto query = %q", action)
	}
}

func TestProductContactItemWhatsApp(t *testing.T) {
	offer := &db.Offer{Title: "Buku & panduan", HomeSlug: "buku"}
	for _, raw := range []string{"https://wa.me/628123456789", "https://api.whatsapp.com/send?phone=628123456789"} {
		action, label := productContactItem(contactItemSettings(contactRow("whatsapp", raw)), offer, "https://shop.example/")
		if label != productContactItemWhatsAppLabel {
			t.Fatalf("%s label = %q", raw, label)
		}
		parsed, err := url.Parse(action)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Host != "wa.me" && parsed.Host != "api.whatsapp.com" {
			t.Fatalf("%s host = %q", raw, parsed.Host)
		}
		message := parsed.Query().Get("text")
		if !strings.Contains(message, productContactItemGreeting) ||
			!strings.Contains(message, offer.Title) ||
			!strings.Contains(message, "https://shop.example/offers/item/buku") {
			t.Fatalf("%s text = %q", raw, message)
		}
		if strings.Contains(action, "https://shop.example//offers") {
			t.Fatalf("%s page URL has a double slash: %q", raw, action)
		}
		if raw == "https://api.whatsapp.com/send?phone=628123456789" && parsed.Query().Get("phone") != "628123456789" {
			t.Fatalf("api.whatsapp.com phone was lost: %q", action)
		}
	}
}

func TestProductContactItemFirstActiveChannelWins(t *testing.T) {
	offer := &db.Offer{Title: "Buku", HomeSlug: "buku"}
	settings := contactItemSettings(
		contactRow("email", "mailto:hi@example.com"),
		contactRow("whatsapp", "https://wa.me/628123456789"),
	)
	if action, _ := productContactItem(settings, offer, "https://shop.example"); !strings.HasPrefix(action, "mailto:") {
		t.Fatalf("first channel did not win: %q", action)
	}
	settings = contactItemSettings(
		contactRow("whatsapp", "https://wa.me/628123456789"),
		contactRow("email", "mailto:hi@example.com"),
	)
	if action, label := productContactItem(settings, offer, "https://shop.example"); !strings.HasPrefix(action, "https://wa.me/") || label != productContactItemWhatsAppLabel {
		t.Fatalf("reordered first channel did not win: %q %q", action, label)
	}
}

func TestProductContactItemPassThroughAndEmpty(t *testing.T) {
	offer := &db.Offer{Title: "Buku", HomeSlug: "buku"}
	if action, label := productContactItem(contactItemSettings(contactRow("telegram", "https://t.me/saidwisdom")), offer, "https://shop.example"); action != "https://t.me/saidwisdom" || label != productContactItemLabel {
		t.Fatalf("telegram action = %q %q", action, label)
	}
	if action, label := productContactItem(contactItemSettings(contactRow("site", "https://example.com/contact?ref=shop")), offer, "https://shop.example"); action != "https://example.com/contact?ref=shop" || label != productContactItemLabel {
		t.Fatalf("external action = %q %q", action, label)
	}
	// A legacy bare address still becomes a usable mailto link.
	if action, _ := productContactItem(contactItemSettings(contactRow("email", "hi@example.com")), offer, "https://shop.example"); !strings.HasPrefix(action, "mailto:hi@example.com?subject=Buku") {
		t.Fatalf("legacy bare email action = %q", action)
	}
	// No channels, feature off, unsafe scheme, missing offer.
	if action, _ := productContactItem(db.SiteSettings{ContactEnabled: true}, offer, "https://shop.example"); action != "" {
		t.Fatalf("action without channels = %q", action)
	}
	disabled := contactItemSettings(contactRow("email", "mailto:hi@example.com"))
	disabled.ContactEnabled = false
	if action, _ := productContactItem(disabled, offer, "https://shop.example"); action != "" {
		t.Fatalf("action with contact disabled = %q", action)
	}
	if action, _ := productContactItem(contactItemSettings(contactRow("site", "javascript:alert(1)")), offer, "https://shop.example"); action != "" {
		t.Fatalf("unsafe scheme reached the action: %q", action)
	}
	if action, label := productContactItem(contactItemSettings(contactRow("email", "mailto:hi@example.com")), nil, "https://shop.example"); action != "" || label != productContactItemLabel {
		t.Fatalf("nil offer action = %q %q", action, label)
	}
}
