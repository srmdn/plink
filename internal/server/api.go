package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/srmdn/plink/internal/db"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

var reservedSlugs = map[string]bool{
	"admin": true,
	"api":   true,
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	data, _ := s.webFS.ReadFile("web/favicon.svg")
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.db.ListLinks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if links == nil {
		links = []db.Link{}
	}
	writeJSON(w, http.StatusOK, links)
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug        string `json:"slug"`
		URL         string `json:"url"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Featured    *bool  `json:"featured"`
		Priority    *int   `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	body.Slug = strings.TrimSpace(body.Slug)
	body.URL = strings.TrimSpace(body.URL)

	if body.Slug == "" || body.URL == "" {
		writeError(w, http.StatusBadRequest, "slug and url are required")
		return
	}
	if reservedSlugs[body.Slug] {
		writeError(w, http.StatusBadRequest, "slug is reserved")
		return
	}

	options := db.LinkOptions{}
	if body.Featured != nil {
		options.Featured = *body.Featured
	}
	if body.Priority != nil {
		if *body.Priority < 0 {
			writeError(w, http.StatusBadRequest, "priority must be a non-negative number")
			return
		}
		options.Priority = *body.Priority
	}

	link, err := s.db.CreateLinkWithOptions(body.Slug, body.URL, body.Description, body.Category, options)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusCreated, link)
}

func (s *Server) handleUpdateLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body struct {
		Slug        string `json:"slug"`
		URL         string `json:"url"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Featured    *bool  `json:"featured"`
		Priority    *int   `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if body.Priority != nil && *body.Priority < 0 {
		writeError(w, http.StatusBadRequest, "priority must be a non-negative number")
		return
	}

	var updateErr error
	if body.Featured == nil && body.Priority == nil {
		// Preserve the legacy API contract: omitted curation fields do not reset metadata.
		updateErr = s.db.UpdateLink(id, body.Slug, body.URL, body.Description, body.Category)
	} else {
		current, err := s.db.GetLinkByID(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
		if current == nil {
			writeError(w, http.StatusNotFound, "link not found")
			return
		}
		options := db.LinkOptions{Featured: current.Featured, Priority: current.Priority}
		if body.Featured != nil {
			options.Featured = *body.Featured
		}
		if body.Priority != nil {
			options.Priority = *body.Priority
		}
		updateErr = s.db.UpdateLinkWithOptions(id, body.Slug, body.URL, body.Description, body.Category, options)
	}
	if err := updateErr; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.db.DeleteLink(id); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleToggleLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.ToggleLink(id); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	links, err := s.db.ListLinks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=\"plink-export.csv\"")
		cw := csv.NewWriter(w)
		cw.Write([]string{"slug", "url", "description", "category", "active", "featured", "priority", "clicks", "created_at"})
		for _, l := range links {
			active := "1"
			if !l.Active {
				active = "0"
			}
			featured := "0"
			if l.Featured {
				featured = "1"
			}
			cw.Write([]string{l.Slug, l.URL, l.Description, l.Category, active, featured, fmt.Sprintf("%d", l.Priority), fmt.Sprintf("%d", l.Clicks), fmt.Sprintf("%d", l.CreatedAt)})
		}
		cw.Flush()
		return
	}

	// Default: JSON
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"plink-export.json\"")
	json.NewEncoder(w).Encode(links)
}

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	analytics, err := s.db.GetAnalytics(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusOK, analytics)
}

func (s *Server) handleOverviewAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := s.db.GetOverviewAnalytics()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, analytics)
}
