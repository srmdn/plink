package config

import (
	"bufio"
	"log"
	"os"
	"strings"
	"time"

	_ "time/tzdata"
)

type Config struct {
	Addr           string
	DBPath         string
	AdminPassword  string
	AdminPath      string
	PublicURL      string
	Timezone       string
	SecureCookies  bool
	Production     bool
	SiteName       string
	SiteDesc       string
	reportLocation *time.Location
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

	return &Config{
		Addr:           getEnv("ADDR", ":8080"),
		DBPath:         getEnv("DB_PATH", "plink.db"),
		AdminPassword:  password,
		AdminPath:      getEnv("ADMIN_PATH", "admin"),
		PublicURL:      getEnv("PUBLIC_URL", ""),
		Timezone:       timezone,
		SecureCookies:  getEnv("APP_ENV", "development") == "production",
		Production:     getEnv("APP_ENV", "development") == "production",
		SiteName:       getEnv("SITE_NAME", "plink"),
		SiteDesc:       getEnv("SITE_DESC", "personal links"),
		reportLocation: reportLocation,
	}
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
