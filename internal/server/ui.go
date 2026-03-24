package server

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/srmdn/plink/internal/db"
)

// ── Data types ──────────────────────────────────────────────────────────────

type loginData struct {
	Error      bool
	AdminPath  string
	Production bool
}

type dashboardData struct {
	Links      []db.Link
	Categories []string
	Query      string
	Category   string
	Count      int
	Total      int
	AdminPath  string
	Production bool
}

type linkFormData struct {
	Link       *db.Link
	Categories []string
	Error      string
	AdminPath  string
}

type analyticsData struct {
	Slug      string
	ShortURL  string
	Total     int64
	Last30d   int64
	TopSource string
	ChartSVG  template.HTML
	Referrers []db.Referrer
}

type dailyFill struct {
	Date   string
	Clicks int64
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) serveLinksSection(w http.ResponseWriter, r *http.Request) {
	links, err := s.db.ListLinks()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	q := r.FormValue("q")
	cat := r.FormValue("category")
	s.renderTemplate(w, "links-section", buildDashboardData(links, q, cat, s.cfg.AdminPath, s.cfg.Production))
}

func buildDashboardData(links []db.Link, q, cat, adminPath string, production bool) dashboardData {
	categories := extractCategories(links)
	filtered := filterLinks(links, q, cat)
	return dashboardData{
		Links:      filtered,
		Categories: categories,
		Query:      q,
		Category:   cat,
		Count:      len(filtered),
		Total:      len(links),
		AdminPath:  adminPath,
		Production: production,
	}
}

func extractCategories(links []db.Link) []string {
	seen := make(map[string]bool)
	var cats []string
	for _, l := range links {
		if l.Category != "" && !seen[l.Category] {
			seen[l.Category] = true
			cats = append(cats, l.Category)
		}
	}
	sort.Strings(cats)
	return cats
}

func filterLinks(links []db.Link, q, cat string) []db.Link {
	if q == "" && cat == "" {
		return links
	}
	q = strings.ToLower(q)
	var result []db.Link
	for _, l := range links {
		if cat != "" && l.Category != cat {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(l.Slug), q) &&
				!strings.Contains(strings.ToLower(l.URL), q) &&
				!strings.Contains(strings.ToLower(l.Description), q) &&
				!strings.Contains(strings.ToLower(l.Category), q) {
				continue
			}
		}
		result = append(result, l)
	}
	return result
}

func fillDays(data []db.DailyClicks, days int) []dailyFill {
	m := make(map[string]int64)
	for _, d := range data {
		m[d.Date] = d.Clicks
	}
	result := make([]dailyFill, days)
	now := time.Now()
	for i := 0; i < days; i++ {
		t := now.AddDate(0, 0, -(days-1-i))
		key := t.Format("2006-01-02")
		result[i] = dailyFill{Date: key, Clicks: m[key]}
	}
	return result
}

func buildChartSVG(daily []dailyFill) template.HTML {
	n := len(daily)
	if n == 0 {
		return ""
	}
	max := int64(1)
	for _, d := range daily {
		if d.Clicks > max {
			max = d.Clicks
		}
	}
	var bars, lbls strings.Builder
	barW := 100.0 / float64(n)
	for i, d := range daily {
		h := 60.0 * float64(d.Clicks) / float64(max)
		x := float64(i) * barW
		w := barW - 0.4
		fmt.Fprintf(&bars, `<rect x="%.2f%%" y="%.1f" width="%.2f%%" height="%.1f" fill="#22c55e" opacity="0.75" rx="1"/>`, x, 60-h, w, h)
	}
	for _, i := range []int{0, n / 2, n - 1} {
		x := (float64(i) + 0.5) / float64(n) * 100
		d := daily[i].Date
		if len(d) >= 10 {
			d = d[5:]
		}
		fmt.Fprintf(&lbls, `<text x="%.1f%%" y="75" text-anchor="middle" fill="#737373" font-size="9" font-family="monospace">%s</text>`, x, d)
	}
	return template.HTML(fmt.Sprintf(
		`<svg class="chart-svg" viewBox="0 0 400 80" height="80">%s%s</svg>`,
		bars.String(), lbls.String(),
	))
}

func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// ── Page handlers ────────────────────────────────────────────────────────────

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil && s.sessions.valid(cookie.Value) {
		http.Redirect(w, r, "/"+s.cfg.AdminPath, http.StatusFound)
		return
	}
	s.renderTemplate(w, "login", loginData{Error: r.URL.Query().Get("error") == "1", AdminPath: s.cfg.AdminPath, Production: s.cfg.Production})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	links, err := s.db.ListLinks()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	data := buildDashboardData(links, r.URL.Query().Get("q"), r.URL.Query().Get("category"), s.cfg.AdminPath, s.cfg.Production)
	s.renderTemplate(w, "dashboard", data)
}

