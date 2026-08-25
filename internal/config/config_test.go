package config

import "testing"

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
