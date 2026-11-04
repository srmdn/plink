package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestUnlistedStandaloneLink(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
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
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	form := url.Values{"slug": {"shared-only"}, "url": {"https://example.com/target"}, "description": {"Hidden resource"}, "category": {"Unique hidden category"}, "featured": {"1"}, "unlisted": {"1"}}
	if response := request("POST", "/admin/links", form); response.Code != http.StatusOK {
		t.Fatalf("create: %d %s", response.Code, response.Body.String())
	}
	link, err := database.GetLinkBySlug("shared-only")
	if err != nil || link == nil {
		t.Fatalf("lookup: %#v, %v", link, err)
	}
	id := strconv.FormatInt(link.ID, 10)
	stored, err := database.GetLinkByID(link.ID)
	if err != nil || !stored.Unlisted {
		t.Fatalf("visibility not stored: %#v %v", stored, err)
	}
	for _, path := range []string{"/", "/links", "/offers"} {
		body := request("GET", path, nil).Body.String()
		if strings.Contains(body, "shared-only") || strings.Contains(body, "Unique hidden category") {
			t.Fatalf("unlisted link leaked into %s", path)
		}
	}
	redirect := request("HEAD", "/shared-only", nil)
	if redirect.Code != http.StatusFound || redirect.Header().Get("Location") != "https://example.com/target" {
		t.Fatalf("shared link = %d, %q", redirect.Code, redirect.Header().Get("Location"))
	}
	if !strings.Contains(request("GET", "/admin/links/"+id+"/edit", nil).Body.String(), `name="unlisted" value="1" checked`) {
		t.Fatal("edit form lost unlisted setting")
	}
	form.Del("unlisted")
	request("PUT", "/admin/links/"+id, form)
	if !strings.Contains(request("GET", "/links", nil).Body.String(), "shared-only") {
		t.Fatal("link should appear after enabling its public listing")
	}
}
