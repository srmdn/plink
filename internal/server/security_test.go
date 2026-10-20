package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/config"
)

func TestClientIPOnlyTrustsProxyHeadersFromLoopback(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/", nil)
	r.Header.Set("X-Real-IP", "198.51.100.7")

	r.RemoteAddr = "203.0.113.10:1234"
	if got := clientIP(r); got != "203.0.113.10" {
		t.Fatalf("remote client IP = %q, want remote address", got)
	}

	r.RemoteAddr = "127.0.0.1:1234"
	if got := clientIP(r); got != "198.51.100.7" {
		t.Fatalf("proxied client IP = %q, want X-Real-IP", got)
	}

	r.Header.Del("X-Real-IP")
	r.Header.Set("X-Forwarded-For", "198.51.100.8, 10.0.0.1")
	if got := clientIP(r); got != "198.51.100.8" {
		t.Fatalf("forwarded client IP = %q, want first address", got)
	}
}

func TestBaseURLOnlyTrustsForwardedProtoFromLoopback(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")

	r.RemoteAddr = "203.0.113.10:1234"
	if got := baseURL(r); got != "http://example.test" {
		t.Fatalf("remote base URL = %q, want http", got)
	}

	r.RemoteAddr = "127.0.0.1:1234"
	if got := baseURL(r); got != "https://example.test" {
		t.Fatalf("proxied base URL = %q, want https", got)
	}
}

func TestRequireAuthSetsNoStore(t *testing.T) {
	s := &Server{cfg: &config.Config{AdminPath: "admin"}, sessions: newSessionStore()}
	handler := s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	for _, authenticated := range []bool{false, true} {
		r := httptest.NewRequest("GET", "/admin", nil)
		if authenticated {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: s.sessions.create()})
		}
		w := httptest.NewRecorder()
		handler(w, r)
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("authenticated=%v cache control = %q, want no-store", authenticated, got)
		}
	}
}

func TestStoredHeadersAreBounded(t *testing.T) {
	referrer := "https://example.com/" + strings.Repeat("a", maxStoredReferrer*2) + "?secret=1#fragment"
	if got := sanitizeReferrer(referrer); len(got) > maxStoredReferrer || strings.Contains(got, "secret") || strings.Contains(got, "#") {
		t.Fatalf("sanitized referrer = %d bytes, want bounded URL without query or fragment", len(got))
	}

	userAgent := strings.Repeat("x", maxStoredUserAgent*2)
	if got := sanitizeUserAgent(userAgent); len(got) > maxStoredUserAgent {
		t.Fatalf("sanitized user agent = %d bytes, want at most %d", len(got), maxStoredUserAgent)
	}
}
