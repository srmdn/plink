package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/srmdn/plink/internal/db"
)

type homeData struct {
	Links      []db.PublicLink
	Featured   []db.PublicLink
	Categories []string
	Category   string
	Query      string
	ShowAll    bool
	SiteName   string
	SiteDesc   string
	IsLoggedIn bool
	AdminPath  string
	Production bool
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	all, err := s.db.ListPublicLinks()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	featured, err := s.db.ListFeaturedLinks(3)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	cat := r.URL.Query().Get("category")
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	showAll := r.URL.Query().Get("all") == "1"

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

	links := make([]db.PublicLink, 0, len(all))
	for _, l := range all {
		if cat != "" && l.Category != cat {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(l.Slug + " " + l.Description + " " + l.Category)
			if !strings.Contains(haystack, strings.ToLower(query)) {
				continue
			}
		}
		links = append(links, l)
	}
	if cat == "" && query == "" && !showAll && len(links) > 6 {
		links = links[:6]
	}

	loggedIn := false
	if cookie, err := r.Cookie(cookieName); err == nil && s.sessions.valid(cookie.Value) {
		loggedIn = true
	}

	s.renderTemplate(w, "home", homeData{
		Links:      links,
		Featured:   featured,
		Categories: categories,
		Category:   cat,
		Query:      query,
		ShowAll:    showAll,
		SiteName:   s.cfg.SiteName,
		SiteDesc:   s.cfg.SiteDesc,
		IsLoggedIn: loggedIn,
		AdminPath:  s.cfg.AdminPath,
		Production: s.cfg.Production,
	})
}
