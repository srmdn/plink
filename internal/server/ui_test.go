package server

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

func TestFilterLinksByQueryCategoryAndStatus(t *testing.T) {
	links := []db.Link{
		{Slug: "active-tools", URL: "https://example.com/tools", Category: "Tools", Active: true},
		{Slug: "paused-tools", URL: "https://example.com/paused", Category: "Tools", Active: false},
		{Slug: "active-course", URL: "https://example.com/course", Category: "Courses", Active: true},
	}

	filtered := filterLinks(links, "tools", "Tools", "active")
	if len(filtered) != 1 || filtered[0].Slug != "active-tools" {
		t.Fatalf("filtered links = %#v, want active-tools", filtered)
	}

	filtered = filterLinks(links, "", "", "paused")
	if len(filtered) != 1 || filtered[0].Slug != "paused-tools" {
		t.Fatalf("paused links = %#v, want paused-tools", filtered)
	}
}

func TestDashboardFiltersPreferFormStateOnLinkSubmit(t *testing.T) {
	form := url.Values{
		"q":               {"new-link"},
		"category":        {"New category"},
		"filter_q":        {"existing-search"},
		"filter_category": {"Existing category"},
		"filter_status":   {"paused"},
	}
	r := httptest.NewRequest("POST", "/admin/links", nil)
	r.PostForm = form
	r.Form = form

	q, category, status := dashboardFilters(r)
	if q != "existing-search" || category != "Existing category" || status != "paused" {
		t.Fatalf("filters = %q, %q, %q", q, category, status)
	}
}

func TestDashboardFiltersDefaultToActive(t *testing.T) {
	r := httptest.NewRequest("GET", "/admin", nil)
	_, _, status := dashboardFilters(r)
	if status != "active" {
		t.Fatalf("default status = %q, want active", status)
	}

	r = httptest.NewRequest("GET", "/admin?status=", nil)
	_, _, status = dashboardFilters(r)
	if status != "" {
		t.Fatalf("explicit all status = %q, want empty", status)
	}

	r = httptest.NewRequest("GET", "/admin?status=paused", nil)
	_, _, status = dashboardFilters(r)
	if status != "paused" {
		t.Fatalf("paused status = %q, want paused", status)
	}
}

func TestDashboardDateFilterPrefersFormState(t *testing.T) {
	form := url.Values{
		"date":        {"2026-08-24"},
		"filter_date": {"2026-08-25"},
	}
	r := httptest.NewRequest("POST", "/admin/links", nil)
	r.PostForm = form
	r.Form = form
	if got := dashboardDateFilter(r); got != "2026-08-25" {
		t.Fatalf("dashboard date = %q, want 2026-08-25", got)
	}

	r = httptest.NewRequest("GET", "/admin/links?date=2026-02-30", nil)
	if got := dashboardDateFilter(r); got != "" {
		t.Fatalf("invalid dashboard date = %q, want empty", got)
	}
}

func TestDashboardURLUsesDashboardRouteAndActiveFilterDefaults(t *testing.T) {
	s := &Server{cfg: &config.Config{AdminPath: "admin"}}
	r := httptest.NewRequest("GET", "/admin/links?q=hello&category=Tools&status=paused&date=2026-08-25", nil)
	if got := s.dashboardURL(r); got != "/admin?category=Tools&date=2026-08-25&q=hello&status=paused" {
		t.Fatalf("dashboard URL = %q", got)
	}

	r = httptest.NewRequest("GET", "/admin/links?status=active", nil)
	if got := s.dashboardURL(r); got != "/admin" {
		t.Fatalf("default dashboard URL = %q, want /admin", got)
	}
}

func TestPublicBaseURL(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/admin", nil)
	if got := publicBaseURL("https://example.com/", r); got != "https://example.com" {
		t.Fatalf("configured public URL = %q", got)
	}
	if got := publicBaseURL("", r); got != "http://example.test" {
		t.Fatalf("request public URL = %q", got)
	}
}

