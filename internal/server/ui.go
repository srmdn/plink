package server

import (
	"fmt"
	"html"
	"html/template"
	"math"
	"net/http"
	"net/url"
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
	ID                  int64
	Slug                string
	ShortURL            string
	AnalyticsURL        string
	OverviewURL         string
	Destination         string
	Description         string
	Category            string
	Provider            string
	Channel             string
	Campaign            string
	Total               int64
	Last30d             int64
	LastClick           string
	LastClickTitle      string
	DirectTraffic       string
	DirectTrafficLabel  string
	HasRecentClicks     bool
	ChartSVG            template.HTML
	Referrers           []db.Referrer
	ReferrerTotal       int64
	TrafficSourcesLabel string
	TrafficSourcesEmpty string
}

type overviewAnalyticsData struct {
	Total               int64
	TotalLabel          string
	Last30d             int64
	ShowLast30d         bool
	DirectTraffic       string
	DirectTrafficLabel  string
	Referrers           []db.SourceSummary
	TrafficSourcesLabel string
	TrafficSourcesEmpty string
	AnalyticsURL        string
	DateSelected        bool
}

type analyticsPageLink struct {
	ID             int64
	Slug           string
	Category       string
	Provider       string
	Channel        string
	Campaign       string
	LinkCount      int
	Clicks         int64
	PreviousClicks int64
	ClicksPerLink  string
	TrendLabel     string
	TrendClass     string
}

type analyticsSlice struct {
	Label   string
	Clicks  int64
	Percent int64
	Class   string
}

type analyticsPageData struct {
	AdminPath           string
	Production          bool
	Periods             []analyticsPeriodSpec
	Period              string
	PeriodLabel         string
	Group               string
	GroupLabel          string
	Groups              []analyticsGroupSpec
	Category            string
	Categories          []string
	TotalClicks         int64
	ActiveLinks         int
	TotalLinks          int
	DirectTraffic       string
	TrafficSources      []db.SourceSummary
	TrafficSourcesLabel string
	TopLinks            []analyticsPageLink
	HasTrend            bool
	SelectedLink        *analyticsData
	DonutSlices         []analyticsSlice
	DonutSVG            template.HTML
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
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Push-Url", s.dashboardURL(r))
	}
	s.renderTemplate(w, "links-section", data)
}

func (s *Server) dashboardURL(r *http.Request) string {
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

const defaultAnalyticsPeriod = "30d"

type analyticsPeriodSpec struct {
	Key   string
	Label string
	Days  int
}

type analyticsGroupSpec struct {
	Key   string
	Label string
}

var analyticsPeriods = []analyticsPeriodSpec{
	{Key: "7d", Label: "last 7 days", Days: 7},
	{Key: "30d", Label: "last 30 days", Days: 30},
	{Key: "90d", Label: "last 90 days", Days: 90},
	{Key: "all", Label: "all time"},
}

var analyticsGroups = []analyticsGroupSpec{
	{Key: "link", Label: "by link"},
	{Key: "provider", Label: "by provider"},
	{Key: "channel", Label: "by channel"},
	{Key: "campaign", Label: "by campaign"},
}

func analyticsPeriodSpecFor(key string) analyticsPeriodSpec {
	for _, period := range analyticsPeriods {
		if period.Key == key {
			return period
		}
	}
	return analyticsPeriodSpecFor(defaultAnalyticsPeriod)
}

func analyticsPeriodFromRequest(r *http.Request) string {
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		period = defaultAnalyticsPeriod
	}
	return analyticsPeriodSpecFor(period).Key
}

func analyticsGroupSpecFor(key string) analyticsGroupSpec {
	for _, group := range analyticsGroups {
		if group.Key == key {
			return group
		}
	}
	return analyticsGroups[0]
}

func analyticsGroupFromRequest(r *http.Request) string {
	return analyticsGroupSpecFor(strings.TrimSpace(r.URL.Query().Get("group"))).Key
}

func analyticsRange(period string, now time.Time) (time.Time, time.Time, bool) {
	spec := analyticsPeriodSpecFor(period)
	if spec.Days == 0 {
		return time.Time{}, time.Time{}, false
	}
	return now.AddDate(0, 0, -spec.Days), now, true
}

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

func buildChartSVG(daily []dailyFill) template.HTML {
	return buildChartSVGWithMode(daily, true)
}

