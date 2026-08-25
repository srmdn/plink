package server

import (
	"fmt"
	"html"
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
	Links            []db.Link
	Categories       []string
	Query            string
	Category         string
	Status           string
	Date             string
	DateLabel        string
	Count            int
	CountLabel       string
	Total            int
	ActiveCount      int
	PausedCount      int
	TotalClicks      int64
	ClicksLabel      string
	DailyExportDate  string
	DailyExportLabel string
	AdminPath        string
	PublicURL        string
	Production       bool
	OOB              bool
}

type linkFormData struct {
	Link           *db.Link
	Categories     []string
	Error          string
	Query          string
	CategoryFilter string
	StatusFilter   string
	DateFilter     string
	AdminPath      string
}

type analyticsData struct {
	Slug            string
	ShortURL        string
	Total           int64
	Last30d         int64
	LastClick       string
	LastClickTitle  string
	DirectTraffic   string
	HasRecentClicks bool
	ChartSVG        template.HTML
	Referrers       []db.Referrer
}

type overviewAnalyticsData struct {
	Total         int64
	Last30d       int64
	DirectTraffic string
	Referrers     []db.SourceSummary
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
	data, err := s.dashboardDataForRequest(r, links)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	data.PublicURL = publicBaseURL(s.cfg.PublicURL, r)
	data.OOB = true
	s.renderTemplate(w, "links-section", data)
}

func dashboardFilters(r *http.Request) (q, cat, status string) {
	q = r.FormValue("q")
	cat = r.FormValue("category")
	status = r.FormValue("status")
	statusSet := false
	if _, ok := r.Form["status"]; ok {
		statusSet = true
	}
	if _, ok := r.Form["filter_q"]; ok {
		q = r.FormValue("filter_q")
	}
	if _, ok := r.Form["filter_category"]; ok {
		cat = r.FormValue("filter_category")
	}
	if _, ok := r.Form["filter_status"]; ok {
		status = r.FormValue("filter_status")
		statusSet = true
	}
	if !statusSet {
		status = "active"
	}
	return q, cat, status
}

const dashboardDateLayout = "2006-01-02"

func parseDashboardDate(value string) (time.Time, bool) {
	return parseDashboardDateIn(value, time.Local)
}

func dashboardDateFilter(r *http.Request) string {
	date := r.FormValue("date")
	if _, ok := r.Form["filter_date"]; ok {
		date = r.FormValue("filter_date")
	}
	day, ok := parseDashboardDate(date)
	if !ok {
		return ""
	}
	return day.Format(dashboardDateLayout)
}

func dashboardDateLabel(date string) string {
	day, ok := parseDashboardDate(date)
	if !ok {
		return ""
	}
	return day.Format("02 Jan")
}

func (s *Server) dashboardDataForRequest(r *http.Request, links []db.Link) (dashboardData, error) {
	q, cat, status := dashboardFilters(r)
	date := dashboardDateFilter(r)
	location := s.cfg.ReportLocation()
	var daily map[int64]int64
	if date != "" {
		day, _ := parseDashboardDateIn(date, location)
		var err error
		daily, err = s.db.ClickCountsBetween(day.Unix(), day.AddDate(0, 0, 1).Unix())
		if err != nil {
			return dashboardData{}, err
		}
	}
	return buildDashboardDataForDateIn(links, q, cat, status, date, daily, location, s.cfg.AdminPath, s.cfg.Production), nil
}

func buildDashboardData(links []db.Link, q, cat, status, adminPath string, production bool) dashboardData {
	return buildDashboardDataForDate(links, q, cat, status, "", nil, adminPath, production)
}

func buildDashboardDataForDate(links []db.Link, q, cat, status, date string, daily map[int64]int64, adminPath string, production bool) dashboardData {
	return buildDashboardDataForDateIn(links, q, cat, status, date, daily, time.Local, adminPath, production)
}

