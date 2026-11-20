package server

// Analytics aggregation data, SVG charts, and handlers.

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

type analyticsData struct {
	ID                  int64
	Slug                string
	ShortURL            string
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
	Percent string
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
	WhatsAppClicks      int64
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
	// Click timestamps are stored as whole seconds and the query's end is
	// exclusive. Include the current second so fresh clicks appear immediately.
	return now.AddDate(0, 0, -spec.Days), now.Truncate(time.Second).Add(time.Second), true
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
			fmt.Fprintf(&bars, `<g class="chart-day" tabindex="0" focusable="true" role="button" aria-label="Filter report to %s" data-date="%s"><title>%s — %d clicks. Click to filter the report.</title><rect class="chart-hit" x="%.2f%%" y="0" width="%.2f%%" height="60" fill="transparent"/><rect class="chart-bar" x="%.2f%%" y="%.1f" width="%.2f%%" height="%.1f" fill="#22c55e" opacity="0.75" rx="1"/></g>`, ariaLabel, d.Date, safeLabel, d.Clicks, x, w, x, 60-h, w, h)
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
			Percent: percentOfLabel(link.Clicks, total),
			Class:   fmt.Sprintf("segment-%d", i+1),
		})
	}
	if len(links) > limit && total > used {
		result = append(result, analyticsSlice{
			Label:   "Other",
			Clicks:  total - used,
			Percent: percentOfLabel(total-used, total),
			Class:   "segment-other",
		})
	}
	return result
}

func buildAnalyticsDonutSVG(slices []analyticsSlice, total int64) template.HTML {
	if len(slices) == 0 || total <= 0 {
		return ""
	}
	var segments strings.Builder
	var labels []string
	var offset float64
	for _, slice := range slices {
		labels = append(labels, fmt.Sprintf("%s %s", slice.Label, slice.Percent))
		label := html.EscapeString(slice.Label)
		detail := html.EscapeString(fmt.Sprintf("%s: %d clicks (%s)", slice.Label, slice.Clicks, slice.Percent))
		percent := html.EscapeString(strings.TrimSuffix(slice.Percent, "%"))
		width := float64(slice.Clicks) * 100 / float64(total)
		fmt.Fprintf(&segments, `<circle class="donut-segment %s" cx="60" cy="60" r="44" pathLength="100" stroke-dasharray="%.6f %.6f" stroke-dashoffset="-%.6f" tabindex="0" aria-label="%s" data-label="%s" data-clicks="%d" data-percent="%s"><title>%s</title></circle>`, slice.Class, width, 100-width, offset, detail, label, slice.Clicks, percent, detail)
		offset += width
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
	data.WhatsAppClicks, err = s.db.CountProductWhatsAppClicks(startUnix, endUnix, category)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
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
		ShortURL:            s.publicURL(r) + "/" + link.Slug,
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
	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, "/"+s.cfg.AdminPath+"/analytics/dashboard?link="+strconv.FormatInt(id, 10), http.StatusFound)
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
	if r.Header.Get("HX-Request") != "true" {
		target := "/" + s.cfg.AdminPath + "/analytics/dashboard"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
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
		DateSelected:        dateLabel != "",
	})
}
