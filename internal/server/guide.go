package server

import "net/http"

type guideData struct {
	AdminPath  string
	Production bool
}

func (s *Server) handleGuide(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	s.renderTemplate(w, "guide-dashboard", guideData{
		AdminPath: s.cfg.AdminPath, Production: s.cfg.Production,
	})
}