func buildChartSVGWithMode(daily []dailyFill, interactive bool) template.HTML {
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
		if interactive {
			fmt.Fprintf(&bars, `<g class="chart-day" tabindex="0" focusable="true" role="button" aria-label="Filter report to %s" data-date="%s" onclick="selectReportDate(this.dataset.date)" onkeydown="if(event.key==='Enter'||event.key===' '){event.preventDefault();selectReportDate(this.dataset.date)}"><title>%s — %d clicks. Click to filter the report.</title><rect class="chart-hit" x="%.2f%%" y="0" width="%.2f%%" height="60" fill="transparent"/><rect class="chart-bar" x="%.2f%%" y="%.1f" width="%.2f%%" height="%.1f" fill="#22c55e" opacity="0.75" rx="1"/></g>`, ariaLabel, d.Date, safeLabel, d.Clicks, x, w, x, 60-h, w, h)
		} else {
			fmt.Fprintf(&bars, `<g class="chart-day chart-day-static"><title>%s — %d clicks.</title><rect class="chart-bar" x="%.2f%%" y="%.1f" width="%.2f%%" height="%.1f" fill="#22c55e" opacity="0.75" rx="1"/></g>`, safeLabel, d.Clicks, x, 60-h, w, h)
		}
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

func buildAnalyticsPageData(links []db.Link, clickCounts map[int64]int64, overview *db.OverviewAnalytics, period, category string) analyticsPageData {
	return buildAnalyticsPageDataGrouped(links, clickCounts, overview, period, category, "link")
}

func buildAnalyticsPageDataGrouped(links []db.Link, clickCounts map[int64]int64, overview *db.OverviewAnalytics, period, category, group string) analyticsPageData {
	return buildAnalyticsPageDataWithPrevious(links, clickCounts, nil, overview, period, category, group)
}

func buildAnalyticsPageDataWithPrevious(links []db.Link, clickCounts, previousClickCounts map[int64]int64, overview *db.OverviewAnalytics, period, category, group string) analyticsPageData {
	spec := analyticsPeriodSpecFor(period)
	groupSpec := analyticsGroupSpecFor(group)
	categories := extractCategories(links)
	linkRows := make([]analyticsPageLink, 0, len(links))
	var totalClicks int64
	activeLinks, totalLinks := 0, 0

	for _, link := range links {
		if category != "" && link.Category != category {
			continue
		}
		totalLinks++
		if !link.Active {
			continue
		}
		activeLinks++
		clicks := link.Clicks
		if clickCounts != nil {
			clicks = clickCounts[link.ID]
		}
		previousClicks := int64(0)
		if previousClickCounts != nil {
			previousClicks = previousClickCounts[link.ID]
		}
		totalClicks += clicks
		linkRows = append(linkRows, analyticsPageLink{
			ID: link.ID, Slug: link.Slug, Category: link.Category,
			Provider: link.Provider, Channel: link.Channel, Campaign: link.Campaign,
			LinkCount: 1, Clicks: clicks, PreviousClicks: previousClicks,
		})
	}
	filtered := aggregateAnalyticsRows(linkRows, groupSpec.Key)
	decorateAnalyticsRows(filtered, previousClickCounts != nil)
	filtered = filterAnalyticsRowsWithClicks(filtered)

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Clicks != filtered[j].Clicks {
			return filtered[i].Clicks > filtered[j].Clicks
		}
		return filtered[i].Slug < filtered[j].Slug
	})
	topLinks := filtered
	if len(topLinks) > 8 {
		topLinks = topLinks[:8]
	}
	data := analyticsPageData{
		Period:              spec.Key,
		Periods:             analyticsPeriods,
		PeriodLabel:         spec.Label,
		Group:               groupSpec.Key,
		GroupLabel:          groupSpec.Label,
		Groups:              analyticsGroups,
		Category:            category,
		Categories:          categories,
		TotalClicks:         totalClicks,
		ActiveLinks:         activeLinks,
		TotalLinks:          totalLinks,
		DirectTraffic:       directTrafficSummaryLabel(overview.Referrers, overview.TotalClicks),
		TrafficSources:      overview.Referrers,
		TrafficSourcesLabel: spec.Label,
		TopLinks:            topLinks,
		HasTrend:            previousClickCounts != nil,
		DonutSlices:         buildAnalyticsSlices(filtered, totalClicks),
	}
	data.DonutSVG = buildAnalyticsDonutSVG(data.DonutSlices, totalClicks)
	return data
}

