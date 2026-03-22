package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const cookieName = "plink_session"
const sessionTTL = 24 * time.Hour

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
}

func newSessionStore() *sessionStore {
	s := &sessionStore{sessions: make(map[string]time.Time)}
	go s.cleanup()
	return s
}

func (s *sessionStore) create() string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	s.mu.Lock()
	s.sessions[token] = time.Now().Add(sessionTTL)
	s.mu.Unlock()

	return token
}

func (s *sessionStore) valid(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.sessions, token)
		return false
	}
	return true
}

func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *sessionStore) cleanup() {
	for range time.Tick(time.Hour) {
		s.mu.Lock()
		for token, exp := range s.sessions {
			if time.Now().After(exp) {
				delete(s.sessions, token)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil || !s.sessions.valid(cookie.Value) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		// Refresh CSRF cookie on authenticated page loads
		if r.Method == http.MethodGet {
			s.setCSRFCookie(w)
		}
		next(w, r)
	}
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
		http.Redirect(w, r, "/admin/login?error=1", http.StatusFound)
		return
	}

	s.loginLimiter.reset(ip)
	token := s.sessions.create()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
	s.setCSRFCookie(w)
	http.Redirect(w, r, "/admin", http.StatusFound)
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
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}
