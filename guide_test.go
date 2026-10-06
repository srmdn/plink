package plink

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/server"
)

func TestAdminGuideUsesConfiguredPathAndRequiresLogin(t *testing.T) {
	database, _, cfg := lifecycleTestServer(t)
	cfg.AdminPath = "control-room"
	handler := server.New(cfg, database, webFS)
	guideURL := "/control-room/guide"
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest("GET", guideURL, nil))
	if unauthorized.Code != 302 || unauthorized.Header().Get("Location") != "/control-room/login" {
		t.Fatalf("unauthenticated guide = %d, location %q", unauthorized.Code, unauthorized.Header().Get("Location"))
	}

	login := httptest.NewRequest("POST", "/control-room/login", strings.NewReader(url.Values{"password": {cfg.AdminPassword}}.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loggedIn := httptest.NewRecorder()
	handler.ServeHTTP(loggedIn, login)
	cookies := loggedIn.Result().Cookies()
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		return response
	}

	guide := get(guideURL)
	if guide.Code != 200 || guide.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("guide = %d: %s", guide.Code, guide.Body.String())
	}
	for _, target := range []string{"/control-room", "/control-room/offers", "/control-room/offers/new", "/control-room/analytics/dashboard", "/control-room/settings"} {
		if !strings.Contains(guide.Body.String(), `href="`+target+`"`) {
			t.Fatalf("guide missing configured module link %s", target)
		}
	}
	for _, path := range []string{"/control-room", "/control-room/offers", "/control-room/analytics/dashboard", "/control-room/settings"} {
		page := get(path)
		if page.Code != 200 || !strings.Contains(page.Body.String(), `href="`+guideURL+`"`) {
			t.Fatalf("%s missing guide navigation (status %d)", path, page.Code)
		}
	}
	var clicks, notices int
	if err := database.QueryRow("SELECT COUNT(*) FROM clicks").Scan(&clicks); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow("SELECT COUNT(*) FROM notice_views").Scan(&notices); err != nil {
		t.Fatal(err)
	}
	if clicks != 0 || notices != 0 {
		t.Fatal("admin guide navigation affected public analytics")
	}
}
