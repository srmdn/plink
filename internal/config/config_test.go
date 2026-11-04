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

func TestValidateDevNoAuth(t *testing.T) {
	for _, tc := range []struct {
		value, environment, address string
		want, wantErr               bool
	}{
		{"", "production", ":8080", false, false},
		{"0", "production", ":8080", false, false},
		{"1", "development", "127.0.0.1:8080", true, false},
		{"1", "development", "[::1]:8080", true, false},
		{"1", "production", "127.0.0.1:8080", false, true},
		{"1", "staging", "127.0.0.1:8080", false, true},
		{"1", "development", ":8080", false, true},
		{"1", "development", "0.0.0.0:8080", false, true},
		{"1", "development", "localhost:8080", false, true},
		{"true", "development", "127.0.0.1:8080", false, true},
	} {
		got, err := ValidateDevNoAuth(tc.value, tc.environment, tc.address)
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Errorf("ValidateDevNoAuth(%q, %q, %q) = %v, %v", tc.value, tc.environment, tc.address, got, err)
		}
	}
}