func TestReferrerLabel(t *testing.T) {
	if got := referrerLabel("direct"); got != "Direct / no referrer" {
		t.Fatalf("direct label = %q", got)
	}
	if got := referrerLabel("https://example.com"); got != "https://example.com" {
		t.Fatalf("referrer label = %q", got)
	}
}

func TestPercentOfLabel(t *testing.T) {
	tests := []struct {
		value int64
		total int64
		want  string
	}{
		{value: 0, total: 100, want: "0%"},
		{value: 2, total: 8688, want: "<1%"},
		{value: 88, total: 1032, want: "8%"},
		{value: 73, total: 100, want: "73%"},
	}
	for _, test := range tests {
		if got := percentOfLabel(test.value, test.total); got != test.want {
			t.Errorf("percentOfLabel(%d, %d) = %q, want %q", test.value, test.total, got, test.want)
		}
	}
}

func TestDashboardCountLabelMatchesFilterSemantics(t *testing.T) {
	links := []db.Link{{Slug: "active", Active: true}, {Slug: "paused", Active: false}}
	tests := []struct {
		q, category, status string
		want                string
	}{
		{status: "active", want: "active links"},
		{status: "paused", want: "paused links"},
		{status: "", want: "visible links"},
		{q: "active", status: "active", want: "matching links"},
		{category: "Tools", status: "active", want: "matching links"},
	}
	for _, test := range tests {
		data := buildDashboardData(links, test.q, test.category, test.status, "admin", false)
		if data.CountLabel != test.want {
			t.Errorf("count label for q=%q category=%q status=%q = %q, want %q", test.q, test.category, test.status, data.CountLabel, test.want)
		}
	}
}

func TestBuildDashboardDataForDateSortsAndScopesClicks(t *testing.T) {
	links := []db.Link{
		{ID: 1, Slug: "quiet", Active: true, Clicks: 40},
		{ID: 2, Slug: "top", Active: true, Clicks: 100},
		{ID: 3, Slug: "paused", Active: false, Clicks: 20},
	}
	data := buildDashboardDataForDate(
		links, "", "", "", "2026-08-25",
		map[int64]int64{1: 2, 2: 5, 3: 3}, "admin", false,
	)
	if len(data.Links) != 3 || data.Links[0].Slug != "top" || data.Links[1].Slug != "paused" || data.Links[2].Slug != "quiet" {
		t.Fatalf("daily link order = %#v, want top, paused, quiet", data.Links)
	}
	if data.Links[0].Clicks != 5 || data.TotalClicks != 10 {
		t.Fatalf("daily clicks = %#v, total = %d, want top=5 total=10", data.Links, data.TotalClicks)
	}
	if data.CountLabel != "clicked links" || data.ClicksLabel != "clicks · 25 Aug" || data.DateLabel != "25 Aug" {
		t.Fatalf("daily labels = count %q clicks %q date %q", data.CountLabel, data.ClicksLabel, data.DateLabel)
	}

	data = buildDashboardDataForDate(links, "", "", "active", "2026-08-25", map[int64]int64{2: 5}, "admin", false)
	if len(data.Links) != 1 || data.Links[0].Slug != "top" {
		t.Fatalf("active daily links = %#v, want top only", data.Links)
	}

	if got := dashboardDateLabel(time.Now().In(time.Local).Format(dashboardDateLayout)); got == "" {
		t.Fatal("today should have a dashboard date label")
	}
}

func TestBuildChartSVGIncludesAccessibleZeroClickTargets(t *testing.T) {
	chart := string(buildChartSVG([]dailyFill{{Date: "2026-08-25", Clicks: 0}}))
	if !strings.Contains(chart, `class="chart-day"`) || !strings.Contains(chart, `class="chart-hit"`) {
		t.Fatalf("chart = %s, want accessible day and hit-area elements", chart)
	}
	if !strings.Contains(chart, `aria-label="Filter report to Aug 25, 0 clicks"`) {
		t.Fatalf("chart aria label = %s, want zero-click filter label", chart)
	}
	if !strings.Contains(chart, `data-date="2026-08-25"`) || !strings.Contains(chart, `onclick="selectReportDate(this.dataset.date)"`) {
		t.Fatalf("chart = %s, want date filter action", chart)
	}
}
