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
