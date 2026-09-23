package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

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
		Provider    string `json:"provider"`
		Channel     string `json:"channel"`
		Campaign    string `json:"campaign"`
		Featured    *bool  `json:"featured"`
		Priority    *int   `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	body.Slug = strings.TrimSpace(body.Slug)
	body.URL = strings.TrimSpace(body.URL)
	body.Provider = strings.TrimSpace(body.Provider)
	body.Channel = strings.TrimSpace(body.Channel)
	body.Campaign = strings.TrimSpace(body.Campaign)

	if body.Slug == "" || body.URL == "" {
		writeError(w, http.StatusBadRequest, "slug and url are required")
		return
	}
	if reservedSlugs[body.Slug] {
		writeError(w, http.StatusBadRequest, "slug is reserved")
		return
	}
	if err := validateLinkMetadata(body.Provider, body.Channel, body.Campaign); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	options := db.LinkOptions{Provider: body.Provider, Channel: body.Channel, Campaign: body.Campaign}
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
		Slug        string  `json:"slug"`
		URL         string  `json:"url"`
		Description string  `json:"description"`
		Category    string  `json:"category"`
		Provider    *string `json:"provider"`
		Channel     *string `json:"channel"`
		Campaign    *string `json:"campaign"`
		Featured    *bool   `json:"featured"`
		Priority    *int    `json:"priority"`
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
	if body.Featured == nil && body.Priority == nil && body.Provider == nil && body.Channel == nil && body.Campaign == nil {
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
		options.Provider = current.Provider
		options.Channel = current.Channel
		options.Campaign = current.Campaign
		if body.Featured != nil {
			options.Featured = *body.Featured
		}
		if body.Priority != nil {
			options.Priority = *body.Priority
		}
		if body.Provider != nil {
			options.Provider = strings.TrimSpace(*body.Provider)
		}
		if body.Channel != nil {
			options.Channel = strings.TrimSpace(*body.Channel)
		}
		if body.Campaign != nil {
			options.Campaign = strings.TrimSpace(*body.Campaign)
		}
		if err := validateLinkMetadata(options.Provider, options.Channel, options.Campaign); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
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
	if r.URL.Query().Get("format") == "daily-csv" {
		s.handleDailyExport(w, r)
		return
	}

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
		cw.Write([]string{"slug", "url", "description", "category", "provider", "channel", "campaign", "active", "featured", "priority", "clicks", "created_at"})
		for _, l := range links {
			active := "1"
			if !l.Active {
				active = "0"
			}
			featured := "0"
			if l.Featured {
				featured = "1"
			}
			cw.Write([]string{l.Slug, l.URL, l.Description, l.Category, l.Provider, l.Channel, l.Campaign, active, featured, fmt.Sprintf("%d", l.Priority), fmt.Sprintf("%d", l.Clicks), fmt.Sprintf("%d", l.CreatedAt)})
		}
		cw.Flush()
		return
	}

	// Default: JSON
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"plink-export.json\"")
	json.NewEncoder(w).Encode(links)
}

func (s *Server) handleDailyExport(w http.ResponseWriter, r *http.Request) {
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date == "" {
		date = time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	}
	day, ok := parseDashboardDateIn(date, s.cfg.ReportLocation())
	if !ok {
		writeError(w, http.StatusBadRequest, "date must use YYYY-MM-DD")
		return
	}

	counts, err := s.db.ClickCountsBetween(day.Unix(), day.AddDate(0, 0, 1).Unix())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	links, err := s.db.ListLinks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	type dailyExportRow struct {
		link   db.Link
		clicks int64
	}
	rows := make([]dailyExportRow, 0, len(counts))
	for _, link := range links {
		if clicks := counts[link.ID]; clicks > 0 {
			rows = append(rows, dailyExportRow{link: link, clicks: clicks})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].clicks != rows[j].clicks {
			return rows[i].clicks > rows[j].clicks
		}
		return rows[i].link.Slug < rows[j].link.Slug
	})

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"plink-daily-clicks-%s.csv\"", date))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"date", "slug", "short_url", "destination", "category", "provider", "channel", "campaign", "clicks"})
	shortBase := publicBaseURL(s.cfg.PublicURL, r)
	for _, row := range rows {
		_ = cw.Write([]string{
			date,
			row.link.Slug,
			shortBase + "/" + row.link.Slug,
			row.link.URL,
			row.link.Category,
			row.link.Provider,
			row.link.Channel,
			row.link.Campaign,
			strconv.FormatInt(row.clicks, 10),
		})
	}
	cw.Flush()
}

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	analytics, err := s.db.GetAnalyticsInLocation(id, s.cfg.ReportLocation())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusOK, analytics)
}

func (s *Server) handleOverviewAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := s.db.GetOverviewAnalyticsInLocation(s.cfg.ReportLocation())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, analytics)
}
