package server

import (
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

func (s *Server) publicURL(r *http.Request) string {
	settings, err := s.currentSiteSettings()
	if err == nil {
		if origin, e := config.NormalizePublicURL(settings.PublicOrigin, s.cfg.Production); e == nil && origin != "" {
			return origin
		}
	}
	if origin, e := config.NormalizePublicURL(s.cfg.PublicURL, s.cfg.Production); e == nil && origin != "" {
		return origin
	}
	return baseURL(r)
}

type pageSEO struct {
	Title, Description, URL, Image, SiteName string
	NoIndex                                  bool
}

func seoIdentity(settings db.SiteSettings) (string, string) {
	title, description := settings.SEOTitle, settings.SEODescription
	if title == "" {
		title = settings.SiteName
	}
	if description == "" {
		description = settings.SiteDesc
	}
	if description == "" {
		description = "Kumpulan barang, kelas, dan jasa pilihan."
	}
	return title, description
}

func shareImage(settings db.SiteSettings, base, view string) string {
	if settings.ShareImageURL != "" && validBrandImage(settings.ShareImageURL) {
		parsed, _ := url.Parse(settings.ShareImageURL)
		return mustBaseURL(base).ResolveReference(parsed).String()
	}
	title, description := seoIdentity(settings)
	digest := sha256.Sum256([]byte(title + description + settings.PublicAccent + settings.PublicBackground + settings.LogoURL + settings.HeroEyebrow + settings.ServiceTitle + settings.ServiceDescription))
	values := url.Values{"v": {fmt.Sprintf("%x", digest[:8])}}
	if view != "" {
		values.Set("view", view)
	}
	return base + "/og-image.png?" + values.Encode()
}
func mustBaseURL(base string) *url.URL { parsed, _ := url.Parse(base + "/"); return parsed }

func (s *Server) publicSEO(settings db.SiteSettings, r *http.Request, view string, effectivePage ...int) pageSEO {
	base := s.publicURL(r)
	title, description := seoIdentity(settings)
	canonical := "/"
	values := url.Values{}
	if r.URL.Path != "/" {
		canonical = "/offers"
		label := "Katalog"
		switch view {
		case "services":
			label, description = settings.ServiceTitle, settings.ServiceDescription
			if label == "" {
				label = "Jasa"
			}
			values.Set("view", "services")
		case "resources":
			label = "Resource"
			canonical = "/links"
		case "favorites":
			label = "Favorit"
			values.Set("view", "favorites")
		case "articles":
			label = "Artikel"
			values.Set("view", "articles")
		case "projects":
			label = "Proyek"
			values.Set("view", "projects")
		}
		title = label + " · " + settings.SiteName
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if len(effectivePage) > 0 {
			page = effectivePage[0]
		}
		if page > 1 {
			values.Set("page", strconv.Itoa(page))
		}
	}
	if encoded := values.Encode(); encoded != "" {
		canonical += "?" + encoded
	}
	noIndex := view == "favorites" || strings.TrimSpace(r.URL.Query().Get("q")) != "" || r.URL.Query().Get("category") != "" || r.URL.Query().Get("provider") != ""
	return pageSEO{Title: title, Description: description, URL: base + canonical, Image: shareImage(settings, base, view), SiteName: settings.SiteName, NoIndex: noIndex}
}

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /%s\nDisallow: /api/\nSitemap: %s/sitemap.xml\n", s.cfg.AdminPath, s.publicURL(r))
}
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Location string `xml:"loc"`
	}
	body := struct {
		XMLName   xml.Name `xml:"urlset"`
		Namespace string   `xml:"xmlns,attr"`
		URLs      []entry  `xml:"url"`
	}{Namespace: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	base := s.publicURL(r)
	for _, path := range []string{"/", "/offers", "/offers?view=services", "/offers?view=articles", "/offers?view=projects", "/links"} {
		body.URLs = append(body.URLs, entry{base + path})
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(body)
}
