package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

const cookieName = "plink_session"
const sessionTTL = 24 * time.Hour

// sessionStore keeps admin sessions in SQLite so a restart or deploy does not
// log the operator out mid-work. Only hashes of the session token and of the
// CSRF token bound to it are stored; the raw values live in cookies only.
type sessionStore struct {
	db *db.DB
}

func newSessionStore(database *db.DB) *sessionStore {
	s := &sessionStore{db: database}
	go s.cleanup()
	return s
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// create starts a session and returns its cookie token plus the CSRF token
// bound to it. Both are generated once, at login, and stay stable for the
// lifetime of the session.
func (s *sessionStore) create() (string, string, error) {
	token, csrf := randomToken(), randomToken()
	if err := s.db.CreateSession(hashToken(token), hashToken(csrf), time.Now().Add(sessionTTL)); err != nil {
		return "", "", err
	}
	return token, csrf, nil
}

func (s *sessionStore) valid(token string) bool {
	if token == "" {
		return false
	}
	ok, err := s.db.SessionValid(hashToken(token), time.Now())
	return err == nil && ok
}

// csrfValid reports whether the submitted CSRF token matches the one bound to
// the session. Requests without a live session (dev no-auth mode, or a cookie
// that already expired) pass here; requireAuth is the authority on auth, and
// the double-submit comparison still applies to them.
func (s *sessionStore) csrfValid(sessionToken, csrfToken string) bool {
	if sessionToken == "" || csrfToken == "" {
		return true
	}
	stored, found, err := s.db.SessionCSRFHash(hashToken(sessionToken), time.Now())
	if err != nil {
		return false
	}
	if !found {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(hashToken(csrfToken))) == 1
}

// rotateCSRF issues a replacement CSRF token for a live session. It is only
// used when the browser has no usable token (missing or not bound to the
// session), so a valid token is never invalidated while another tab has a form
// open.
func (s *sessionStore) rotateCSRF(sessionToken string) (string, error) {
	csrf := randomToken()
	err := s.db.UpdateSessionCSRF(hashToken(sessionToken), hashToken(csrf), time.Now())
	return csrf, err
}

func (s *sessionStore) delete(token string) {
	if token == "" {
		return
	}
	if err := s.db.DeleteSession(hashToken(token)); err != nil {
		log.Printf("plink: delete session: %v", err)
	}
}

func (s *sessionStore) cleanup() {
	for range time.Tick(time.Hour) {
		if _, err := s.db.DeleteExpiredSessions(time.Now()); err != nil {
			log.Printf("plink: clean expired sessions: %v", err)
		}
	}
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		cookie, err := r.Cookie(cookieName)
		sessionToken := ""
		if err == nil {
			sessionToken = cookie.Value
		}
		authenticated := s.sessions.valid(sessionToken)
		if !authenticated && !s.allowLocalNoAuth(r) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/"+s.cfg.AdminPath+"/login", http.StatusFound)
			return
		}
		// The CSRF token is stable per session, so it is issued only when the
		// browser does not have one yet. Rotating it on every page load would
		// invalidate forms already rendered in another tab.
		if r.Method == http.MethodGet {
			s.ensureCSRFCookie(w, r, sessionToken)
		}
		next(w, r)
	}
}

// ensureCSRFCookie gives the browser a CSRF token only when it does not have a
// usable one: no cookie at all, or a cookie that is not bound to this session
// (a stale leftover). A matching token is never rotated, so forms already
// rendered in another tab stay valid. In dev no-auth mode there is no session,
// so the token is double-submit only.
func (s *Server) ensureCSRFCookie(w http.ResponseWriter, r *http.Request, sessionToken string) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && cookie.Value != "" {
		if s.sessions.csrfValid(sessionToken, cookie.Value) {
			return
		}
	}
	value := generateCSRFToken()
	if sessionToken != "" && s.sessions.valid(sessionToken) {
		rotated, err := s.sessions.rotateCSRF(sessionToken)
		if err != nil {
			log.Printf("plink: issue csrf token: %v", err)
			return
		}
		value = rotated
	}
	s.setCSRFCookie(w, value)
}

// A loopback listener can still sit behind a reverse proxy. Reject forwarded
// requests and non-local Host headers; never trust proxy IP headers here.
func (s *Server) allowLocalNoAuth(r *http.Request) bool {
	if s.cfg == nil || !s.cfg.DevNoAuth || s.cfg.Production || !config.IsLoopbackListener(s.cfg.Addr) {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return false
	}
	requestHost := r.Host
	if parsed, _, err := net.SplitHostPort(requestHost); err == nil {
		requestHost = parsed
	}
	requestHost = strings.Trim(requestHost, "[]")
	ip := net.ParseIP(requestHost)
	if !strings.EqualFold(requestHost, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return false
	}
	for _, key := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP", "Via"} {
		if _, present := r.Header[http.CanonicalHeaderKey(key)]; present {
			return false
		}
	}
	return true
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.loginLimiter.allow(ip) {
		http.Error(w, "too many requests — try again later", http.StatusTooManyRequests)
		return
	}

	password := []byte(r.FormValue("password"))
	if subtle.ConstantTimeCompare(password, []byte(s.cfg.AdminPassword)) != 1 {
		log.Printf("plink: failed login attempt from %s", ip)
		http.Redirect(w, r, "/"+s.cfg.AdminPath+"/login?error=1", http.StatusFound)
		return
	}

	s.loginLimiter.reset(ip)
	token, csrfToken, err := s.sessions.create()
	if err != nil {
		log.Printf("plink: create session: %v", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
	s.setCSRFCookie(w, csrfToken)
	http.Redirect(w, r, "/"+s.cfg.AdminPath, http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil {
		s.sessions.delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
	})
	http.SetCookie(w, &http.Cookie{
		Name:   csrfCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/login", http.StatusFound)
}
