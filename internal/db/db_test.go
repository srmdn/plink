package db

import (
	"path/filepath"
	"testing"
)

func TestCurationMigrationAndPublicOrdering(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "links.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var version int
	if err := database.QueryRow(`SELECT MAX(version) FROM _migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 4 {
		t.Fatalf("migration version = %d, want 4", version)
	}

	if _, err := database.CreateLinkWithOptions("lower", "https://example.com/lower", "", "Tools", LinkOptions{Featured: true, Priority: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateLinkWithOptions("higher", "https://example.com/higher", "", "Tools", LinkOptions{Featured: true, Priority: 20}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateLink("legacy", "https://example.com/legacy", "", "Tools"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateLinkWithOptions("paused", "https://example.com/paused", "", "Tools", LinkOptions{Featured: true, Priority: 100}); err != nil {
		t.Fatal(err)
	}
	if err := database.ToggleLink(4); err != nil {
		t.Fatal(err)
	}

	featured, err := database.ListFeaturedLinks(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(featured) != 2 || featured[0].Slug != "higher" || featured[1].Slug != "lower" {
		t.Fatalf("featured ordering = %#v, want higher then lower", featured)
	}

	public, err := database.ListPublicLinks()
	if err != nil {
		t.Fatal(err)
	}
	if len(public) != 3 || public[0].Slug != "higher" || public[1].Slug != "lower" || public[2].Slug != "legacy" {
		t.Fatalf("public ordering = %#v, want higher, lower, legacy", public)
	}

	links, err := database.ListLinks()
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 4 || !links[0].Featured || links[0].Priority != 20 {
		t.Fatalf("admin link metadata not loaded: %#v", links)
	}
	legacy, err := database.GetLinkByID(3)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Featured || legacy.Priority != 0 {
		t.Fatalf("legacy defaults = featured %v priority %d, want false/0", legacy.Featured, legacy.Priority)
	}
}

func TestOverviewAnalyticsGroupsReferrers(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "analytics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	first, err := database.CreateLink("first", "https://example.com/first", "", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := database.CreateLink("second", "https://example.com/second", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := database.RecordClick(first.ID, "https://search.example/results", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.RecordClick(second.ID, "", ""); err != nil {
		t.Fatal(err)
	}

	analytics, err := database.GetOverviewAnalytics()
	if err != nil {
		t.Fatal(err)
	}
	if analytics.TotalClicks != 3 || analytics.Last30d != 3 {
		t.Fatalf("overview totals = %#v, want 3/3", analytics)
	}
	if len(analytics.Referrers) != 2 {
		t.Fatalf("referrer count = %d, want 2", len(analytics.Referrers))
	}
	if analytics.Referrers[0].Source != "https://search.example/results" || analytics.Referrers[0].Clicks != 2 || analytics.Referrers[0].LinkCount != 1 {
		t.Fatalf("top referrer = %#v", analytics.Referrers[0])
	}
}

func TestLinkAnalyticsTracksLastClick(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "link-analytics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	link, err := database.CreateLink("analytics", "https://example.com/analytics", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RecordClick(link.ID, "", ""); err != nil {
		t.Fatal(err)
	}

	analytics, err := database.GetAnalytics(link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if analytics.TotalClicks != 1 || analytics.LastClickAt <= 0 {
		t.Fatalf("link analytics = %#v, want one click and a timestamp", analytics)
	}
}
