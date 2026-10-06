package plink

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
	"github.com/srmdn/plink/internal/server"
)

func lifecycleTestServer(t *testing.T) (*db.DB, http.Handler, *config.Config) {
	t.Helper()
	database, err := db.Init(filepath.Join(t.TempDir(), "preview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	cfg := &config.Config{AdminPath: "admin", AdminPassword: "test-password", SiteName: "Test storefront", Timezone: "Asia/Jakarta"}
	return database, server.New(cfg, database, webFS), cfg
}

func TestPublicOfferLifecycle(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	today := time.Now().In(cfg.ReportLocation())
	date := func(offset int) string { return today.AddDate(0, 0, offset).Format("2006-01-02") }
	input := db.OfferInput{Title: "Sample referral", Active: true}
	offer, err := database.CreateOfferWithHomepageLink(input, "sample", "https://provider.example/?ref=old")
	if err != nil {
		t.Fatal(err)
	}
	channel, err := database.CreateOfferLink(offer.ID, "sample-social", "https://provider.example/?ref=social", "social", "test")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, status, starts, ends, behavior, fallback, heading string
		visible                                                 bool
		code                                                    int
		location                                                string
	}{
		{"active", "active", "", "", "notice", "", "", true, 302, "https://provider.example/?ref=social"},
		{"end date inclusive", "active", "", date(0), "notice", "", "", true, 302, "https://provider.example/?ref=social"},
		{"expired no fallback", "active", "", date(-1), "notice", "", "Program sudah berakhir", false, 200, ""},
		{"ended notice", "ended", "", "", "notice", "https://provider.example/go", "Program sudah berakhir", false, 200, ""},
		{"paused overrides fallback", "paused", "", date(-1), "redirect", "https://provider.example/go", "Program sedang dijeda", false, 200, ""},
		{"upcoming", "active", date(1), "", "redirect", "https://provider.example/go", "Program belum dimulai", false, 200, ""},
		{"explicit ended redirect", "ended", "", "", "redirect", "https://provider.example/go", "", false, 302, "https://provider.example/go"},
		{"expired fallback", "active", "", date(-1), "redirect", "https://provider.example/go", "", false, 302, "https://provider.example/go"},
		{"hidden card still redirects", "active", "", "", "notice", "", "", false, 302, "https://provider.example/?ref=social"},
		{"invalid fallback fails to notice", "ended", "", "", "redirect", "javascript:alert(1)", "Program sudah berakhir", false, 200, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input.Active = tc.name != "hidden card still redirects"
			input.ProgramStatus, input.StartsOn, input.EndsOn = tc.status, tc.starts, tc.ends
			input.EndedBehavior, input.FallbackURL = tc.behavior, tc.fallback
			if err := database.UpdateOffer(offer.ID, input); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest("GET", "/sample-social?next=https://untrusted.example", nil)
			request.Header.Set("Referer", "https://social.example/post?private=value#fragment")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.code || response.Header().Get("Location") != tc.location {
				t.Fatalf("response = %d, location %q, body %s", response.Code, response.Header().Get("Location"), response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("slug response must not be stored")
			}
			if tc.code == 200 {
				if !strings.Contains(response.Body.String(), tc.heading) || response.Header().Get("X-Robots-Tag") != "noindex" {
					t.Fatal("notice is missing its heading or noindex")
				}
				if strings.Contains(response.Body.String(), "javascript:") || strings.Contains(response.Body.String(), "ref=social") || strings.Contains(response.Body.String(), "untrusted.example") {
					t.Fatal("notice exposes an unsafe or stale destination")
				}
			}
			home := httptest.NewRecorder()
			handler.ServeHTTP(home, httptest.NewRequest("GET", "/offers", nil))
			if home.Code != 200 {
				t.Fatalf("catalog = %d: %s", home.Code, home.Body.String())
			}
			if strings.Contains(home.Body.String(), "Sample referral") != tc.visible {
				t.Fatal("catalog availability does not match lifecycle")
			}
		})
	}
	if err := database.ToggleLink(channel.ID); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/sample-social", "/unknown-slug"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 404 {
			t.Fatalf("%s = %d, want 404", path, response.Code)
		}
	}
}

