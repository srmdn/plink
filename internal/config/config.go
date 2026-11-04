package config

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
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
	DevNoAuth          bool
	SiteName           string
	SiteDesc           string
	AnalyticsScriptURL string
	AnalyticsWebsiteID string
	CloudflareInsights bool
	UploadsDir         string
	reportLocation     *time.Location
}

func Load() *Config {
	loadEnvFile(".env")
	addr := getEnv("ADDR", ":8080")
	environment := getEnv("APP_ENV", "development")
	devNoAuth, err := ValidateDevNoAuth(getEnv("DEV_NO_AUTH", ""), environment, addr)
	if err != nil {
		log.Fatal(err)
	}

	password := getEnv("ADMIN_PASSWORD", "")
	if password == "" {
		log.Fatal("ADMIN_PASSWORD must be set in .env or environment")
	}
	if len(password) < 12 {
		log.Printf("plink: warning: ADMIN_PASSWORD is shorter than 12 characters")
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

	cloudflareInsights := false
	switch strings.ToLower(strings.TrimSpace(getEnv("CLOUDFLARE_INSIGHTS", ""))) {
	case "1", "true", "on", "yes":
		cloudflareInsights = true
	}

	analyticsURL := strings.TrimSpace(getEnv("ANALYTICS_SCRIPT_URL", ""))
	if analyticsURL != "" {
		parsed, err := url.Parse(analyticsURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			log.Fatalf("ANALYTICS_SCRIPT_URL must be an http(s) URL with a host: %q", analyticsURL)
		}
		analyticsURL = parsed.String()
	}

	dbPath := getEnv("DB_PATH", "plink.db")
	uploadsDir := strings.TrimSpace(getEnv("UPLOADS_DIR", ""))
	if uploadsDir == "" {
		uploadsDir = filepath.Join(filepath.Dir(dbPath), "uploads")
	}

	return &Config{
		Addr:               addr,
		DBPath:             dbPath,
		AdminPassword:      password,
		AdminPath:          getEnv("ADMIN_PATH", "admin"),
		PublicURL:          publicURL,
		Timezone:           timezone,
		SecureCookies:      getEnv("APP_ENV", "development") == "production",
		Production:         getEnv("APP_ENV", "development") == "production",
		DevNoAuth:          devNoAuth,
		SiteName:           getEnv("SITE_NAME", "plink"),
		SiteDesc:           getEnv("SITE_DESC", "personal links"),
		AnalyticsScriptURL: analyticsURL,
		AnalyticsWebsiteID: strings.TrimSpace(getEnv("ANALYTICS_WEBSITE_ID", "")),
		CloudflareInsights: cloudflareInsights,
		UploadsDir:         uploadsDir,
		reportLocation:     reportLocation,
	}
}

// IsLoopbackListener requires a literal loopback IP, not a wildcard or hostname.
func IsLoopbackListener(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ValidateDevNoAuth rejects accidental use outside explicit local development.
func ValidateDevNoAuth(value, environment, addr string) (bool, error) {
	if value == "" || value == "0" {
		return false, nil
	}
	if value != "1" {
		return false, fmt.Errorf("DEV_NO_AUTH must be 1 or 0")
	}
	if environment != "development" || !IsLoopbackListener(addr) {
		return false, fmt.Errorf("DEV_NO_AUTH requires APP_ENV=development and ADDR bound to a literal loopback IP")
	}
	return true, nil
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
