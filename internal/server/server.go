package server

import (
	"embed"
	"net/http"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

type Server struct {
	cfg      *config.Config
	db       *db.DB
	sessions *sessionStore
	webFS    embed.FS
}

func New(cfg *config.Config, database *db.DB, webFS embed.FS) http.Handler {
	s := &Server{
		cfg:      cfg,
		db:       database,
		sessions: newSessionStore(),
		webFS:    webFS,
	}

	mux := http.NewServeMux()

	// Favicon
	mux.HandleFunc("GET /favicon.svg", s.handleFavicon)
	mux.HandleFunc("GET /favicon.ico", s.handleFavicon)

	// Admin UI — no route-level auth; JS handles auth state via API
	mux.HandleFunc("GET /admin", s.handleAdmin)
	mux.HandleFunc("POST /admin/login", s.handleLogin)
	mux.HandleFunc("POST /admin/logout", s.handleLogout)

	// REST API — all require auth
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
