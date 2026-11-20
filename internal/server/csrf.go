package server

import (
	"crypto/rand"
	"crypto/subtle"
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

func (s *Server) setCSRFCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: false, // must be readable by JS to include in requests
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

// requireCSRF validates the CSRF token on state-changing requests. The client
// must send the plink_csrf cookie value in the X-CSRF-Token header (htmx) or
// csrf_token form field (traditional forms): a double-submit check, plus a
// synchronizer check against the token hash bound to the request's session.
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
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		// A token minted for another session must not be replayed against this
		// one, even when the submitted value matches its own cookie.
		if session, err := r.Cookie(cookieName); err == nil && !s.sessions.csrfValid(session.Value, token) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
