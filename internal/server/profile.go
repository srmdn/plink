package server

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/srmdn/plink/internal/db"
)

// socialPlatform is one supported profile link. The order here drives both the
// admin form and the public icon row.
type socialPlatform struct {
	Key     string
	Label   string
	Support bool
}

var socialPlatformList = []socialPlatform{
	{Key: "website", Label: "Website"},
	{Key: "instagram", Label: "Instagram"},
	{Key: "x", Label: "X"},
	{Key: "facebook", Label: "Facebook"},
	{Key: "youtube", Label: "YouTube"},
	{Key: "github", Label: "GitHub"},
	{Key: "linkedin", Label: "LinkedIn"},
	{Key: "telegram", Label: "Telegram"},
	{Key: "whatsapp", Label: "WhatsApp"},
	{Key: "email", Label: "Email"},
	{Key: "trakteer", Label: "Trakteer", Support: true},
	{Key: "kofi", Label: "Ko-fi", Support: true},
	{Key: "buymeacoffee", Label: "Buy Me a Coffee", Support: true},
}

// contactStored is one configurable contact entry for the public floating
// contact button. Order and labels are operator-defined.
type contactStored struct {
	Platform string `json:"platform"`
	Label    string `json:"label"`
	URL      string `json:"url"`
}

// socialPlatformLabel returns the display label for a known platform key.
func socialPlatformLabel(key string) string {
	for _, platform := range socialPlatformList {
		if platform.Key == key {
			return platform.Label
		}
	}
	return key
}

// knownPlatform reports whether key is one of the supported contact icons.
func knownPlatform(key string) bool {
	for _, platform := range socialPlatformList {
		if platform.Key == key {
			return true
		}
	}
	return false
}

// parseContactEntries reads ordered contact entries; unknown/empty rows drop.
func parseContactEntries(raw string) []contactStored {
	entries := []contactStored{}
	if strings.TrimSpace(raw) == "" {
		return entries
	}
	var stored []contactStored
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return entries
	}
	for _, item := range stored {
		if item.Platform != "" && strings.TrimSpace(item.URL) != "" {
			entries = append(entries, item)
		}
	}
	return entries
}

// encodeContactEntries stores non-empty contact rows in submission order.
func encodeContactEntries(entries []contactStored) string {
	clean := make([]contactStored, 0, len(entries))
	for _, item := range entries {
		if item.Platform == "" || strings.TrimSpace(item.URL) == "" {
			continue
		}
		clean = append(clean, contactStored{Platform: item.Platform, Label: strings.TrimSpace(item.Label), URL: strings.TrimSpace(item.URL)})
	}
	if len(clean) == 0 {
		return ""
	}
	encoded, err := json.Marshal(clean)
	if err != nil {
		return ""
	}
	return string(encoded)
}

type socialLink struct {
	Key   string
	Label string
	URL   string
}

type publicProfile struct {
	AvatarURL string
	Links     []socialLink
	Support   []socialLink
	Contact   []socialLink
}

// parseSocialLinks reads the stored JSON into a platform -> URL map. Unknown
// platforms and empty values are dropped.
func parseSocialLinks(raw string) map[string]string {
	values := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return values
	}
	var stored []struct {
		Platform string `json:"platform"`
		URL      string `json:"url"`
	}
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return values
	}
	for _, item := range stored {
		if item.Platform != "" && item.URL != "" {
			values[item.Platform] = item.URL
		}
	}
	return values
}

// encodeSocialLinks stores only known platforms with a value, in display order.
func encodeSocialLinks(values map[string]string) string {
	stored := make([]map[string]string, 0, len(values))
	for _, platform := range socialPlatformList {
		if value := strings.TrimSpace(values[platform.Key]); value != "" {
			stored = append(stored, map[string]string{"platform": platform.Key, "url": value})
		}
	}
	if len(stored) == 0 {
		return ""
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// buildPublicProfile resolves the avatar (falling back to the logo) and the
// ordered, non-empty social links for the public header.
func buildPublicProfile(settings db.SiteSettings) publicProfile {
	profile := publicProfile{}
	if settings.AvatarURL != "" && validBrandImage(settings.AvatarURL) {
		profile.AvatarURL = settings.AvatarURL
	} else if settings.LogoURL != "" && validBrandImage(settings.LogoURL) {
		profile.AvatarURL = settings.LogoURL
	}
	values := parseSocialLinks(settings.SocialLinks)
	for _, platform := range socialPlatformList {
		value := values[platform.Key]
		if value == "" {
			continue
		}
		link := socialLink{Key: platform.Key, Label: platform.Label, URL: value}
		if platform.Support {
			profile.Support = append(profile.Support, link)
		} else {
			profile.Links = append(profile.Links, link)
		}
	}
	contactEntries := parseContactEntries(settings.ContactLinks)
	for _, entry := range contactEntries {
		label := strings.TrimSpace(entry.Label)
		if label == "" {
			label = socialPlatformLabel(entry.Platform)
		}
		profile.Contact = append(profile.Contact, socialLink{Key: entry.Platform, Label: label, URL: entry.URL})
	}
	return profile
}

// validSocialURL accepts an https/http URL, or an email address (optionally
// prefixed with mailto:) for the email platform.
func validSocialURL(platform, raw string) bool {
	if raw == "" {
		return true
	}
	if len(raw) > 2048 || strings.ContainsAny(raw, "\r\n\\") {
		return false
	}
	if platform == "email" {
		value := strings.TrimSpace(raw)
		value = strings.TrimPrefix(value, "mailto:")
		value = strings.TrimPrefix(value, "//")
		return strings.Contains(value, "@") && !strings.ContainsAny(value, " \t")
	}
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Hostname() != ""
}
