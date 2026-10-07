package server

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

const defaultAffiliateDisclosure = "Sebagian tautan di sini referral. Kalau lo beli lewat situ, gue dapat komisi tanpa nambah biaya buat lo."

func (s *Server) currentSiteSettings() (db.SiteSettings, error) {
	settings, err := s.db.GetSiteSettings()
	if err != nil {
		return settings, err
	}
	if !settings.Configured {
		settings.SiteName = s.cfg.SiteName
		settings.SiteDesc = s.cfg.SiteDesc
		settings.AffiliateDisclosure = defaultAffiliateDisclosure
	}
	if settings.FeaturedLimit < 1 || settings.FeaturedLimit > 3 {
		settings.FeaturedLimit = 3
	}
	normalizePalette(&settings)
	return settings, nil
}

func (s *Server) renderSettingsDashboard(w http.ResponseWriter, r *http.Request, data settingsDashboardData) {
	data.EffectivePublicURL = s.publicURL(r)
	data.SEO = s.publicSEO(data.Settings, r, "")
	data.SEO.Title, data.SEO.Description = seoIdentity(data.Settings)
	data.Social = parseSocialLinks(data.Settings.SocialLinks)
	data.AdminPath = s.cfg.AdminPath
	data.Production = s.cfg.Production
	s.renderTemplate(w, "settings-dashboard", data)
}

func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.renderSettingsDashboard(w, r, settingsDashboardData{
		Settings: settings,
		Saved:    r.URL.Query().Get("saved") == "1",
	})
}

