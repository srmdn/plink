package server

import (
	"net/http"
	"sort"

	"github.com/srmdn/plink/internal/db"
)

type homeData struct {
	Links      []db.PublicLink
	Categories []string
	Category   string
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

	s.renderTemplate(w, "home", homeData{
		Links:      links,
		Categories: categories,
		Category:   cat,
	})
}
