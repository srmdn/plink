package plink

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
	"github.com/srmdn/plink/internal/server"
)

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func sessionTestConfig() *config.Config {
	return &config.Config{AdminPath: "admin", AdminPassword: "test-password", SiteName: "Restart", Timezone: "Asia/Jakarta"}
}

// TestSessionsSurviveRestart rebuilds the server over the same database file,
// which is what a deploy does, and expects the login to still be valid.
func TestSessionsSurviveRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	cfg := sessionTestConfig()

	database, err := db.Init(path)
	if err != nil {
		t.Fatal(err)
	}
	cookies := adminLogin(t, server.New(cfg, database, webFS), cfg)
	session := cookieNamed(cookies, "plink_session").Value
	csrf := cookieNamed(cookies, "plink_csrf").Value
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := db.Init(path)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	handler := server.New(cfg, restarted, webFS)

	if response := adminGet(t, handler, cookies, "/admin"); response.Code != http.StatusOK {
		t.Fatalf("admin page after restart = %d, want 200", response.Code)
	}
	// The CSRF token bound at login must survive too, otherwise open forms
	// break on the first save after a deploy.
	form := url.Values{"site_name": {"After restart"}, "featured_limit": {"3"}, "csrf_token": {csrf}}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, adminRequest("POST", "/admin/settings", form, cookies))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save after restart = %d, want 303", response.Code)
	}

	var stored int
	if err := restarted.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash = ?`, tokenHash(session)).Scan(&stored); err != nil || stored != 1 {
		t.Fatalf("persisted session row = %d, error %v, want 1", stored, err)
	}
}

// TestExpiredSessionRejected forces the stored expiry into the past and expects
// the admin page to redirect to login.
func TestExpiredSessionRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expired.db")
	cfg := sessionTestConfig()
	database, err := db.Init(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	handler := server.New(cfg, database, webFS)

	cookies := adminLogin(t, handler, cfg)
	session := cookieNamed(cookies, "plink_session").Value
	if _, err := database.Exec(`UPDATE sessions SET expires_at = ? WHERE token_hash = ?`, time.Now().Add(-time.Minute).Unix(), tokenHash(session)); err != nil {
		t.Fatal(err)
	}

	response := adminGet(t, handler, cookies, "/admin")
	if response.Code != http.StatusFound || !strings.HasSuffix(response.Header().Get("Location"), "/admin/login") {
		t.Fatalf("expired session = %d %q, want redirect to login", response.Code, response.Header().Get("Location"))
	}
}

// TestLogoutDeletesPersistedSession expects logout to remove the row, not just
// clear the cookies.
func TestLogoutDeletesPersistedSession(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	cookies := adminLogin(t, handler, cfg)
	session := cookieNamed(cookies, "plink_session").Value
	csrf := cookieNamed(cookies, "plink_csrf").Value

	request := adminRequest("POST", "/admin/logout", url.Values{}, cookies)
	request.Header.Set("X-CSRF-Token", csrf)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("logout = %d, want 302", response.Code)
	}

	var stored int
	if err := database.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash = ?`, tokenHash(session)).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("session rows after logout = %d, error %v, want 0", stored, err)
	}
	if response := adminGet(t, handler, cookies, "/admin"); response.Code != http.StatusFound {
		t.Fatalf("admin page with logged-out cookie = %d, want redirect", response.Code)
	}
}
