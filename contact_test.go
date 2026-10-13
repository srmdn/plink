package plink

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/db"
)

func TestPublicContactButton(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)

	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.SiteName = "Said"
	settings.ContactTitle = "Chat saya"
	settings.ContactNote = "Balas cepat ya"
	settings.ContactLinks = `[{"platform":"whatsapp","url":"https://wa.me/628123"},{"platform":"email","url":"mailto:hi@example.com"}]`
	settings.ContactEnabled = true
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}

	for _, path := range []string{"/", "/offers"} {
		body := get(path).Body.String()
		if !strings.Contains(body, "data-contact-fab") || !strings.Contains(body, "https://wa.me/628123") || !strings.Contains(body, "Chat saya") {
			t.Fatalf("contact button missing or incomplete on %s", path)
		}
	}

	settings.ContactEnabled = false
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(get("/").Body.String(), "data-contact-fab") {
		t.Fatal("contact button rendered while the feature is disabled")
	}

	settings.ContactEnabled = true
	settings.ContactLinks = ""
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(get("/").Body.String(), "data-contact-fab") {
		t.Fatal("contact button rendered without any channels")
	}
}

func TestContactChannelHiddenFromPublicResources(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)

	if _, err := database.CreateLinkWithOptions("wa", "https://wa.me/628123", "WhatsApp", "", db.LinkOptions{Channel: "contact"}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateLinkWithOptions("promo", "https://example.com", "Promo", "", db.LinkOptions{}); err != nil {
		t.Fatal(err)
	}

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}

	links := get("/links").Body.String()
	if strings.Contains(links, "/wa\"") {
		t.Fatal("contact-channel link appeared in the public resources list")
	}
	if !strings.Contains(links, "/promo") {
		t.Fatal("normal link missing from the public resources list")
	}
	if code := get("/wa").Code; code != http.StatusFound {
		t.Fatalf("contact-channel link did not redirect, status=%d", code)
	}
}
