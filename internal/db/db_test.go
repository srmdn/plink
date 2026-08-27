package db

import (
	"path/filepath"
	"testing"
	"time"
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
	third, err := database.CreateLink("third", "https://example.com/third", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, referrer := range []string{
		"https://search.example/results",
		"http://SEARCH.example/other?query=1",
	} {
		if err := database.RecordClick(first.ID, referrer, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.RecordClick(second.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.RecordClick(third.ID, "https://search.example/results", ""); err != nil {
		t.Fatal(err)
	}

	analytics, err := database.GetOverviewAnalytics()
	if err != nil {
		t.Fatal(err)
	}
	if analytics.TotalClicks != 4 || analytics.Last30d != 4 {
		t.Fatalf("overview totals = %#v, want 4/4", analytics)
	}
	if len(analytics.Referrers) != 2 {
		t.Fatalf("referrer count = %d, want 2", len(analytics.Referrers))
	}
	top := analytics.Referrers[0]
	if top.Source != "search.example" || top.Clicks != 3 || top.LinkCount != 2 || len(top.Details) != 2 {
		t.Fatalf("top referrer = %#v", top)
	}
	if top.Details[1].Source != "http://SEARCH.example/other?query=1" || top.Details[1].Clicks != 1 {
		t.Fatalf("raw referrer details = %#v", top.Details)
	}
}

func TestNormalizeReferrer(t *testing.T) {
	tests := map[string]string{
		"":                                      "direct",
		"direct":                                "direct",
		"DIRECT":                                "direct",
		"https://Example.com/path/?q=1#section": "example.com",
		"http://example.com:80/":                "example.com",
		"https://example.com.":                  "example.com",
	}
	for input, want := range tests {
		if got := normalizeReferrer(input); got != want {
			t.Errorf("normalizeReferrer(%q) = %q, want %q", input, got, want)
		}
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

func TestAnalyticsDailyUsesProvidedLocation(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "timezone.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	link, err := database.CreateLink("timezone", "https://example.com/timezone", "", "")
	if err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(location)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	for _, clickedAt := range []time.Time{day.Add(30 * time.Minute), day.Add(23 * time.Hour)} {
		if _, err := database.Exec(`INSERT INTO clicks (link_id, clicked_at, referrer, user_agent) VALUES (?, ?, '', '')`, link.ID, clickedAt.Unix()); err != nil {
			t.Fatal(err)
		}
	}

	analytics, err := database.GetAnalyticsInLocation(link.ID, location)
	if err != nil {
		t.Fatal(err)
	}
	if len(analytics.Daily) != 1 || analytics.Daily[0].Date != day.Format("2006-01-02") || analytics.Daily[0].Clicks != 2 {
		t.Fatalf("daily analytics = %#v, want two clicks on %s", analytics.Daily, day.Format("2006-01-02"))
	}
}

func TestClickCountsBetweenUsesHalfOpenRange(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "daily.db"))
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
	start := time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	for _, click := range []struct {
		linkID int64
		at     int64
	}{
		{first.ID, start.Unix()},
		{first.ID, start.Add(time.Hour).Unix()},
		{second.ID, end.Add(-time.Second).Unix()},
		{second.ID, end.Unix()},
	} {
		if _, err := database.Exec(`INSERT INTO clicks (link_id, clicked_at, referrer, user_agent) VALUES (?, ?, '', '')`, click.linkID, click.at); err != nil {
			t.Fatal(err)
		}
	}

	counts, err := database.ClickCountsBetween(start.Unix(), end.Unix())
	if err != nil {
		t.Fatal(err)
	}
	if counts[first.ID] != 2 || counts[second.ID] != 1 {
		t.Fatalf("daily counts = %#v, want first=2 second=1", counts)
	}
}

func TestGetReferrersBetweenUsesReportingDayBoundaries(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "referrer-day.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	link, err := database.CreateLink("referrers", "https://example.com/referrers", "", "")
	if err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.August, 25, 0, 0, 0, 0, location)
	for _, click := range []struct {
		at       time.Time
		referrer string
	}{
		{start.Add(-time.Second), "https://before.example/page"},
		{start, "https://search.example/first"},
		{start.Add(12 * time.Hour), "https://search.example/second"},
		{start.AddDate(0, 0, 1).Add(-time.Second), ""},
		{start.AddDate(0, 0, 1), "https://after.example/page"},
	} {
		if _, err := database.Exec(`INSERT INTO clicks (link_id, clicked_at, referrer, user_agent) VALUES (?, ?, ?, '')`, link.ID, click.at.Unix(), click.referrer); err != nil {
			t.Fatal(err)
		}
	}

	referrers, err := database.GetReferrersBetween(link.ID, start.Unix(), start.AddDate(0, 0, 1).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if len(referrers) != 2 || referrers[0].Source != "search.example" || referrers[0].Clicks != 2 || referrers[1].Source != "direct" || referrers[1].Clicks != 1 {
		t.Fatalf("daily referrers = %#v", referrers)
	}

	referrers, err = database.GetReferrersBetween(link.ID, start.AddDate(0, 0, 2).Unix(), start.AddDate(0, 0, 3).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if len(referrers) != 0 {
		t.Fatalf("empty-day referrers = %#v, want none", referrers)
	}
}

func TestGetOverviewAnalyticsBetweenUsesReportingDayBoundaries(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "overview-referrer-day.db"))
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
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.August, 25, 0, 0, 0, 0, location)
	for _, click := range []struct {
		linkID   int64
		at       time.Time
		referrer string
	}{
		{first.ID, start.Add(-time.Second), "https://before.example/page"},
		{first.ID, start, "https://search.example/first"},
		{second.ID, start.Add(2 * time.Hour), "https://search.example/second"},
		{second.ID, start.AddDate(0, 0, 1), "https://after.example/page"},
	} {
		if _, err := database.Exec(`INSERT INTO clicks (link_id, clicked_at, referrer, user_agent) VALUES (?, ?, ?, '')`, click.linkID, click.at.Unix(), click.referrer); err != nil {
			t.Fatal(err)
		}
	}

	analytics, err := database.GetOverviewAnalyticsBetween(start.Unix(), start.AddDate(0, 0, 1).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if analytics.TotalClicks != 2 || len(analytics.Referrers) != 1 {
		t.Fatalf("daily overview = %#v", analytics)
	}
	referrer := analytics.Referrers[0]
	if referrer.Source != "search.example" || referrer.Clicks != 2 || referrer.LinkCount != 2 {
		t.Fatalf("daily overview referrer = %#v", referrer)
	}

	analytics, err = database.GetOverviewAnalyticsBetween(start.AddDate(0, 0, 2).Unix(), start.AddDate(0, 0, 3).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if analytics.TotalClicks != 0 || len(analytics.Referrers) != 0 {
		t.Fatalf("empty-day overview = %#v", analytics)
	}
}
