package server

import (
	"log"
	"net/http"
	"net/url"
	"time"
)

func isAllowedURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
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
	w.Header().Set("Cache-Control", "no-store")
	slug := r.PathValue("slug")

	link, err := s.db.GetLinkBySlug(slug)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if link == nil {
		http.NotFound(w, r)
		return
	}

	destination := link.URL
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	if link.OfferID > 0 {
		offer, err := s.db.GetOfferLifecycle(link.OfferID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		status := offer.LifecycleStatus(today)
		if status != "active" {
			if (status == "ended" || status == "expired") && offer.EndedBehavior == "redirect" && validOfferDestination(offer.FallbackURL) {
				destination = offer.FallbackURL
			} else {
				if s.renderOfferNotice(w, r, offer, status) && r.Method == http.MethodGet {
					if err := s.db.RecordNoticeView(link.ID, status, sanitizeReferrer(r.Referer()), r.UserAgent(), time.Now().Unix()); err != nil {
						log.Printf("record notice view: %v", err)
					}
				}
				return
			}
		}
	}
	if !isAllowedURL(destination) {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodGet {
		go s.db.RecordClick(link.ID, sanitizeReferrer(r.Referer()), r.UserAgent())
	}

	http.Redirect(w, r, destination, http.StatusFound)
}