// ── Partial handlers (htmx) ──────────────────────────────────────────────────

func (s *Server) handleLinksSection(w http.ResponseWriter, r *http.Request) {
	s.serveLinksSection(w, r)
}

func (s *Server) handleNewLinkForm(w http.ResponseWriter, r *http.Request) {
	links, _ := s.db.ListLinks()
	s.renderTemplate(w, "link-form", linkFormData{Categories: extractCategories(links), AdminPath: s.cfg.AdminPath})
}

func (s *Server) handleEditLinkForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	links, _ := s.db.ListLinks()
	cats := extractCategories(links)
	var link *db.Link
	for i := range links {
		if links[i].ID == id {
			l := links[i]
			link = &l
			break
		}
	}
	if link == nil {
		http.NotFound(w, r)
		return
	}
	s.renderTemplate(w, "link-form", linkFormData{Link: link, Categories: cats, AdminPath: s.cfg.AdminPath})
}

func (s *Server) handleCreateLinkUI(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	url := strings.TrimSpace(r.FormValue("url"))
	desc := strings.TrimSpace(r.FormValue("description"))
	cat := strings.TrimSpace(r.FormValue("category"))

	links, _ := s.db.ListLinks()
	cats := extractCategories(links)

	formErr := func(msg string, link *db.Link) {
		w.Header().Set("HX-Retarget", "#modal-body")
		w.Header().Set("HX-Reswap", "innerHTML")
		s.renderTemplate(w, "link-form", linkFormData{Link: link, Categories: cats, Error: msg, AdminPath: s.cfg.AdminPath})
	}

	if slug == "" || url == "" {
		formErr("slug and url are required", nil)
		return
	}
	if reservedSlugs[slug] || slug == s.cfg.AdminPath {
		formErr("slug is reserved", &db.Link{Slug: slug, URL: url, Description: desc, Category: cat})
		return
	}

	if _, err := s.db.CreateLink(slug, url, desc, cat); err != nil {
		msg := "failed to create link"
		if strings.Contains(err.Error(), "UNIQUE") {
			msg = "slug already exists"
		}
		formErr(msg, &db.Link{Slug: slug, URL: url, Description: desc, Category: cat})
		return
	}

	w.Header().Set("HX-Trigger", "closeModal")
	s.serveLinksSection(w, r)
}

func (s *Server) handleUpdateLinkUI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	url := strings.TrimSpace(r.FormValue("url"))
	desc := strings.TrimSpace(r.FormValue("description"))
	cat := strings.TrimSpace(r.FormValue("category"))

	links, _ := s.db.ListLinks()
	cats := extractCategories(links)

	formErr := func(msg string) {
		w.Header().Set("HX-Retarget", "#modal-body")
		w.Header().Set("HX-Reswap", "innerHTML")
		s.renderTemplate(w, "link-form", linkFormData{
			Link:       &db.Link{ID: id, Slug: slug, URL: url, Description: desc, Category: cat},
			Categories: cats,
			Error:      msg,
			AdminPath:  s.cfg.AdminPath,
		})
	}

	if slug == "" || url == "" {
		formErr("slug and url are required")
		return
	}

	if err := s.db.UpdateLink(id, slug, url, desc, cat); err != nil {
		msg := "failed to update link"
		if strings.Contains(err.Error(), "UNIQUE") {
			msg = "slug already exists"
		}
		formErr(msg)
		return
	}

	w.Header().Set("HX-Trigger", "closeModal")
	s.serveLinksSection(w, r)
}

func (s *Server) handleDeleteLinkUI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteLink(id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.serveLinksSection(w, r)
}

func (s *Server) handleToggleLinkUI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.db.ToggleLink(id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.serveLinksSection(w, r)
}

func (s *Server) handleAnalyticsUI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	links, _ := s.db.ListLinks()
	var slug string
	for _, l := range links {
		if l.ID == id {
			slug = l.Slug
			break
		}
	}

	data, err := s.db.GetAnalytics(id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	daily := fillDays(data.Daily, 30)
	var last30d int64
	for _, d := range daily {
		last30d += d.Clicks
	}

	topSource := "—"
	if len(data.Referrers) > 0 {
		topSource = data.Referrers[0].Source
	}

	s.renderTemplate(w, "analytics", analyticsData{
		Slug:      slug,
		ShortURL:  baseURL(r) + "/" + slug,
		Total:     data.TotalClicks,
		Last30d:   last30d,
		TopSource: topSource,
		ChartSVG:  buildChartSVG(daily),
		Referrers: data.Referrers,
	})
}
