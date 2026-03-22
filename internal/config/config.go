package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

type Config struct {
	Addr          string
	DBPath        string
	AdminPassword string
	AdminPath     string
	SecureCookies bool
}

func Load() *Config {
	loadEnvFile(".env")

	password := getEnv("ADMIN_PASSWORD", "")
	if password == "" {
		log.Fatal("ADMIN_PASSWORD must be set in .env or environment")
	}

	return &Config{
		Addr:          getEnv("ADDR", ":8080"),
		DBPath:        getEnv("DB_PATH", "plink.db"),
		AdminPassword: password,
		AdminPath:     getEnv("ADMIN_PATH", "admin"),
		SecureCookies: getEnv("APP_ENV", "development") == "production",
	}
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
