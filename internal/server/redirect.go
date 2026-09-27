package server

import (
	"net/http"
	"net/url"
	"time"
)

func isAllowedURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func sanitizeReferrer(ref string) string {
	u, err := url.Parse(ref)
	if err != nil || u.Scheme == "" {
		return ref
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	link, err := s.db.GetLinkBySlug(slug)
	if err != nil || link == nil {
		http.NotFound(w, r)
		return
	}

	destination := link.URL
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	if link.OfferID > 0 && link.OfferEndsOn != "" && link.OfferEndsOn < today && isAllowedURL(link.FallbackURL) {
		destination = link.FallbackURL
	}
	if !isAllowedURL(destination) {
		http.NotFound(w, r)
		return
	}

	go s.db.RecordClick(link.ID, sanitizeReferrer(r.Referer()), r.UserAgent())

	http.Redirect(w, r, destination, http.StatusFound)
}
