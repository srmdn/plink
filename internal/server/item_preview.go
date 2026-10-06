package server

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/srmdn/plink/internal/db"
)

// Preview fetchers, not general search crawlers or mobile in-app browsers.
// User agents are hints, not authentication; unknown fetchers retain redirects.
func isSocialPreviewBot(agent string) bool {
	agent = strings.ToLower(agent)
	for _, name := range []string{"facebookexternalhit", "meta-externalfetcher", "twitterbot", "linkedinbot", "whatsapp", "discordbot", "slackbot-linkexpanding", "telegrambot", "pinterestbot"} {
		if strings.Contains(agent, name) {
			return true
		}
	}
	return false
}

func itemCopy(offer *db.Offer, status string) (string, string) {
	if status != "active" {
		notice := noticeContent(offer, status)
		return notice.Heading + " · " + offer.Title, notice.Message
	}
	description := offer.Description
	if strings.TrimSpace(description) == "" {
		description = offer.Title
		if offer.Provider != "" {
			description += " · " + offer.Provider
		}
	}
	return offer.Title, description
}

func (s *Server) itemSEO(settings db.SiteSettings, r *http.Request, offer *db.Offer, status, slug string) pageSEO {
	title, description := itemCopy(offer, status)
	base := s.publicURL(r)
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s", title, description, offer.Provider, settings.LogoURL, settings.SiteName, settings.PublicAccent, settings.PublicBackground)))
	image := base + "/og-image.png?" + url.Values{"slug": {slug}, "v": {fmt.Sprintf("%x", digest[:8])}}.Encode()
	// Ended/paused cards use a generated status image, avoiding stale promo art.
	if status == "active" && offer.ImageURL != "" && validBrandImage(offer.ImageURL) {
		u, _ := url.Parse(offer.ImageURL)
		image = mustBaseURL(base).ResolveReference(u).String()
	}
	return pageSEO{Title: title, Description: description, URL: base + "/" + url.PathEscape(slug), Image: image, SiteName: settings.SiteName, NoIndex: true}
}

func (s *Server) renderItemPreview(w http.ResponseWriter, r *http.Request, settings db.SiteSettings, offer *db.Offer, status, slug string) {
	data := s.itemSEO(settings, r, offer, status, slug)
	var body bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&body, "item-preview", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("X-Robots-Tag", "noindex")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(body.Bytes())
}