func TestNoticeViewsKeepRedirectAnalyticsAndChannelAttribution(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	input := db.OfferInput{Title: "Ended referral", Active: true, ProgramStatus: "ended", NoticeMessage: "Credit ended. <script>alert(1)</script>", StatusChangedOn: "2026-09-22", NoticeSourceURL: "https://provider.example/announcement", VerifiedOn: "2026-10-05", FallbackURL: "https://provider.example"}
	offer, err := database.CreateOfferWithHomepageLink(input, "ended", "https://provider.example/?ref=old")
	if err != nil {
		t.Fatal(err)
	}
	channel, err := database.CreateOfferLink(offer.ID, "ended-social", "https://provider.example/?ref=social", "social", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RecordClick(channel.ID, "", "test"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/ended", "/ended-social"} {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Referer", "https://social.example/post?private=value#fragment")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		if response.Code != 200 || !strings.Contains(response.Body.String(), "&lt;script&gt;") || strings.Contains(response.Body.String(), "<script>alert") {
			t.Fatal("notice must render safely")
		}
		if !strings.Contains(response.Body.String(), "2026-09-22") || !strings.Contains(response.Body.String(), "https://provider.example/announcement") {
			t.Fatal("notice metadata missing")
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("HEAD", "/ended-social", nil))
	updated, err := database.GetOfferByID(offer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Clicks != 1 || updated.NoticeViews != 2 {
		t.Fatalf("clicks = %d, notice views = %d", updated.Clicks, updated.NoticeViews)
	}
	links, err := database.ListOfferLinks(offer.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range links {
		if link.NoticeViews != 1 {
			t.Fatalf("slug %s notice views = %d", link.Slug, link.NoticeViews)
		}
	}
	analytics, err := database.GetOverviewAnalytics()
	if err != nil {
		t.Fatal(err)
	}
	if analytics.TotalClicks != 1 {
		t.Fatalf("notice inflated analytics to %d", analytics.TotalClicks)
	}
	var ref string
	if err := database.QueryRow("SELECT referrer FROM notice_views LIMIT 1").Scan(&ref); err != nil {
		t.Fatal(err)
	}
	if ref != "https://social.example/post" {
		t.Fatalf("referrer not sanitized: %q", ref)
	}
}

func TestOfferLifecycleAdminFormAndProtection(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	offer, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Editable offer", Active: true}, "editable", "https://provider.example/?ref=old")
	if err != nil {
		t.Fatal(err)
	}
	path := "/admin/offers/" + strconv.FormatInt(offer.ID, 10)
	form := url.Values{"title": {"Editable offer"}, "active": {"1"}, "program_status": {"ended"}, "ended_behavior": {"notice"}, "notice_message": {"Credit is no longer offered."}, "notice_source_url": {"https://provider.example/news"}, "status_changed_on": {"2026-09-22"}, "verified_on": {"2026-10-05"}, "fallback_url": {"https://provider.example"}}
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest("POST", path, strings.NewReader(form.Encode())))
	if unauthorized.Code != 302 {
		t.Fatalf("unauthenticated write = %d", unauthorized.Code)
	}
	login := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {"test-password"}}.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loggedIn := httptest.NewRecorder()
	handler.ServeHTTP(loggedIn, login)
	cookies := loggedIn.Result().Cookies()
	post := func(csrf bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range cookies {
			r.AddCookie(cookie)
			if csrf && cookie.Name == "plink_csrf" {
				r.Header.Set("X-CSRF-Token", cookie.Value)
			}
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		return response
	}
	if response := post(false); response.Code != 403 {
		t.Fatalf("missing CSRF = %d", response.Code)
	}
	if response := post(true); response.Code != 303 {
		t.Fatalf("save = %d: %s", response.Code, response.Body.String())
	}
	updated, err := database.GetOfferByID(offer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProgramStatus != "ended" || updated.NoticeMessage != form.Get("notice_message") || updated.VerifiedOn != "2026-10-05" {
		t.Fatal("lifecycle metadata was not saved")
	}
	get := httptest.NewRequest("GET", path, nil)
	for _, cookie := range cookies {
		get.AddCookie(cookie)
	}
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, get)
	if page.Code != 200 || !strings.Contains(page.Body.String(), `value="ended" selected`) {
		t.Fatalf("editor = %d: %s", page.Code, page.Body.String())
	}
	form.Set("notice_source_url", "javascript:alert(1)")
	if response := post(true); response.Code != 200 || !strings.Contains(response.Body.String(), "URL sumber harus diawali") {
		t.Fatal("invalid source URL was accepted")
	}
	updated, _ = database.GetOfferByID(offer.ID)
	if updated.NoticeSourceURL != "https://provider.example/news" {
		t.Fatal("invalid submission modified the offer")
	}
}

func TestPublishedStandaloneSlugsCanJoinAnOfferWithoutLosingHistory(t *testing.T) {
	database, handler, _ := lifecycleTestServer(t)
	original, err := database.CreateLinkWithOptions("already-shared", "https://provider.example/?ref=original", "Original description", "AI", db.LinkOptions{Channel: "blog", Campaign: "launch", Provider: "Original provider"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RecordClick(original.ID, "", "test"); err != nil {
		t.Fatal(err)
	}
	secondary, err := database.CreateLinkWithOptions("already-shared-social", "https://provider.example/?ref=secondary", "Social description", "AI", db.LinkOptions{Channel: "social", Campaign: "launch"})
	if err != nil {
		t.Fatal(err)
	}
	login := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {"test-password"}}.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loggedIn := httptest.NewRecorder()
	handler.ServeHTTP(loggedIn, login)
	cookies := loggedIn.Result().Cookies()
	post := func(path string, form url.Values, csrf bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range cookies {
			r.AddCookie(cookie)
			if csrf && cookie.Name == "plink_csrf" {
				r.Header.Set("X-CSRF-Token", cookie.Value)
			}
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		return response
	}
	form := url.Values{"title": {"Published program"}, "active": {"1"}, "program_status": {"ended"}, "reuse_home_slug": {"1"}, "home_slug": {"already-shared"}}
	response := post("/admin/offers", form, true)
	if response.Code != 303 || strings.Contains(response.Header().Get("Location"), "error=") {
		t.Fatalf("reuse = %d: %s", response.Code, response.Body.String())
	}
	updated, err := database.GetLinkByID(original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.OfferID == 0 || updated.Slug != original.Slug || updated.URL != original.URL || updated.Clicks != 1 || updated.Channel != "blog" || updated.Campaign != "launch" {
		t.Fatalf("published link changed: %#v", updated)
	}
	path := "/admin/offers/" + strconv.FormatInt(updated.OfferID, 10) + "/links/attach"
	if response := post(path, url.Values{"slug": {"already-shared-social"}}, false); response.Code != 403 {
		t.Fatalf("attach without CSRF = %d", response.Code)
	}
	if response := post(path, url.Values{"slug": {"already-shared-social"}}, true); response.Code != 303 || strings.Contains(response.Header().Get("Location"), "error=") {
		t.Fatal("failed to attach published channel slug")
	}
	attached, err := database.GetLinkByID(secondary.ID)
	if err != nil {
		t.Fatal(err)
	}
	if attached.OfferID != updated.OfferID || attached.URL != secondary.URL || attached.Channel != "social" || attached.Campaign != "launch" {
		t.Fatalf("attached link changed: %#v", attached)
	}
	for _, slug := range []string{original.Slug, secondary.Slug} {
		page := httptest.NewRecorder()
		handler.ServeHTTP(page, httptest.NewRequest("GET", "/"+slug, nil))
		if page.Code != 200 || !strings.Contains(page.Body.String(), "Program sudah berakhir") {
			t.Fatalf("published slug %s did not inherit notice", slug)
		}
	}
	other, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Other program", Active: true}, "other-program", "https://other.example")
	if err != nil {
		t.Fatal(err)
	}
	otherPath := "/admin/offers/" + strconv.FormatInt(other.ID, 10) + "/links/attach"
	if response := post(otherPath, url.Values{"slug": {"already-shared-social"}}, true); !strings.Contains(response.Header().Get("Location"), "error=") {
		t.Fatal("link was moved from another offer")
	}
	attached, _ = database.GetLinkByID(secondary.ID)
	if attached.OfferID != updated.OfferID {
		t.Fatal("failed attach moved the original link")
	}
	var countBefore, countAfter int
	if err := database.QueryRow("SELECT COUNT(*) FROM offers").Scan(&countBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateOfferWithExistingHomepageLink(db.OfferInput{Title: "Invalid reuse"}, original.Slug); err == nil {
		t.Fatal("already-owned slug was reused")
	}
	if err := database.QueryRow("SELECT COUNT(*) FROM offers").Scan(&countAfter); err != nil {
		t.Fatal(err)
	}
	if countBefore != countAfter {
		t.Fatal("failed reuse left an orphan offer")
	}
}
