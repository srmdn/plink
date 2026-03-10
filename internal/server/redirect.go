package server

import "net/http"

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	link, err := s.db.GetLinkBySlug(slug)
	if err != nil || link == nil {
		http.NotFound(w, r)
		return
	}

	go s.db.RecordClick(link.ID, r.Referer(), r.UserAgent())

	http.Redirect(w, r, link.URL, http.StatusFound)
}
