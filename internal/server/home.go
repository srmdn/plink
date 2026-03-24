package server

import (
	"net/http"
	"sort"

	"github.com/srmdn/plink/internal/db"
)

type homeData struct {
	Links       []db.PublicLink
	Categories  []string
	Category    string
	SiteName    string
	SiteDesc    string
	IsLoggedIn  bool
	AdminPath   string
	Production  bool
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	all, err := s.db.ListPublicLinks()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	cat := r.URL.Query().Get("category")

	// Extract unique categories
	seen := make(map[string]bool)
	var categories []string
	for _, l := range all {
		if l.Category != "" && !seen[l.Category] {
			seen[l.Category] = true
			categories = append(categories, l.Category)
		}
	}
	sort.Strings(categories)

	// Filter by category if requested
	links := all
	if cat != "" {
		links = links[:0]
		for _, l := range all {
			if l.Category == cat {
				links = append(links, l)
			}
		}
	}

	loggedIn := false
	if cookie, err := r.Cookie(cookieName); err == nil && s.sessions.valid(cookie.Value) {
		loggedIn = true
	}

	s.renderTemplate(w, "home", homeData{
		Links:      links,
		Categories: categories,
		Category:   cat,
		SiteName:   s.cfg.SiteName,
		SiteDesc:   s.cfg.SiteDesc,
		IsLoggedIn: loggedIn,
		AdminPath:  s.cfg.AdminPath,
		Production: s.cfg.Production,
	})
}
