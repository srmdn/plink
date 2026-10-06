package server

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/srmdn/plink/internal/db"
)

type brandAssets struct{ LogoURL, FaviconURL, Version string }

func validBrandImage(raw string) bool {
	if raw == "" {
		return true
	}
	if len(raw) > 2048 || strings.ContainsAny(raw, "\r\n\\") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil {
		return false
	}
	if parsed.IsAbs() {
		return parsed.Scheme == "https" && parsed.Hostname() != ""
	}
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || parsed.Host != "" {
		return false
	}
	// Avoid a custom favicon redirecting back to its own generated endpoint.
	normalized := path.Clean(parsed.Path)
	return normalized != "/favicon.svg" && normalized != "/favicon.ico"
}

func brandingAssets(settings db.SiteSettings) brandAssets {
	normalizePalette(&settings)
	assets := brandAssets{}
	if validBrandImage(settings.LogoURL) {
		assets.LogoURL = settings.LogoURL
	}
	if validBrandImage(settings.FaviconURL) {
		assets.FaviconURL = settings.FaviconURL
	}
	if assets.FaviconURL == "" {
		assets.FaviconURL = assets.LogoURL
	}
	digest := sha256.Sum256([]byte(assets.LogoURL + "\x00" + assets.FaviconURL + "\x00" + settings.PublicAccent))
	assets.Version = fmt.Sprintf("%x", digest[:6])
	return assets
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	target := brandingAssets(settings).FaviconURL
	parsed, _ := url.Parse(target)
	self := parsed != nil && strings.EqualFold(parsed.Host, r.Host) && (path.Clean(parsed.Path) == "/favicon.svg" || path.Clean(parsed.Path) == "/favicon.ico")
	if target != "" && !self {
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="9" fill="%s"/><path d="M10 22 22 10M11 10h11v11" fill="none" stroke="white" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/></svg>`, settings.PublicAccent)
}
