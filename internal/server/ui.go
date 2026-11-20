package server

// Shared admin page scaffolding: template rendering and the login page.

import (
	"net/http"

	"github.com/srmdn/plink/internal/db"
)

type loginData struct {
	Error      bool
	AdminPath  string
	Production bool
}

type settingsDashboardData struct {
	EffectivePublicURL string
	SEO                pageSEO
	Settings           db.SiteSettings
	Social             map[string]string
	ContactList        []contactStored
	AdminPath          string
	Production         bool
	Error              string
	Saved              bool
	ContrastWarning    string
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.isLoggedIn(r) {
		http.Redirect(w, r, "/"+s.cfg.AdminPath, http.StatusFound)
		return
	}
	s.renderTemplate(w, "login", loginData{Error: r.URL.Query().Get("error") == "1", AdminPath: s.cfg.AdminPath, Production: s.cfg.Production})
}
