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
	cfg      *config.Config
	db       *db.DB
	sessions *sessionStore
	webFS    embed.FS
	tmpl     *template.Template
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
		}).ParseFS(webFS,
			"web/templates/*.html",
			"web/templates/partials/*.html",
		),
	)

	s := &Server{
		cfg:      cfg,
		db:       database,
		sessions: newSessionStore(),
		webFS:    webFS,
		tmpl:     tmpl,
	}

	mux := http.NewServeMux()

	// Static assets
	jsFS, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /js/", http.FileServer(http.FS(jsFS)))
	mux.HandleFunc("GET /favicon.svg", s.handleFavicon)
	mux.HandleFunc("GET /favicon.ico", s.handleFavicon)

	// Auth
	mux.HandleFunc("GET /admin/login", s.handleLoginPage)
	mux.HandleFunc("POST /admin/login", s.handleLogin)
	mux.HandleFunc("POST /admin/logout", s.handleLogout)

	// Admin UI
	mux.HandleFunc("GET /admin", s.requireAuth(s.handleDashboard))
	mux.HandleFunc("GET /admin/links", s.requireAuth(s.handleLinksSection))
	mux.HandleFunc("GET /admin/links/new", s.requireAuth(s.handleNewLinkForm))
	mux.HandleFunc("GET /admin/links/{id}/edit", s.requireAuth(s.handleEditLinkForm))
	mux.HandleFunc("GET /admin/links/{id}/analytics", s.requireAuth(s.handleAnalyticsUI))
	mux.HandleFunc("POST /admin/links", s.requireAuth(s.handleCreateLinkUI))
	mux.HandleFunc("PUT /admin/links/{id}", s.requireAuth(s.handleUpdateLinkUI))
	mux.HandleFunc("DELETE /admin/links/{id}", s.requireAuth(s.handleDeleteLinkUI))
	mux.HandleFunc("PATCH /admin/links/{id}/toggle", s.requireAuth(s.handleToggleLinkUI))

	// REST API (kept for external use / backwards compat)
	mux.HandleFunc("GET /api/links", s.requireAuth(s.handleListLinks))
	mux.HandleFunc("POST /api/links", s.requireAuth(s.handleCreateLink))
	mux.HandleFunc("PUT /api/links/{id}", s.requireAuth(s.handleUpdateLink))
	mux.HandleFunc("DELETE /api/links/{id}", s.requireAuth(s.handleDeleteLink))
	mux.HandleFunc("GET /api/links/{id}/analytics", s.requireAuth(s.handleAnalytics))
	mux.HandleFunc("PATCH /api/links/{id}/toggle", s.requireAuth(s.handleToggleLink))
	mux.HandleFunc("GET /api/export", s.requireAuth(s.handleExport))

	// Catch-all: slug redirect (must be last)
	mux.HandleFunc("GET /{slug}", s.handleRedirect)

	return mux
}
