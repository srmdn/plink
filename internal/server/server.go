package server

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

type Server struct {
	ogCacheMu    sync.Mutex
	ogCache      map[string][]byte
	cfg          *config.Config
	db           *db.DB
	sessions     *sessionStore
	loginLimiter *loginLimiter
	webFS        embed.FS
	uploadsDir   string
	tmpl         *template.Template
}

// analyticsTag is the optional, operator-configured analytics snippet rendered
// on admin pages. It is empty unless ANALYTICS_SCRIPT_URL is set at launch, so
// self-hosted instances never load a third-party script by default.
type analyticsTag struct {
	URL string
	ID  string
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	scriptSrc := "script-src 'self' 'unsafe-inline'"
	connectSrc := ""
	if origin := s.cfg.AnalyticsOrigin(); origin != "" {
		scriptSrc += " " + origin
		connectSrc = " connect-src 'self' " + origin + ";"
	}
	csp := "default-src 'self'; " + scriptSrc + "; style-src 'self' 'unsafe-inline'; img-src 'self' https: data:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'" + connectSrc
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

func New(cfg *config.Config, database *db.DB, webFS embed.FS) http.Handler {
	tmpl := template.Must(
		template.New("").Funcs(template.FuncMap{
			"dict": func(values ...any) map[string]any {
				result := map[string]any{}
				for i := 0; i+1 < len(values); i += 2 {
					result[values[i].(string)] = values[i+1]
				}
				return result
			},
			"itemTypeLabel": func(value string) string {
				return map[string]string{"product": "Produk", "referral": "Referral", "service": "Jasa", "resource": "Resource"}[value]
			},
			"percent": func(val, max int64) int64 {
				if max == 0 {
					return 0
				}
				return val * 100 / max
			},
			"percentOf": func(val, total int64) int64 {
				if total == 0 {
					return 0
				}
				return val * 100 / total
			},
			"percentOfLabel": percentOfLabel,
			"referrerLabel":  func(value string) string { return uiText(referrerLabel(value)) },
			"uiText":         uiText,
			"branding":       func() brandAssets { settings, _ := database.GetSiteSettings(); return brandingAssets(settings) },
			"offerStatus":    offerStatus,
			"isGuide":        func(data any) bool { _, ok := data.(guideData); return ok },
			"add": func(left, right int) int {
				return left + right
			},
			"js":       template.JSEscaper,
			"urlquery": template.URLQueryEscaper,
			"analyticsTag": func() any {
				if cfg.AnalyticsScriptURL == "" {
					return nil
				}
				return analyticsTag{URL: cfg.AnalyticsScriptURL, ID: cfg.AnalyticsWebsiteID}
			},
		}).ParseFS(webFS,
			"web/templates/*.html",
			"web/templates/partials/*.html",
		),
	)

	s := &Server{
		cfg:          cfg,
		db:           database,
		sessions:     newSessionStore(),
		loginLimiter: newLoginLimiter(),
		webFS:        webFS,
		uploadsDir:   cfg.UploadsDir,
		tmpl:         tmpl,
	}

	ap := "/" + cfg.AdminPath

	mux := http.NewServeMux()

	// Static assets
	jsFS, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /js/", http.FileServer(http.FS(jsFS)))
	mux.HandleFunc("GET /js/theme.css", s.handleTheme)
	mux.HandleFunc("GET /favicon.svg", s.handleFavicon)
	mux.HandleFunc("GET /favicon.ico", s.handleFavicon)
	mux.HandleFunc("GET /media/{name}", s.handleMedia)

	// Auth
	mux.HandleFunc("GET "+ap+"/login", s.handleLoginPage)
	mux.HandleFunc("POST "+ap+"/login", s.handleLogin)
	mux.HandleFunc("POST "+ap+"/logout", s.requireCSRF(s.handleLogout))

	// Admin UI
	mux.HandleFunc("GET "+ap, s.requireAuth(s.handleDashboard))
	mux.HandleFunc("GET "+ap+"/offers", s.requireAuth(s.handleOffersDashboard))
	mux.HandleFunc("GET "+ap+"/settings", s.requireAuth(s.handleSettingsPage))
	mux.HandleFunc("GET "+ap+"/guide", s.requireAuth(s.handleGuide))
	mux.HandleFunc("POST "+ap+"/settings", s.requireAuth(s.requireCSRF(s.handleSaveSettings)))
	mux.HandleFunc("GET "+ap+"/offers/new", s.requireAuth(s.handleNewOffer))
	mux.HandleFunc("GET "+ap+"/offers/{id}", s.requireAuth(s.handleOfferDetail))
	mux.HandleFunc("POST "+ap+"/offers", s.requireAuth(s.requireCSRF(s.handleCreateOffer)))
	mux.HandleFunc("POST "+ap+"/offers/{id}", s.requireAuth(s.requireCSRF(s.handleUpdateOffer)))
	mux.HandleFunc("POST "+ap+"/offers/{id}/links", s.requireAuth(s.requireCSRF(s.handleCreateOfferLink)))
	mux.HandleFunc("POST "+ap+"/offers/{id}/links/attach", s.requireAuth(s.requireCSRF(s.handleAttachExistingOfferLink)))
	mux.HandleFunc("POST "+ap+"/offers/{id}/links/{linkID}/homepage", s.requireAuth(s.requireCSRF(s.handleSetOfferHomepageLink)))
	mux.HandleFunc("POST "+ap+"/offers/{id}/toggle", s.requireAuth(s.requireCSRF(s.handleToggleOffer)))
	mux.HandleFunc("GET "+ap+"/analytics/dashboard", s.requireAuth(s.handleAnalyticsDashboard))
	mux.HandleFunc("GET "+ap+"/links", s.requireAuth(s.handleLinksSection))
	mux.HandleFunc("GET "+ap+"/links/new", s.requireAuth(s.handleNewLinkForm))
	mux.HandleFunc("GET "+ap+"/links/{id}/edit", s.requireAuth(s.handleEditLinkForm))
	mux.HandleFunc("GET "+ap+"/links/{id}/analytics", s.requireAuth(s.handleAnalyticsUI))
	mux.HandleFunc("GET "+ap+"/analytics", s.requireAuth(s.handleOverviewAnalyticsUI))
	mux.HandleFunc("POST "+ap+"/links", s.requireAuth(s.requireCSRF(s.handleCreateLinkUI)))
	mux.HandleFunc("PUT "+ap+"/links/{id}", s.requireAuth(s.requireCSRF(s.handleUpdateLinkUI)))
	mux.HandleFunc("DELETE "+ap+"/links/{id}", s.requireAuth(s.requireCSRF(s.handleDeleteLinkUI)))
	mux.HandleFunc("PATCH "+ap+"/links/{id}/toggle", s.requireAuth(s.requireCSRF(s.handleToggleLinkUI)))

	// REST API (kept for external use / backwards compat)
	mux.HandleFunc("GET /api/links", s.requireAuth(s.handleListLinks))
	mux.HandleFunc("POST /api/links", s.requireAuth(s.requireCSRF(s.handleCreateLink)))
	mux.HandleFunc("PUT /api/links/{id}", s.requireAuth(s.requireCSRF(s.handleUpdateLink)))
	mux.HandleFunc("DELETE /api/links/{id}", s.requireAuth(s.requireCSRF(s.handleDeleteLink)))
	mux.HandleFunc("GET /api/links/{id}/analytics", s.requireAuth(s.handleAnalytics))
	mux.HandleFunc("GET /api/analytics", s.requireAuth(s.handleOverviewAnalytics))
	mux.HandleFunc("PATCH /api/links/{id}/toggle", s.requireAuth(s.requireCSRF(s.handleToggleLink)))
	mux.HandleFunc("GET /api/export", s.requireAuth(s.handleExport))

	// Public metadata and share image
	mux.HandleFunc("GET /robots.txt", s.handleRobots)
	mux.HandleFunc("GET /sitemap.xml", s.handleSitemap)
	mux.HandleFunc("GET /og-image.png", s.handleOGImage)

	// Public homepage
	mux.HandleFunc("GET /offers", s.handleOfferCatalog)
	mux.HandleFunc("GET /links", s.handleLinkCatalog)
	mux.HandleFunc("GET /{$}", s.handleHome)

	// Catch-all: slug redirect (must be last)
	mux.HandleFunc("GET /{slug}", s.handleRedirect)

	return s.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == ap || strings.HasPrefix(r.URL.Path, ap+"/") || strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}
		mux.ServeHTTP(w, r)
	}))
}
