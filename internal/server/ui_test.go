package server

import (
	"net/http/httptest"
	"net/url"
	"testing"

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