func buildDashboardDataForDateIn(links []db.Link, q, cat, status, date string, daily map[int64]int64, location *time.Location, adminPath string, production bool) dashboardData {
	location = dashboardLocation(location)
	categories := extractCategories(links)
	displayLinks := links
	if date != "" {
		displayLinks = make([]db.Link, 0, len(links))
		for _, link := range links {
			link.Clicks = daily[link.ID]
			if link.Clicks > 0 {
				displayLinks = append(displayLinks, link)
			}
		}
		sort.SliceStable(displayLinks, func(i, j int) bool {
			if displayLinks[i].Clicks != displayLinks[j].Clicks {
				return displayLinks[i].Clicks > displayLinks[j].Clicks
			}
			return displayLinks[i].Slug < displayLinks[j].Slug
		})
	}
	filtered := filterLinks(displayLinks, q, cat, status)
	var totalClicks int64
	activeCount, pausedCount := 0, 0
	for _, link := range links {
		if date == "" {
			totalClicks += link.Clicks
		} else {
			totalClicks += daily[link.ID]
		}
		if link.Active {
			activeCount++
		} else {
			pausedCount++
		}
	}
	countLabel := "visible links"
	if q != "" || cat != "" {
		countLabel = "matching links"
	} else if date != "" {
		countLabel = "clicked links"
	} else if status == "active" {
		countLabel = "active links"
	} else if status == "paused" {
		countLabel = "paused links"
	}
	dateLabel := dashboardDateLabel(date)
	clicksLabel := "total clicks"
	if dateLabel != "" {
		clicksLabel = "clicks · " + dateLabel
	}
	exportDate := date
	if exportDate == "" {
		exportDate = time.Now().In(location).Format(dashboardDateLayout)
	}
	return dashboardData{
		Links:            filtered,
		Categories:       categories,
		Query:            q,
		Category:         cat,
		Status:           status,
		Date:             date,
		DateLabel:        dateLabel,
		Count:            len(filtered),
		CountLabel:       countLabel,
		Total:            len(links),
		ActiveCount:      activeCount,
		PausedCount:      pausedCount,
		TotalClicks:      totalClicks,
		ClicksLabel:      clicksLabel,
		DailyExportDate:  exportDate,
		DailyExportLabel: "daily clicks · " + dashboardDateLabel(exportDate),
		AdminPath:        adminPath,
		Production:       production,
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

func filterLinks(links []db.Link, q, cat, status string) []db.Link {
	if q == "" && cat == "" && status == "" {
		return links
	}
	q = strings.ToLower(q)
	var result []db.Link
	for _, l := range links {
		if status == "active" && !l.Active {
			continue
		}
		if status == "paused" && l.Active {
			continue
		}
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
	return fillDaysIn(data, days, time.Local)
}

func fillDaysIn(data []db.DailyClicks, days int, location *time.Location) []dailyFill {
	m := make(map[string]int64)
	for _, d := range data {
		m[d.Date] = d.Clicks
	}
	result := make([]dailyFill, days)
	now := time.Now().In(dashboardLocation(location))
	for i := 0; i < days; i++ {
		t := now.AddDate(0, 0, -(days - 1 - i))
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
		label := d.Date
		if parsed, err := time.Parse("2006-01-02", d.Date); err == nil {
			label = parsed.Format("Jan 02")
		}
		ariaLabel := html.EscapeString(fmt.Sprintf("%s, %d clicks", label, d.Clicks))
		safeLabel := html.EscapeString(label)
		fmt.Fprintf(&bars, `<g class="chart-day" tabindex="0" focusable="true" role="img" aria-label="%s"><title>%s — %d clicks</title><rect class="chart-hit" x="%.2f%%" y="0" width="%.2f%%" height="60" fill="transparent"/><rect class="chart-bar" x="%.2f%%" y="%.1f" width="%.2f%%" height="%.1f" fill="#22c55e" opacity="0.75" rx="1"/></g>`, ariaLabel, safeLabel, d.Clicks, x, w, x, 60-h, w, h)
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

func publicBaseURL(configured string, r *http.Request) string {
	if configured = strings.TrimRight(strings.TrimSpace(configured), "/"); configured != "" {
		return configured
	}
	return baseURL(r)
}

func formatAnalyticsTimestamp(unix int64) string {
	return formatAnalyticsTimestampIn(unix, time.Local)
}

func formatAnalyticsTimestampIn(unix int64, location *time.Location) string {
	if unix <= 0 {
		return "—"
	}
	return time.Unix(unix, 0).In(dashboardLocation(location)).Format("02 Jan 2006, 15:04")
}

func formatAnalyticsLastClick(unix int64) (string, string) {
	return formatAnalyticsLastClickIn(unix, time.Local)
}

func formatAnalyticsLastClickIn(unix int64, location *time.Location) (string, string) {
	if unix <= 0 {
		return "—", "no clicks yet"
	}

	location = dashboardLocation(location)
	clickedAt := time.Unix(unix, 0).In(location)
	full := formatAnalyticsTimestampIn(unix, location)
	now := time.Now().In(location)
	if clickedAt.Year() == now.Year() && clickedAt.YearDay() == now.YearDay() {
		return "Today " + clickedAt.Format("15:04"), full
	}
	yesterday := now.AddDate(0, 0, -1)
	if clickedAt.Year() == yesterday.Year() && clickedAt.YearDay() == yesterday.YearDay() {
		return "Yesterday", full
	}
	return clickedAt.Format("Jan 02"), full
}

func percentOfLabel(val, total int64) string {
	if total <= 0 {
		return "0%"
	}
	if val > 0 && val*100 < total {
		return "<1%"
	}
	return fmt.Sprintf("%d%%", val*100/total)
}

func directTrafficLabel(referrers []db.Referrer, total int64) string {
	if total <= 0 {
		return "—"
	}
	for _, referrer := range referrers {
		if referrer.Source == "direct" {
			return percentOfLabel(referrer.Clicks, total)
		}
	}
	return "0%"
}

func directTrafficSummaryLabel(referrers []db.SourceSummary, total int64) string {
	if total <= 0 {
		return "—"
	}
	for _, referrer := range referrers {
		if referrer.Source == "direct" {
			return percentOfLabel(referrer.Clicks, total)
		}
	}
	return "0%"
}

func referrerLabel(source string) string {
	if strings.EqualFold(source, "direct") || source == "" {
		return "Direct / no referrer"
	}
	return source
}

func parseDashboardDateIn(value string, location *time.Location) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	day, err := time.ParseInLocation(dashboardDateLayout, value, dashboardLocation(location))
	if err != nil || day.Format(dashboardDateLayout) != value {
		return time.Time{}, false
	}
	return day, true
}

func dashboardLocation(location *time.Location) *time.Location {
	if location == nil {
		return time.UTC
	}
	return location
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
	data, err := s.dashboardDataForRequest(r, links)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	data.PublicURL = publicBaseURL(s.cfg.PublicURL, r)
	s.renderTemplate(w, "dashboard", data)
}

// ── Partial handlers (htmx) ──────────────────────────────────────────────────

func (s *Server) handleLinksSection(w http.ResponseWriter, r *http.Request) {
	s.serveLinksSection(w, r)
}

func (s *Server) handleNewLinkForm(w http.ResponseWriter, r *http.Request) {
	links, _ := s.db.ListLinks()
	q, cat, status := dashboardFilters(r)
	date := dashboardDateFilter(r)
	s.renderTemplate(w, "link-form", linkFormData{
		Categories: extractCategories(links), Query: q, CategoryFilter: cat, StatusFilter: status, DateFilter: date,
		AdminPath: s.cfg.AdminPath,
	})
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
	q, cat, status := dashboardFilters(r)
	date := dashboardDateFilter(r)
	s.renderTemplate(w, "link-form", linkFormData{
		Link: link, Categories: cats, Query: q, CategoryFilter: cat, StatusFilter: status, DateFilter: date,
		AdminPath: s.cfg.AdminPath,
	})
}

func parseLinkOptions(r *http.Request) (db.LinkOptions, error) {
	priority := 0
	priorityValue := strings.TrimSpace(r.FormValue("priority"))
	if priorityValue != "" {
		parsed, err := strconv.Atoi(priorityValue)
		if err != nil || parsed < 0 {
			return db.LinkOptions{}, fmt.Errorf("priority must be a non-negative number")
		}
		priority = parsed
	}
	featured := r.FormValue("featured") == "1" || r.FormValue("featured") == "on" || r.FormValue("featured") == "true"
	return db.LinkOptions{Featured: featured, Priority: priority}, nil
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
	options, optionsErr := parseLinkOptions(r)
	q, filterCat, filterStatus := dashboardFilters(r)
	date := dashboardDateFilter(r)

	links, _ := s.db.ListLinks()
	cats := extractCategories(links)

	formErr := func(msg string, link *db.Link) {
		w.Header().Set("HX-Retarget", "#modal-body")
		w.Header().Set("HX-Reswap", "innerHTML")
		s.renderTemplate(w, "link-form", linkFormData{
			Link: link, Categories: cats, Error: msg, Query: q, CategoryFilter: filterCat,
			StatusFilter: filterStatus, DateFilter: date, AdminPath: s.cfg.AdminPath,
		})
	}

	if slug == "" || url == "" {
		formErr("slug and url are required", nil)
		return
	}
	if reservedSlugs[slug] || slug == s.cfg.AdminPath {
		formErr("slug is reserved", &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Featured: options.Featured, Priority: options.Priority})
		return
	}
	if optionsErr != nil {
		formErr(optionsErr.Error(), &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Featured: options.Featured, Priority: options.Priority})
		return
	}

	if _, err := s.db.CreateLinkWithOptions(slug, url, desc, cat, options); err != nil {
		msg := "failed to create link"
		if strings.Contains(err.Error(), "UNIQUE") {
			msg = "slug already exists"
		}
		formErr(msg, &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Featured: options.Featured, Priority: options.Priority})
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
	options, optionsErr := parseLinkOptions(r)
	q, filterCat, filterStatus := dashboardFilters(r)
	date := dashboardDateFilter(r)

	links, _ := s.db.ListLinks()
	cats := extractCategories(links)

	formErr := func(msg string) {
		w.Header().Set("HX-Retarget", "#modal-body")
		w.Header().Set("HX-Reswap", "innerHTML")
		s.renderTemplate(w, "link-form", linkFormData{
			Link:       &db.Link{ID: id, Slug: slug, URL: url, Description: desc, Category: cat, Featured: options.Featured, Priority: options.Priority},
			Categories: cats, Error: msg, Query: q, CategoryFilter: filterCat,
			StatusFilter: filterStatus, DateFilter: date, AdminPath: s.cfg.AdminPath,
		})
	}

	if slug == "" || url == "" {
		formErr("slug and url are required")
		return
	}

	if optionsErr != nil {
		formErr(optionsErr.Error())
		return
	}

	if err := s.db.UpdateLinkWithOptions(id, slug, url, desc, cat, options); err != nil {
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

	location := s.cfg.ReportLocation()
	data, err := s.db.GetAnalyticsInLocation(id, location)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	daily := fillDaysIn(data.Daily, 30, location)
	var last30d int64
	for _, d := range daily {
		last30d += d.Clicks
	}

	lastClick, lastClickTitle := formatAnalyticsLastClickIn(data.LastClickAt, location)

	s.renderTemplate(w, "analytics", analyticsData{
		Slug:            slug,
		ShortURL:        publicBaseURL(s.cfg.PublicURL, r) + "/" + slug,
		Total:           data.TotalClicks,
		Last30d:         last30d,
		LastClick:       lastClick,
		LastClickTitle:  lastClickTitle,
		DirectTraffic:   directTrafficLabel(data.Referrers, data.TotalClicks),
		HasRecentClicks: last30d > 0,
		ChartSVG:        buildChartSVG(daily),
		Referrers:       data.Referrers,
	})
}

func (s *Server) handleOverviewAnalyticsUI(w http.ResponseWriter, r *http.Request) {
	data, err := s.db.GetOverviewAnalyticsInLocation(s.cfg.ReportLocation())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	s.renderTemplate(w, "overview-analytics", overviewAnalyticsData{
		Total:         data.TotalClicks,
		Last30d:       data.Last30d,
		DirectTraffic: directTrafficSummaryLabel(data.Referrers, data.TotalClicks),
		Referrers:     data.Referrers,
	})
}
