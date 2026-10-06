package server

import (
	"bytes"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/srmdn/plink/internal/db"
)

type offerNoticeData struct {
	SEO         pageSEO
	SiteName    string
	SiteDesc    string
	Offer       *db.Offer
	Heading     string
	Message     string
	DateLabel   string
	Date        string
	Destination string
	SourceURL   string
}

func (s *Server) renderOfferNotice(w http.ResponseWriter, r *http.Request, offer *db.Offer, status string) bool {
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return false
	}
	data := offerNoticeData{
		SiteName: settings.SiteName, SiteDesc: settings.SiteDesc, Offer: offer,
		Heading:   "Program sudah berakhir",
		Message:   "Promo atau manfaat referral dari program ini sudah tidak berlaku. Tautan lama tetap tersedia agar lo bisa membaca pembaruan ini.",
		DateLabel: "Status berlaku sejak", Date: offer.StatusChangedOn,
	}
	switch status {
	case "paused":
		data.Heading = "Program sedang dijeda"
		data.Message = "Promo atau manfaat referral dari program ini sedang dijeda atau diverifikasi. Cek kembali sebelum mendaftar atau membeli."
	case "upcoming":
		data.Heading = "Program belum dimulai"
		data.Message = "Promo atau manfaat referral dari program ini belum berlaku. Cek kembali setelah tanggal mulai."
		data.DateLabel, data.Date = "Mulai berlaku", offer.StartsOn
	case "expired":
		data.DateLabel, data.Date = "Terakhir berlaku", offer.EndsOn
	}
	if offer.NoticeMessage != "" {
		data.Message = offer.NoticeMessage
	}
	// The owner chooses a current, non-referral destination; never reuse a stale
	// referral URL automatically, or accept destinations from request parameters.
	if validOfferDestination(offer.FallbackURL) {
		data.Destination = offer.FallbackURL
	}
	if validOfferDestination(offer.NoticeSourceURL) {
		data.SourceURL = offer.NoticeSourceURL
	}
	data.SEO = s.publicSEO(settings, r, "")
	data.SEO.Title = data.Heading + " · " + offer.Title
	data.SEO.Description = data.Message
	data.SEO.URL = s.publicURL(r) + r.URL.Path
	data.SEO.NoIndex = true
	var body bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&body, "offer-notice", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return false
	}
	w.Header().Set("X-Robots-Tag", "noindex")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = w.Write(body.Bytes())
	return err == nil
}

func (s *Server) handleAttachExistingOfferLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid offer id", http.StatusBadRequest)
		return
	}
	offer, err := s.db.GetOfferLifecycle(id)
	if err != nil || offer == nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	if err := validateOfferSlug(slug, s.cfg.AdminPath); err != nil {
		http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	if err := s.db.AttachExistingOfferLink(id, slug); err != nil {
		http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10)+"?error="+url.QueryEscape("existing slug must be active and not already attached to an offer"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}
