package server

import (
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

func TestContrastRatioMatchesWCAG(t *testing.T) {
	cases := []struct {
		foreground, background string
		want                   float64
	}{
		{"#000000", "#ffffff", 21},
		{"#ffffff", "#000000", 21},
		{"#777777", "#ffffff", 4.48},
		{"#f4f1e9", "#f4f1e9", 1},
	}
	for _, tc := range cases {
		ratio, ok := contrastRatio(tc.foreground, tc.background)
		if !ok {
			t.Fatalf("%s on %s: unreadable", tc.foreground, tc.background)
		}
		if diff := ratio - tc.want; diff > 0.1 || diff < -0.1 {
			t.Fatalf("%s on %s = %.2f, want %.2f", tc.foreground, tc.background, ratio, tc.want)
		}
	}
	if _, ok := contrastRatio("red", "#ffffff"); ok {
		t.Fatal("invalid color was accepted")
	}
	if _, ok := contrastRatio("#12345", "#ffffff"); ok {
		t.Fatal("short color was accepted")
	}
}

func TestThemeContrastWarning(t *testing.T) {
	safe := db.SiteSettings{
		PublicAccent: "#1a1a1a", PublicBackground: "#f4f1e9",
		AdminAccent: "#c43424", AdminBackground: "#f4f1e9",
	}
	if warning := themeContrastWarning(safe); warning != "" {
		t.Fatalf("safe palette warned: %q", warning)
	}

	low := db.SiteSettings{
		PublicAccent: "#f4f1e9", PublicBackground: "#f4f1e9",
		AdminAccent: "#c43424", AdminBackground: "#f4f1e9",
	}
	warning := themeContrastWarning(low)
	if !strings.Contains(warning, "Contrast warning:") || !strings.Contains(warning, "public accent") {
		t.Fatalf("low public contrast not reported: %q", warning)
	}
	if strings.Contains(warning, "admin accent") {
		t.Fatalf("safe admin palette reported: %q", warning)
	}

	bothLow := db.SiteSettings{
		PublicAccent: "#f4f1e9", PublicBackground: "#f4f1e9",
		AdminAccent: "#f4f1e9", AdminBackground: "#f4f1e9",
	}
	if warning := themeContrastWarning(bothLow); !strings.Contains(warning, "public accent") || !strings.Contains(warning, "admin accent") {
		t.Fatalf("both scopes should be reported: %q", warning)
	}
}

// TestSettingsPageShowsContrastWarningAfterSave checks the warning is attached
// only to the post-save render, so an invalid form never stacks two notices.
func TestSettingsPageShowsContrastWarningAfterSave(t *testing.T) {
	low := db.SiteSettings{
		SiteName: "Test", HeroTitle: "Test", ServiceTitle: "Services",
		PublicAccent: "#f4f1e9", PublicBackground: "#f4f1e9",
		AdminAccent: "#f4f1e9", AdminBackground: "#f4f1e9",
	}
	s := &Server{cfg: &config.Config{AdminPath: "admin"}, settings: &settingsCache{}}
	s.settings.store(low)
	s.tmpl = template.Must(template.New("").Parse(
		`{{define "settings-dashboard"}}{{if .Saved}}saved{{end}}|{{.ContrastWarning}}{{end}}`,
	))

	render := func(saved bool) string {
		response := httptest.NewRecorder()
		s.renderSettingsDashboard(response, httptest.NewRequest("GET", "/admin/settings?saved=1", nil), settingsDashboardData{Saved: saved, Settings: low})
		return response.Body.String()
	}
	if body := render(true); !strings.Contains(body, "saved|Contrast warning:") {
		t.Fatalf("post-save render = %q", body)
	}
	if body := render(false); strings.Contains(body, "Contrast warning:") {
		t.Fatalf("error render leaked the warning: %q", body)
	}
}
