package server

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

type Server struct {
	cfg          *config.Config
	db           *db.DB
	sessions     *sessionStore
	loginLimiter *loginLimiter
	webFS        embed.FS
	tmpl         *template.Template
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func New(cfg *config.Config, database *db.DB, webFS embed.FS) http.Handler {
	tmpl := template.Must(
		template.New("").Funcs(template.FuncMap{
			"percent": func(val, max int64) int64 {
				if max == 0 {
					return 0
				}
				return val * 100 / max
			},
			"js": template.JSEscaper,
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
		tmpl:         tmpl,
	}

	ap := "/" + cfg.AdminPath

	mux := http.NewServeMux()

	// Static assets
	jsFS, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /js/", http.FileServer(http.FS(jsFS)))
	mux.HandleFunc("GET /favicon.svg", s.handleFavicon)
	mux.HandleFunc("GET /favicon.ico", s.handleFavicon)

	// Auth
	mux.HandleFunc("GET "+ap+"/login", s.handleLoginPage)
	mux.HandleFunc("POST "+ap+"/login", s.handleLogin)
	mux.HandleFunc("POST "+ap+"/logout", s.requireCSRF(s.handleLogout))

	// Admin UI
	mux.HandleFunc("GET "+ap, s.requireAuth(s.handleDashboard))
	mux.HandleFunc("GET "+ap+"/links", s.requireAuth(s.handleLinksSection))
	mux.HandleFunc("GET "+ap+"/links/new", s.requireAuth(s.handleNewLinkForm))
	mux.HandleFunc("GET "+ap+"/links/{id}/edit", s.requireAuth(s.handleEditLinkForm))
	mux.HandleFunc("GET "+ap+"/links/{id}/analytics", s.requireAuth(s.handleAnalyticsUI))
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
	mux.HandleFunc("PATCH /api/links/{id}/toggle", s.requireAuth(s.requireCSRF(s.handleToggleLink)))
	mux.HandleFunc("GET /api/export", s.requireAuth(s.handleExport))

	// Public homepage
	mux.HandleFunc("GET /{$}", s.handleHome)

	// Catch-all: slug redirect (must be last)
	mux.HandleFunc("GET /{slug}", s.handleRedirect)

	return securityHeaders(mux)
}
