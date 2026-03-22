package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const csrfCookieName = "plink_csrf"
const csrfHeaderName = "X-CSRF-Token"
const csrfFieldName = "csrf_token"

func generateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) setCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    generateCSRFToken(),
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: false, // must be readable by JS to include in requests
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

// requireCSRF validates the double-submit CSRF token on state-changing requests.
// The client must send the plink_csrf cookie value in the X-CSRF-Token header
// (htmx) or csrf_token form field (traditional forms).
func (s *Server) requireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		token := r.Header.Get(csrfHeaderName)
		if token == "" {
			token = r.FormValue(csrfFieldName)
		}
		if token == "" || token != cookie.Value {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
