package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/config"
)

// adminLogin logs in and returns the cookies the browser would keep. The CSRF
// token is issued once, at login, and is expected to stay stable.
func adminLogin(t *testing.T, handler http.Handler, cfg *config.Config) []*http.Cookie {
	t.Helper()
	form := url.Values{"password": {cfg.AdminPassword}}
	request := httptest.NewRequest("POST", "/admin/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	cookies := response.Result().Cookies()
	if cookieNamed(cookies, "plink_session") == nil || cookieNamed(cookies, "plink_csrf") == nil {
		t.Fatal("login did not issue a session and CSRF cookie")
	}
	return cookies
}

func cookieNamed(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// adminRequest builds an admin request carrying the given cookies. Header
// values such as the CSRF token are added by the caller.
func adminRequest(method, path string, form url.Values, cookies []*http.Cookie) *http.Request {
	var request *http.Request
	if form == nil {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	return request
}

func adminGet(t *testing.T, handler http.Handler, cookies []*http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, adminRequest("GET", path, nil, cookies))
	return response
}

// TestCSRFCookieStableAcrossAdminGETs is the regression test for the multi-tab
// bug: authenticated page loads used to rotate plink_csrf, so any form already
// rendered in another tab submitted a token the server no longer accepted.
func TestCSRFCookieStableAcrossAdminGETs(t *testing.T) {
	_, handler, cfg := lifecycleTestServer(t)
	cookies := adminLogin(t, handler, cfg)
	initial := cookieNamed(cookies, "plink_csrf").Value

	for _, path := range []string{"/admin", "/admin/settings", "/admin/offers", "/admin/links"} {
		response := adminGet(t, handler, cookies, path)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, response.Code)
		}
		if rotated := cookieNamed(response.Result().Cookies(), "plink_csrf"); rotated != nil && rotated.Value != initial {
			t.Fatalf("GET %s rotated the CSRF cookie", path)
		}
	}
}

// TestCSRFTokenFromFirstTabStillAccepted simulates two tabs: tab A renders a
// form, tab B loads another admin page, then tab A submits the token it read at
// page load.
func TestCSRFTokenFromFirstTabStillAccepted(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cookies := adminLogin(t, handler, cfg)
	token := cookieNamed(cookies, "plink_csrf").Value

	if response := adminGet(t, handler, cookies, "/admin/settings"); response.Code != http.StatusOK {
		t.Fatalf("second tab load = %d, want 200", response.Code)
	}

	form := url.Values{
		"site_name":      {"Multi tab"},
		"featured_limit": {"3"},
		"csrf_token":     {token},
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, adminRequest("POST", "/admin/settings", form, cookies))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("submit from first tab = %d, want 303", response.Code)
	}
	settings, err := database.GetSiteSettings()
	if err != nil || settings.SiteName != "Multi tab" {
		t.Fatalf("settings from first tab not saved: %v", err)
	}
}

// TestStaleCSRFCookieIsRebound covers recovery: a cookie that is not bound to
// the live session (for example a leftover from a lost logout response) is
// replaced on the next admin page load instead of leaving every form at 403.
func TestStaleCSRFCookieIsRebound(t *testing.T) {
	_, handler, cfg := lifecycleTestServer(t)
	cookies := adminLogin(t, handler, cfg)
	session := cookieNamed(cookies, "plink_session")

	stale := &http.Cookie{Name: "plink_csrf", Value: "stale-token-from-nowhere"}
	response := adminGet(t, handler, []*http.Cookie{session, stale}, "/admin")
	if response.Code != http.StatusOK {
		t.Fatalf("admin page = %d, want 200", response.Code)
	}
	fresh := cookieNamed(response.Result().Cookies(), "plink_csrf")
	if fresh == nil || fresh.Value == "stale-token-from-nowhere" {
		t.Fatal("stale CSRF cookie was not replaced")
	}

	form := url.Values{"site_name": {"Rebound"}, "featured_limit": {"3"}, "csrf_token": {fresh.Value}}
	saved := httptest.NewRecorder()
	handler.ServeHTTP(saved, adminRequest("POST", "/admin/settings", form, []*http.Cookie{session, fresh}))
	if saved.Code != http.StatusSeeOther {
		t.Fatalf("save with rebound token = %d, want 303", saved.Code)
	}
}

// TestCSRFRejectsTokenFromAnotherSession keeps the synchronizer honest: a token
// minted for one session must not work against another, even when the request
// sends that token in both the cookie and the form field.
func TestCSRFRejectsTokenFromAnotherSession(t *testing.T) {
	_, handler, cfg := lifecycleTestServer(t)
	first := adminLogin(t, handler, cfg)
	second := adminLogin(t, handler, cfg)
	firstCSRF := cookieNamed(first, "plink_csrf")
	secondCSRF := cookieNamed(second, "plink_csrf")
	if firstCSRF.Value == secondCSRF.Value {
		t.Fatal("two sessions received the same CSRF token")
	}

	form := url.Values{"site_name": {"Cross session"}, "featured_limit": {"3"}}
	submit := func(cookies []*http.Cookie, token string) int {
		values := url.Values{}
		for key, value := range form {
			values[key] = value
		}
		values.Set("csrf_token", token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, adminRequest("POST", "/admin/settings", values, cookies))
		return response.Code
	}

	for _, tc := range []struct {
		name    string
		cookies []*http.Cookie
		token   string
	}{
		{"first session with second token", []*http.Cookie{cookieNamed(first, "plink_session"), firstCSRF}, secondCSRF.Value},
		{"first session with second cookie and token", []*http.Cookie{cookieNamed(first, "plink_session"), secondCSRF}, secondCSRF.Value},
		{"second session with first cookie and token", []*http.Cookie{cookieNamed(second, "plink_session"), firstCSRF}, firstCSRF.Value},
	} {
		if code := submit(tc.cookies, tc.token); code != http.StatusForbidden {
			t.Fatalf("%s = %d, want 403", tc.name, code)
		}
	}

	if code := submit([]*http.Cookie{cookieNamed(first, "plink_session"), firstCSRF}, firstCSRF.Value); code != http.StatusSeeOther {
		t.Fatalf("matching session and token = %d, want 303", code)
	}
}