func filterAnalyticsRowsWithClicks(rows []analyticsPageLink) []analyticsPageLink {
	filtered := rows[:0]
	for _, row := range rows {
		if row.Clicks > 0 {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func aggregateAnalyticsRows(rows []analyticsPageLink, group string) []analyticsPageLink {
	if group == "link" || len(rows) == 0 {
		return rows
	}
	grouped := make(map[string]analyticsPageLink)
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		key := analyticsGroupValue(row, group)
		if _, ok := grouped[key]; !ok {
			row.ID = 0
			row.Slug = key
			row.Category = ""
			row.LinkCount = 1
			grouped[key] = row
			order = append(order, key)
			continue
		}
		aggregate := grouped[key]
		aggregate.Clicks += row.Clicks
		aggregate.PreviousClicks += row.PreviousClicks
		aggregate.LinkCount++
		grouped[key] = aggregate
	}
	result := make([]analyticsPageLink, 0, len(order))
	for _, key := range order {
		result = append(result, grouped[key])
	}
	return result
}

func decorateAnalyticsRows(rows []analyticsPageLink, comparable bool) {
	for i := range rows {
		row := &rows[i]
		if row.LinkCount > 0 {
			row.ClicksPerLink = fmt.Sprintf("%.1f", float64(row.Clicks)/float64(row.LinkCount))
		} else {
			row.ClicksPerLink = "0.0"
		}
		if !comparable {
			row.TrendLabel = "—"
			row.TrendClass = "neutral"
			continue
		}
		row.TrendLabel, row.TrendClass = analyticsTrendLabel(row.Clicks, row.PreviousClicks)
	}
}

func analyticsTrendLabel(current, previous int64) (string, string) {
	if previous <= 0 {
		if current > 0 {
			return "new", "up"
		}
		return "—", "neutral"
	}
	change := int64(math.Round(float64(current-previous) * 100 / float64(previous)))
	if change > 0 {
		return fmt.Sprintf("+%d%%", change), "up"
	}
	if change < 0 {
		return fmt.Sprintf("%d%%", change), "down"
	}
	return "0%", "flat"
}

func analyticsGroupValue(row analyticsPageLink, group string) string {
	value := ""
	switch group {
	case "provider":
		value = row.Provider
	case "channel":
		value = row.Channel
	case "campaign":
		value = row.Campaign
	}
	if strings.TrimSpace(value) == "" {
		return "Unassigned"
	}
	return value
}

func buildAnalyticsSlices(links []analyticsPageLink, total int64) []analyticsSlice {
	if total <= 0 || len(links) == 0 {
		return nil
	}
	const maxSlices = 5
	limit := len(links)
	if limit > maxSlices {
		limit = maxSlices
	}
	result := make([]analyticsSlice, 0, limit+1)
	var used int64
	for i := 0; i < limit; i++ {
		link := links[i]
		used += link.Clicks
		result = append(result, analyticsSlice{
			Label:   link.Slug,
			Clicks:  link.Clicks,
			Percent: link.Clicks * 100 / total,
			Class:   fmt.Sprintf("segment-%d", i+1),
		})
	}
	if len(links) > limit && total > used {
		result = append(result, analyticsSlice{
			Label:   "Other",
			Clicks:  total - used,
			Percent: (total - used) * 100 / total,
			Class:   "segment-other",
		})
	}
	// Keep the visual mathematically closed after integer rounding.
	var percent int64
	for i := range result {
		if i == len(result)-1 {
			result[i].Percent = 100 - percent
		} else {
			percent += result[i].Percent
		}
	}
	return result
}

func buildAnalyticsDonutSVG(slices []analyticsSlice, total int64) template.HTML {
	if len(slices) == 0 || total <= 0 {
		return ""
	}
	var segments strings.Builder
	var labels []string
	var offset int64
	for _, slice := range slices {
		labels = append(labels, fmt.Sprintf("%s %d percent", slice.Label, slice.Percent))
		label := html.EscapeString(slice.Label)
		detail := html.EscapeString(fmt.Sprintf("%s: %d clicks (%d%%)", slice.Label, slice.Clicks, slice.Percent))
		fmt.Fprintf(&segments, `<circle class="donut-segment %s" cx="60" cy="60" r="44" pathLength="100" stroke-dasharray="%d %d" stroke-dashoffset="-%d" tabindex="0" aria-label="%s" data-label="%s" data-clicks="%d" data-percent="%d"><title>%s</title></circle>`, slice.Class, slice.Percent, 100-slice.Percent, offset, detail, label, slice.Clicks, slice.Percent, detail)
		offset += slice.Percent
	}
	aria := html.EscapeString(fmt.Sprintf("Click share: %s", strings.Join(labels, ", ")))
	return template.HTML(fmt.Sprintf(`<svg class="analytics-donut" viewBox="0 0 120 120" role="img" aria-label="%s"><circle class="donut-track" cx="60" cy="60" r="44"/>%s</svg>`, aria, segments.String()))
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

func (s *Server) handleAnalyticsDashboard(w http.ResponseWriter, r *http.Request) {
	links, err := s.db.ListLinks()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	period := analyticsPeriodFromRequest(r)
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	group := analyticsGroupFromRequest(r)
	location := s.cfg.ReportLocation()
	now := time.Now().In(location)
	start, end, bounded := analyticsRange(period, now)
	var startUnix, endUnix *int64
	var clickCounts map[int64]int64
	var previousClickCounts map[int64]int64
	if bounded {
		startValue, endValue := start.Unix(), end.Unix()
		startUnix, endUnix = &startValue, &endValue
		clickCounts, err = s.db.ClickCountsBetween(startValue, endValue)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		previousStart := start.AddDate(0, 0, -analyticsPeriodSpecFor(period).Days)
		previousClickCounts, err = s.db.ClickCountsBetween(previousStart.Unix(), startValue)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
	}

	linkIDs := make([]int64, 0, len(links))
	for _, link := range links {
		if link.Active && (category == "" || link.Category == category) {
			linkIDs = append(linkIDs, link.ID)
		}
	}
	overview, err := s.db.GetOverviewAnalyticsForLinks(linkIDs, startUnix, endUnix)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	data := buildAnalyticsPageDataWithPrevious(links, clickCounts, previousClickCounts, overview, period, category, group)
	data.AdminPath = s.cfg.AdminPath
	data.Production = s.cfg.Production
	if rawID := strings.TrimSpace(r.URL.Query().Get("link")); rawID != "" {
		linkID, parseErr := strconv.ParseInt(rawID, 10, 64)
		if parseErr != nil || linkID <= 0 {
			http.Error(w, "invalid link id", http.StatusBadRequest)
			return
		}
		link, linkErr := s.db.GetLinkByID(linkID)
		if linkErr != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if link == nil {
			http.NotFound(w, r)
			return
		}
		selected, detailErr := s.analyticsDataForLink(r, link, false)
		if detailErr != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		query := url.Values{"period": []string{period}}
		if category != "" {
			query.Set("category", category)
		}
		if group != "link" {
			query.Set("group", group)
		}
		selected.OverviewURL = "/" + s.cfg.AdminPath + "/analytics/dashboard?" + query.Encode()
		data.SelectedLink = selected
	}
	s.renderTemplate(w, "analytics-dashboard", data)
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

func validateLinkMetadata(provider, channel, campaign string) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "provider", value: provider},
		{name: "channel", value: channel},
		{name: "campaign", value: campaign},
	} {
		if len(field.value) > 120 {
			return fmt.Errorf("%s must be 120 characters or fewer", field.name)
		}
	}
	return nil
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
	provider := strings.TrimSpace(r.FormValue("provider"))
	channel := strings.TrimSpace(r.FormValue("channel"))
	campaign := strings.TrimSpace(r.FormValue("campaign"))
	if err := validateLinkMetadata(provider, channel, campaign); err != nil {
		return db.LinkOptions{}, err
	}
	return db.LinkOptions{
		Featured: featured,
		Priority: priority,
		Provider: provider,
		Channel:  channel,
		Campaign: campaign,
	}, nil
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
		formErr("slug is reserved", &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign, Featured: options.Featured, Priority: options.Priority})
		return
	}
	if optionsErr != nil {
		formErr(optionsErr.Error(), &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign, Featured: options.Featured, Priority: options.Priority})
		return
	}

	if _, err := s.db.CreateLinkWithOptions(slug, url, desc, cat, options); err != nil {
		msg := "failed to create link"
		if strings.Contains(err.Error(), "UNIQUE") {
			msg = "slug already exists"
		}
		formErr(msg, &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign, Featured: options.Featured, Priority: options.Priority})
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
			Link:       &db.Link{ID: id, Slug: slug, URL: url, Description: desc, Category: cat, Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign, Featured: options.Featured, Priority: options.Priority},
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

