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

func TestCatalogAdminVisibilityFilter(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	for _, item := range []struct {
		title, slug, status string
		active              bool
	}{
		{"Visible referral", "visible-referral", "active", true},
		{"Hidden ended referral", "hidden-ended", "ended", false},
		{"Visible ended referral", "visible-ended", "ended", true},
	} {
		_, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: item.title, Active: item.active, ProgramStatus: item.status}, item.slug, "https://example.com")
		if err != nil {
			t.Fatal(err)
		}
	}
	login := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {"test-password"}}.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loggedIn := httptest.NewRecorder()
	handler.ServeHTTP(loggedIn, login)
	get := func(path string) string {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		for _, cookie := range loggedIn.Result().Cookies() {
			req.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != 200 {
			t.Fatalf("GET %s: %d", path, response.Code)
		}
		return response.Body.String()
	}
	hidden := get("/admin/offers?visibility=hidden")
	if !strings.Contains(hidden, "Hidden ended referral") || strings.Contains(hidden, "Visible referral") || strings.Contains(hidden, "Visible ended referral") {
		t.Fatal("hidden filter must follow visibility independently of program status")
	}
	if !strings.Contains(hidden, "Ended") || !strings.Contains(hidden, "Hidden") {
		t.Fatal("hidden ended item must show both program status and visibility")
	}
	shown := get("/admin/offers?visibility=shown")
	if !strings.Contains(shown, "Visible referral") || !strings.Contains(shown, "Visible ended referral") || strings.Contains(shown, "Hidden ended referral") {
		t.Fatal("shown filter must use the visibility setting, not availability")
	}
	if !strings.Contains(shown, "Allowed to appear") || !strings.Contains(shown, "Ended") {
		t.Fatal("shown ended item must show visibility and program status separately")
	}
	empty := get("/admin/offers?visibility=hidden&q=no-such-item")
	if !strings.Contains(empty, "No matching items") || !strings.Contains(empty, "Clear filters") || strings.Contains(empty, "Buat item pertama") {
		t.Fatal("filtered empty state must offer to clear filters")
	}
}

func TestCatalogSearchChannelSlugAndKeepFilters(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	offer, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Provider", ItemType: "referral", Active: true}, "primary", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	channel, err := database.CreateOfferLink(offer.ID, "promo-instagram", "https://example.com", "instagram", "launch")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.ToggleLink(channel.ID); err != nil {
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
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	search := request("GET", "/admin/offers?q=INSTAGRAM&item_type=referral&visibility=shown", nil)
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), "Provider") {
		t.Fatal("search should find a catalog item through its inactive channel slug")
	}
	for _, value := range []string{`name="q" value="INSTAGRAM"`, `name="item_type" value="referral"`, `name="visibility" value="shown"`} {
		if !strings.Contains(search.Body.String(), value) {
			t.Fatalf("toggle form missing %s", value)
		}
	}
	form := url.Values{"q": {"INSTAGRAM"}, "item_type": {"referral"}, "visibility": {"shown"}}
	res := request("POST", "/admin/offers/"+strconv.FormatInt(offer.ID, 10)+"/toggle", form)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("toggle status = %d", res.Code)
	}
	location, err := url.Parse(res.Header().Get("Location"))
	if err != nil || location.Path != "/admin/offers" || location.Query().Get("q") != "INSTAGRAM" || location.Query().Get("item_type") != "referral" || location.Query().Get("visibility") != "shown" {
		t.Fatalf("filters lost on redirect: %q", res.Header().Get("Location"))
	}
	if body := request("GET", location.RequestURI(), nil).Body.String(); !strings.Contains(body, "No matching items") {
		t.Fatal("hidden item should leave the shown-filter results after toggle")
	}
}
