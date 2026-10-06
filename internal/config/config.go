package config

import (
	"bufio"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	_ "time/tzdata"
)

type Config struct {
	Addr               string
	DBPath             string
	AdminPassword      string
	AdminPath          string
	PublicURL          string
	Timezone           string
	SecureCookies      bool
	Production         bool
	SiteName           string
	SiteDesc           string
	AnalyticsScriptURL string
	AnalyticsWebsiteID string
	reportLocation     *time.Location
}

func Load() *Config {
	loadEnvFile(".env")

	password := getEnv("ADMIN_PASSWORD", "")
	if password == "" {
		log.Fatal("ADMIN_PASSWORD must be set in .env or environment")
	}

	timezone := getEnv("APP_TIMEZONE", "UTC")
	reportLocation, err := time.LoadLocation(timezone)
	if err != nil {
		log.Fatalf("APP_TIMEZONE must be a valid IANA timezone, such as Asia/Jakarta: %v", err)
	}

	publicURL, err := NormalizePublicURL(getEnv("PUBLIC_URL", ""), getEnv("APP_ENV", "development") == "production")
	if err != nil {
		log.Fatalf("PUBLIC_URL tidak valid: %v", err)
	}

	analyticsURL := strings.TrimSpace(getEnv("ANALYTICS_SCRIPT_URL", ""))
	if analyticsURL != "" {
		parsed, err := url.Parse(analyticsURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			log.Fatalf("ANALYTICS_SCRIPT_URL must be an http(s) URL with a host: %q", analyticsURL)
		}
		analyticsURL = parsed.String()
	}

	return &Config{
		Addr:               getEnv("ADDR", ":8080"),
		DBPath:             getEnv("DB_PATH", "plink.db"),
		AdminPassword:      password,
		AdminPath:          getEnv("ADMIN_PATH", "admin"),
		PublicURL:          publicURL,
		Timezone:           timezone,
		SecureCookies:      getEnv("APP_ENV", "development") == "production",
		Production:         getEnv("APP_ENV", "development") == "production",
		SiteName:           getEnv("SITE_NAME", "plink"),
		SiteDesc:           getEnv("SITE_DESC", "personal links"),
		AnalyticsScriptURL: analyticsURL,
		AnalyticsWebsiteID: strings.TrimSpace(getEnv("ANALYTICS_WEBSITE_ID", "")),
		reportLocation:     reportLocation,
	}
}

// AnalyticsOrigin returns the scheme and host of the configured analytics
// script, or an empty string when no valid script is configured. It is used to
// allow that exact origin through the Content-Security-Policy.
func (c *Config) AnalyticsOrigin() string {
	if c == nil || c.AnalyticsScriptURL == "" {
		return ""
	}
	parsed, err := url.Parse(c.AnalyticsScriptURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

// ReportLocation returns the timezone used for calendar-day analytics. The
// loader validates APP_TIMEZONE, while this fallback keeps manually-created
// Config values safe in tests and integrations.
func (c *Config) ReportLocation() *time.Location {
	if c != nil && c.reportLocation != nil {
		return c.reportLocation
	}
	if c != nil && c.Timezone != "" {
		if location, err := time.LoadLocation(c.Timezone); err == nil {
			return location
		}
	}
	return time.UTC
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