func (s *Server) analyticsDataForLink(r *http.Request, link *db.Link, interactiveChart bool) (*analyticsData, error) {
	location := s.cfg.ReportLocation()
	data, err := s.db.GetAnalyticsInLocation(link.ID, location)
	if err != nil {
		return nil, err
	}

	daily := fillDaysIn(data.Daily, 30, location)
	var last30d int64
	for _, d := range daily {
		last30d += d.Clicks
	}

	lastClick, lastClickTitle := formatAnalyticsLastClickIn(data.LastClickAt, location)
	referrers := data.Referrers
	referrerTotal := data.TotalClicks
	trafficSourcesLabel := "all time"
	trafficSourcesEmpty := "no clicks yet"
	if date := dashboardDateFilter(r); date != "" {
		day, _ := parseDashboardDateIn(date, location)
		referrers, err = s.db.GetReferrersBetween(link.ID, day.Unix(), day.AddDate(0, 0, 1).Unix())
		if err != nil {
			return nil, err
		}
		referrerTotal = 0
		for _, referrer := range referrers {
			referrerTotal += referrer.Clicks
		}
		trafficSourcesLabel = dashboardDateLabel(date)
		trafficSourcesEmpty = "no clicks on " + trafficSourcesLabel
	}
	directTrafficText := "direct traffic"
	if trafficSourcesLabel != "all time" {
		directTrafficText += " · " + trafficSourcesLabel
	}

	return &analyticsData{
		ID:                  link.ID,
		Slug:                link.Slug,
		ShortURL:            publicBaseURL(s.cfg.PublicURL, r) + "/" + link.Slug,
		AnalyticsURL:        "/" + s.cfg.AdminPath + "/links/" + strconv.FormatInt(link.ID, 10) + "/analytics",
		Destination:         link.URL,
		Description:         link.Description,
		Category:            link.Category,
		Provider:            link.Provider,
		Channel:             link.Channel,
		Campaign:            link.Campaign,
		Total:               data.TotalClicks,
		Last30d:             last30d,
		LastClick:           lastClick,
		LastClickTitle:      lastClickTitle,
		DirectTraffic:       directTrafficLabel(referrers, referrerTotal),
		DirectTrafficLabel:  directTrafficText,
		HasRecentClicks:     last30d > 0,
		ChartSVG:            buildChartSVGWithMode(daily, interactiveChart),
		Referrers:           referrers,
		ReferrerTotal:       referrerTotal,
		TrafficSourcesLabel: trafficSourcesLabel,
		TrafficSourcesEmpty: trafficSourcesEmpty,
	}, nil
}