// parseSettingsForm parses a urlencoded or multipart settings form with a
// bounded body size, so a logo or share-image upload cannot exhaust memory.
func (s *Server) parseSettingsForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseMultipartForm(maxFormBytes); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return false
	}
	return true
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if !s.parseSettingsForm(w, r) {
		return
	}
	previous, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	settings := db.SiteSettings{
		SiteName:            strings.TrimSpace(r.FormValue("site_name")),
		SiteDesc:            strings.TrimSpace(r.FormValue("site_desc")),
		AffiliateDisclosure: strings.TrimSpace(r.FormValue("affiliate_disclosure")),
		HeroEnabled:         r.FormValue("hero_enabled") == "1" || r.FormValue("hero_enabled") == "on",
	}
	settings.ItemPreviews = previous.ItemPreviews
	if values, present := r.PostForm["item_previews_present"]; present {
		checked := r.PostForm["item_previews"]
		if len(values) != 1 || values[0] != "1" || len(checked) > 1 || (len(checked) == 1 && checked[0] != "1") {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		settings.ItemPreviews = r.FormValue("item_previews") == "1"
	}
	settings.PublicOrigin, settings.HeroEyebrow, settings.HeroTitle, settings.HeroDescription = previous.PublicOrigin, previous.HeroEyebrow, previous.HeroTitle, previous.HeroDescription
	settings.ServiceTitle, settings.ServiceDescription = previous.ServiceTitle, previous.ServiceDescription
	for _, field := range []struct {
		name  string
		value *string
		limit int
	}{
		{"public_origin", &settings.PublicOrigin, 2048}, {"hero_eyebrow", &settings.HeroEyebrow, 80}, {"hero_title", &settings.HeroTitle, 160}, {"hero_description", &settings.HeroDescription, 300}, {"service_title", &settings.ServiceTitle, 80}, {"service_description", &settings.ServiceDescription, 300},
	} {
		if values, present := r.PostForm[field.name]; present {
			if len(values) != 1 || len(values[0]) > field.limit {
				s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "Isian alamat atau konten terlalu panjang."})
				return
			}
			*field.value = strings.TrimSpace(values[0])
		}
	}
	origin, originErr := config.NormalizePublicURL(settings.PublicOrigin, s.cfg.Production)
	if originErr != nil {
		s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "Alamat publik tidak valid: " + originErr.Error()})
		return
	}
	settings.PublicOrigin = origin
	if settings.HeroTitle == "" || settings.ServiceTitle == "" {
		s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "Judul beranda dan bagian jasa wajib diisi."})
		return
	}
	settings.PublicAccent, settings.PublicBackground = previous.PublicAccent, previous.PublicBackground
	settings.AdminAccent, settings.AdminBackground = previous.AdminAccent, previous.AdminBackground
	settings.LogoURL, settings.FaviconURL = previous.LogoURL, previous.FaviconURL
	settings.AvatarURL, settings.SocialLinks = previous.AvatarURL, previous.SocialLinks
	settings.SEOTitle, settings.SEODescription, settings.ShareImageURL = previous.SEOTitle, previous.SEODescription, previous.ShareImageURL
	for _, field := range []struct {
		name  string
		value *string
		limit int
	}{
		{"seo_title", &settings.SEOTitle, 100}, {"seo_description", &settings.SEODescription, 200}, {"share_image_url", &settings.ShareImageURL, 2048},
	} {
		if values, present := r.PostForm[field.name]; present {
			if len(values) != 1 || len(values[0]) > field.limit {
				s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "Isian SEO terlalu panjang atau tidak valid."})
				return
			}
			*field.value = strings.TrimSpace(values[0])
		}
	}
	if path, err := s.applyUploadedFile(r, "share_image_file"); err != nil {
		s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: err.Error()})
		return
	} else if path != "" {
		settings.ShareImageURL = path
	}
	if !validBrandImage(settings.ShareImageURL) || strings.HasPrefix(settings.ShareImageURL, "/og-image.png") {
		s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "Gambar share harus berupa URL HTTPS atau path gambar lokal, bukan endpoint gambar otomatis."})
		return
	}
	for _, asset := range []struct {
		name  string
		value *string
	}{{"logo_url", &settings.LogoURL}, {"favicon_url", &settings.FaviconURL}, {"avatar_url", &settings.AvatarURL}} {
		if values, present := r.PostForm[asset.name]; present {
			if len(values) != 1 {
				http.Error(w, "bad request", 400)
				return
			}
			*asset.value = strings.TrimSpace(values[0])
			if !validBrandImage(*asset.value) {
				s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "Logo, favicon, dan avatar harus berupa URL HTTPS atau path lokal, maksimal 2048 karakter."})
				return
			}
		}
	}
	for _, asset := range []struct {
		field string
		value *string
	}{{"logo_file", &settings.LogoURL}, {"favicon_file", &settings.FaviconURL}, {"avatar_file", &settings.AvatarURL}} {
		path, err := s.applyUploadedFile(r, asset.field)
		if err != nil {
			s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: err.Error()})
			return
		}
		if path != "" {
			*asset.value = path
		}
	}
	socialValues := parseSocialLinks(settings.SocialLinks)
	for _, platform := range socialPlatformList {
		values, present := r.PostForm["social_"+platform.Key]
		if !present {
			continue
		}
		if len(values) != 1 {
			http.Error(w, "bad request", 400)
			return
		}
		raw := strings.TrimSpace(values[0])
		if !validSocialURL(platform.Key, raw) {
			s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "Tautan sosial tidak valid. Gunakan URL http(s), atau alamat email untuk Email."})
			return
		}
		if raw == "" {
			delete(socialValues, platform.Key)
		} else {
			if platform.Key == "email" && !strings.HasPrefix(raw, "mailto:") {
				raw = "mailto:" + raw
			}
			socialValues[platform.Key] = raw
		}
	}
	settings.SocialLinks = encodeSocialLinks(socialValues)
	for _, color := range []struct {
		name  string
		value *string
	}{
		{"public_accent", &settings.PublicAccent}, {"public_background", &settings.PublicBackground},
		{"admin_accent", &settings.AdminAccent}, {"admin_background", &settings.AdminBackground},
	} {
		if value, present := r.PostForm[color.name]; present {
			if len(value) != 1 || !hexColor.MatchString(value[0]) {
				s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "Warna harus berupa kode hex enam digit, misalnya #c43424."})
				return
			}
			*color.value = strings.ToLower(value[0])
		}
	}
	limit, err := strconv.Atoi(strings.TrimSpace(r.FormValue("featured_limit")))
	if err != nil || limit < 1 || limit > 3 {
		settings.FeaturedLimit = 3
		s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "featured slides must be between 1 and 3"})
		return
	}
	settings.FeaturedLimit = limit
	if settings.SiteName == "" {
		s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: "site name is required"})
		return
	}
	for _, field := range []struct {
		name  string
		value string
		limit int
	}{
		{name: "site name", value: settings.SiteName, limit: 80},
		{name: "site description", value: settings.SiteDesc, limit: 200},
		{name: "affiliate disclosure", value: settings.AffiliateDisclosure, limit: 500},
	} {
		if len(field.value) > field.limit {
			s.renderSettingsDashboard(w, r, settingsDashboardData{Settings: settings, Error: fmt.Sprintf("%s must be %d characters or fewer", field.name, field.limit)})
			return
		}
	}
	if err := s.db.SaveSiteSettings(settings); err != nil {
		http.Error(w, "failed to save settings", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/settings?saved=1", http.StatusSeeOther)
}

func normalizePalette(settings *db.SiteSettings) {
	for _, color := range []struct {
		value    *string
		fallback string
	}{
		{&settings.PublicAccent, "#c43424"}, {&settings.PublicBackground, "#f4f1e9"},
		{&settings.AdminAccent, "#c43424"}, {&settings.AdminBackground, "#f4f1e9"},
	} {
		if !hexColor.MatchString(*color.value) {
			*color.value = color.fallback
		}
	}
}

func (s *Server) handleTheme(w http.ResponseWriter, r *http.Request) {
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	accent, background := settings.PublicAccent, settings.PublicBackground
	if r.URL.Query().Get("scope") == "admin" {
		accent, background = settings.AdminAccent, settings.AdminBackground
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, ":root{--accent:%s;--bg:%s;--paper:%s;--green:%s}", accent, background, background, accent)
}
