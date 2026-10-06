package config

import "testing"

func TestAnalyticsOrigin(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"", ""},
		{"https://analytics.example.com/script.js", "https://analytics.example.com"},
		{"http://localhost:3001/script.js", "http://localhost:3001"},
		{"https://example.com", "https://example.com"},
		{"ftp://example.com/script.js", ""},
		{"https://exa mple.com/x", ""},
		{"https://", ""},
	}
	for _, tc := range cases {
		if got := (&Config{AnalyticsScriptURL: tc.url}).AnalyticsOrigin(); got != tc.want {
			t.Fatalf("AnalyticsOrigin(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
	if got := (*Config)(nil).AnalyticsOrigin(); got != "" {
		t.Fatalf("nil AnalyticsOrigin = %q, want empty", got)
	}
}

func TestReportLocation(t *testing.T) {
	location := (&Config{Timezone: "Asia/Jakarta"}).ReportLocation()
	if location.String() != "Asia/Jakarta" {
		t.Fatalf("report location = %q, want Asia/Jakarta", location)
	}

	if got := (&Config{}).ReportLocation().String(); got != "UTC" {
		t.Fatalf("empty timezone location = %q, want UTC", got)
	}
	if got := (&Config{Timezone: "invalid/timezone"}).ReportLocation().String(); got != "UTC" {
		t.Fatalf("invalid timezone location = %q, want UTC", got)
	}
}
