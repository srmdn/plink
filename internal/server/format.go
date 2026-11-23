package server

// Formatting and URL helpers shared by the admin UI.

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/srmdn/plink/internal/db"
)

func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || (isLoopbackRemote(r) && r.Header.Get("X-Forwarded-Proto") == "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func publicBaseURL(configured string, r *http.Request) string {
	if configured = strings.TrimRight(strings.TrimSpace(configured), "/"); configured != "" {
		return configured
	}
	return baseURL(r)
}

func formatAnalyticsTimestamp(unix int64) string {
	return formatAnalyticsTimestampIn(unix, time.Local)
}

func formatAnalyticsTimestampIn(unix int64, location *time.Location) string {
	if unix <= 0 {
		return "—"
	}
	return time.Unix(unix, 0).In(dashboardLocation(location)).Format("02 Jan 2006, 15:04")
}

func formatAnalyticsLastClick(unix int64) (string, string) {
	return formatAnalyticsLastClickIn(unix, time.Local)
}

func formatAnalyticsLastClickIn(unix int64, location *time.Location) (string, string) {
	if unix <= 0 {
		return "—", "no clicks yet"
	}

	location = dashboardLocation(location)
	clickedAt := time.Unix(unix, 0).In(location)
	full := formatAnalyticsTimestampIn(unix, location)
	now := time.Now().In(location)
	if clickedAt.Year() == now.Year() && clickedAt.YearDay() == now.YearDay() {
		return "Today " + clickedAt.Format("15:04"), full
	}
	yesterday := now.AddDate(0, 0, -1)
	if clickedAt.Year() == yesterday.Year() && clickedAt.YearDay() == yesterday.YearDay() {
		return "Yesterday", full
	}
	return clickedAt.Format("Jan 02"), full
}

func percentOfLabel(val, total int64) string {
	if total <= 0 {
		return "0%"
	}
	if val > 0 && val*100 < total {
		return "<1%"
	}
	return fmt.Sprintf("%d%%", val*100/total)
}

func directTrafficLabel(referrers []db.Referrer, total int64) string {
	if total <= 0 {
		return "—"
	}
	for _, referrer := range referrers {
		if referrer.Source == "direct" {
			return percentOfLabel(referrer.Clicks, total)
		}
	}
	return "0%"
}

func directTrafficSummaryLabel(referrers []db.SourceSummary, total int64) string {
	if total <= 0 {
		return "—"
	}
	for _, referrer := range referrers {
		if referrer.Source == "direct" {
			return percentOfLabel(referrer.Clicks, total)
		}
	}
	return "0%"
}

func referrerLabel(source string) string {
	if strings.EqualFold(source, "direct") || source == "" {
		return "Direct / no referrer"
	}
	return source
}

// humanDate renders a stored date for people instead of machines. Article
// dates are stored either as a plain day or as a full timestamp, so both forms
// are accepted; anything unrecognised is returned unchanged rather than
// silently blanked.
func humanDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("Jan 2, 2006")
		}
	}
	return value
}

// initialsOf reduces a title to the two-character token shown in the compact
// list rows. Letter and digit runs split the title, so "Islami.click" becomes
// "IC" and "The supply chain behind AI" becomes "TS"; a single word falls back
// to its first two characters.
func initialsOf(title string) string {
	words := strings.FieldsFunc(title, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) == 0 {
		return ""
	}
	first := []rune(words[0])
	token := []rune{unicode.ToUpper(first[0])}
	if len(words) > 1 {
		return string(append(token, unicode.ToUpper([]rune(words[1])[0])))
	}
	if len(first) > 1 {
		return string(append(token, unicode.ToUpper(first[1])))
	}
	return string(token)
}
