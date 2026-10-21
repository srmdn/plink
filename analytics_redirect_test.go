package plink

import (
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestAnalyticsFragmentsRedirectDirectNavigation(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	link, err := database.CreateLink("analytics-link", "https://example.com/analytics", "", "Tools")
	if err != nil {
		t.Fatal(err)
	}
	cookies := loginAs(t, handler, cfg)

	get := func(path string, htmx bool) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest("GET", path, nil)
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
		if htmx {
			request.Header.Set("HX-Request", "true")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	overview := get("/admin/analytics?date=2026-10-21", false)
	if overview.Code != 302 || overview.Header().Get("Location") != "/admin/analytics/dashboard?date=2026-10-21" {
		t.Fatalf("direct overview = %d, location %q", overview.Code, overview.Header().Get("Location"))
	}
	if overview := get("/admin/analytics", true); overview.Code != 200 {
		t.Fatalf("htmx overview = %d, want 200", overview.Code)
	}

	linkPath := "/admin/links/" + strconv.FormatInt(link.ID, 10) + "/analytics"
	linkRedirect := get(linkPath, false)
	want := "/admin/analytics/dashboard?link=" + strconv.FormatInt(link.ID, 10)
	if linkRedirect.Code != 302 || linkRedirect.Header().Get("Location") != want {
		t.Fatalf("direct link analytics = %d, location %q, want %q", linkRedirect.Code, linkRedirect.Header().Get("Location"), want)
	}
	if linkAnalytics := get(linkPath, true); linkAnalytics.Code != 200 {
		t.Fatalf("htmx link analytics = %d, want 200", linkAnalytics.Code)
	}
}
