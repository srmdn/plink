package server

// Dashboard link data and the admin home page.

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/srmdn/plink/internal/db"
)

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
	Page             int
	Pages            int
	PageLinks        []catalogPageLink
}

// linksPerPage keeps the admin link list scannable; the whole catalog is still
// loaded so counts and filters keep working across pages.
const linksPerPage = 25

func dashboardPageNumber(r *http.Request) int {
	page, _ := strconv.Atoi(r.FormValue("page"))
	if page < 1 {
		return 1
	}
	return page
}

// paginateLinks slices the filtered list and builds the page links. The page
// number is clamped to the last page so a deleted row cannot strand the
// visitor on an empty page.
func (s *Server) paginateLinks(r *http.Request, data *dashboardData) {
	total := len(data.Links)
	pages := (total + linksPerPage - 1) / linksPerPage
	if pages < 1 {
		pages = 1
	}
	page := dashboardPageNumber(r)
	if page > pages {
		page = pages
	}
	start := (page - 1) * linksPerPage
	end := start + linksPerPage
	if end > total {
		end = total
	}
	data.Links = data.Links[start:end]
	data.Page = page
	data.Pages = pages
	if pages < 2 {
		return
	}
	data.PageLinks = make([]catalogPageLink, 0, pages)
	for n := 1; n <= pages; n++ {
		data.PageLinks = append(data.PageLinks, catalogPageLink{
			Number:  n,
			URL:     s.dashboardURLForPage(r, n),
			Current: n == page,
		})
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
	data.PublicURL = s.publicURL(r)
	data.OOB = true
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Push-Url", s.dashboardURL(r))
	}
	s.renderTemplate(w, "links-section", data)
}

func (s *Server) dashboardURL(r *http.Request) string {
	return s.dashboardURLForPage(r, dashboardPageNumber(r))
}

func (s *Server) dashboardURLForPage(r *http.Request, page int) string {
	q, category, status := dashboardFilters(r)
	date := dashboardDateFilter(r)
	values := make(url.Values)
	if q != "" {
		values.Set("q", q)
	}
	if category != "" {
		values.Set("category", category)
	}
	if status != "active" {
		values.Set("status", status)
	}
	if date != "" {
		values.Set("date", date)
	}
	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	path := "/" + s.cfg.AdminPath
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
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
	data := buildDashboardDataForDateIn(links, q, cat, status, date, daily, location, s.cfg.AdminPath, s.cfg.Production)
	s.paginateLinks(r, &data)
	return data, nil
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
	countLabel := "Visible links"
	if q != "" || cat != "" {
		countLabel = "Matching links"
	} else if date != "" {
		countLabel = "Clicked links"
	} else if status == "active" {
		countLabel = "Active links"
	} else if status == "paused" {
		countLabel = "Paused links"
	}
	dateLabel := dashboardDateLabel(date)
	clicksLabel := "Total clicks"
	if dateLabel != "" {
		clicksLabel = "Clicks · " + dateLabel
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
		DailyExportLabel: "Daily clicks · " + dashboardDateLabel(exportDate),
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
				!strings.Contains(strings.ToLower(l.Category), q) &&
				!strings.Contains(strings.ToLower(l.Provider), q) &&
				!strings.Contains(strings.ToLower(l.Channel), q) &&
				!strings.Contains(strings.ToLower(l.Campaign), q) {
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
	data.PublicURL = s.publicURL(r)
	s.renderTemplate(w, "dashboard", data)
}