func (s *Server) handleAnalyticsUI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	link, err := s.db.GetLinkByID(id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if link == nil {
		http.NotFound(w, r)
		return
	}
	data, err := s.analyticsDataForLink(r, link, true)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, "analytics", data)
}

func (s *Server) handleOverviewAnalyticsUI(w http.ResponseWriter, r *http.Request) {
	data, err := s.db.GetOverviewAnalyticsInLocation(s.cfg.ReportLocation())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	date := dashboardDateFilter(r)
	dateLabel := ""
	if date != "" {
		location := s.cfg.ReportLocation()
		day, _ := parseDashboardDateIn(date, location)
		data, err = s.db.GetOverviewAnalyticsBetween(day.Unix(), day.AddDate(0, 0, 1).Unix())
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		dateLabel = dashboardDateLabel(date)
	}

	totalLabel := "total clicks"
	directTrafficText := "direct traffic"
	trafficSourcesLabel := "all time"
	trafficSourcesEmpty := "no clicks yet"
	if dateLabel != "" {
		totalLabel = "clicks · " + dateLabel
		directTrafficText += " · " + dateLabel
		trafficSourcesLabel = dateLabel
		trafficSourcesEmpty = "no clicks on " + dateLabel
	}

	s.renderTemplate(w, "overview-analytics", overviewAnalyticsData{
		Total:               data.TotalClicks,
		TotalLabel:          totalLabel,
		Last30d:             data.Last30d,
		ShowLast30d:         dateLabel == "",
		DirectTraffic:       directTrafficSummaryLabel(data.Referrers, data.TotalClicks),
		DirectTrafficLabel:  directTrafficText,
		Referrers:           data.Referrers,
		TrafficSourcesLabel: trafficSourcesLabel,
		TrafficSourcesEmpty: trafficSourcesEmpty,
		AnalyticsURL:        "/" + s.cfg.AdminPath + "/analytics",
		DateSelected:        dateLabel != "",
	})
}
