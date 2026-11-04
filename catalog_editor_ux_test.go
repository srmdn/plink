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

func TestPublicCatalogNavigationKeepsExistingRoutes(t *testing.T) {
	_, handler, _ := lifecycleTestServer(t)
	for _, path := range []string{"/", "/offers", "/offers?view=categories"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		body := w.Body.String()
		if w.Code != http.StatusOK || !strings.Contains(body, `href="/offers?view=categories"`) || !strings.Contains(body, `Katalog</a>`) {
			t.Fatalf("%s: catalog navigation missing", path)
		}
		if path != "/" && !strings.Contains(body, `<h1>Katalog</h1>`) {
			t.Fatalf("%s: catalog heading missing", path)
		}
		if !strings.Contains(body, `href="/offers?view=services"`) || !strings.Contains(body, `href="/offers?view=projects"`) {
			t.Fatalf("%s: services or projects navigation lost", path)
		}
	}
}

func TestCatalogEditorKeepsExistingProgramSettings(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	input := db.OfferInput{Title: "Example referral", ItemType: "referral", Active: true, ProgramStatus: "ended", EndedBehavior: "redirect", FallbackURL: "https://example.com/fallback", NoticeMessage: "Program ended", StartsOn: "2026-01-01", EndsOn: "2026-12-31"}
	offer, err := database.CreateOfferWithHomepageLink(input, "example-offer", "https://example.com/ref")
	if err != nil {
		t.Fatal(err)
	}
	cookies := loginAs(t, handler, cfg)
	request := func(method, path string, form url.Values) *httptest.ResponseRecorder {
		t.Helper()
		body := ""
		if form != nil {
			body = form.Encode()
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
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
	newForm := request("GET", "/admin/offers/new", nil).Body.String()
	if !strings.Contains(newForm, `<details class="form-section offer-program-settings" >`) {
		t.Fatal("new item should initially collapse advanced settings")
	}
	path := "/admin/offers/" + strconv.FormatInt(offer.ID, 10)
	editForm := request("GET", path, nil).Body.String()
	if !strings.Contains(editForm, `<details class="form-section offer-program-settings" open>`) || !strings.Contains(editForm, input.FallbackURL) || !strings.Contains(editForm, input.NoticeMessage) {
		t.Fatal("existing settings should remain visible and populated")
	}
	form := url.Values{"title": {"Updated referral"}, "item_type": {"referral"}, "active": {"1"}, "program_status": {input.ProgramStatus}, "ended_behavior": {input.EndedBehavior}, "fallback_url": {input.FallbackURL}, "notice_message": {input.NoticeMessage}, "starts_on": {input.StartsOn}, "ends_on": {input.EndsOn}}
	if response := request("POST", path, form); response.Code != http.StatusSeeOther {
		t.Fatalf("save = %d: %s", response.Code, response.Body.String())
	}
	saved, err := database.GetOfferByID(offer.ID)
	if err != nil || saved.ProgramStatus != input.ProgramStatus || saved.EndedBehavior != input.EndedBehavior || saved.FallbackURL != input.FallbackURL || saved.NoticeMessage != input.NoticeMessage || saved.StartsOn != input.StartsOn || saved.EndsOn != input.EndsOn {
		t.Fatalf("program settings changed unexpectedly: %#v, %v", saved, err)
	}
}
