package server

// Link CRUD handlers and link form parsing.

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/srmdn/plink/internal/db"
)

type linkFormData struct {
	Link           *db.Link
	Editing        bool
	Categories     []string
	Error          string
	Query          string
	CategoryFilter string
	StatusFilter   string
	DateFilter     string
	AdminPath      string
}

func (s *Server) handleLinksSection(w http.ResponseWriter, r *http.Request) {
	s.serveLinksSection(w, r)
}

func (s *Server) handleNewLinkForm(w http.ResponseWriter, r *http.Request) {
	links, _ := s.db.ListLinks()
	q, cat, status := dashboardFilters(r)
	date := dashboardDateFilter(r)
	s.renderTemplate(w, "link-form", linkFormData{
		Categories: extractCategories(links), Query: q, CategoryFilter: cat, StatusFilter: status, DateFilter: date,
		AdminPath: s.cfg.AdminPath,
	})
}

func (s *Server) handleEditLinkForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	links, _ := s.db.ListLinks()
	cats := extractCategories(links)
	var link *db.Link
	for i := range links {
		if links[i].ID == id {
			l := links[i]
			link = &l
			break
		}
	}
	if link == nil {
		http.NotFound(w, r)
		return
	}
	q, cat, status := dashboardFilters(r)
	date := dashboardDateFilter(r)
	s.renderTemplate(w, "link-form", linkFormData{
		Link: link, Editing: true, Categories: cats, Query: q, CategoryFilter: cat, StatusFilter: status, DateFilter: date,
		AdminPath: s.cfg.AdminPath,
	})
}

func validateLinkMetadata(provider, channel, campaign string) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "provider", value: provider},
		{name: "channel", value: channel},
		{name: "campaign", value: campaign},
	} {
		if len(field.value) > 120 {
			return fmt.Errorf("%s must be 120 characters or fewer", field.name)
		}
	}
	return nil
}

func parseLinkOptions(r *http.Request) (db.LinkOptions, error) {
	options := db.LinkOptions{
		Featured: r.FormValue("featured") == "1" || r.FormValue("featured") == "on" || r.FormValue("featured") == "true",
		Unlisted: r.FormValue("unlisted") == "1",
		Provider: strings.TrimSpace(r.FormValue("provider")),
		Channel:  strings.TrimSpace(r.FormValue("channel")),
		Campaign: strings.TrimSpace(r.FormValue("campaign")),
	}
	priority := 0
	priorityValue := strings.TrimSpace(r.FormValue("priority"))
	if priorityValue != "" {
		parsed, err := strconv.Atoi(priorityValue)
		if err != nil || parsed < 0 {
			return options, fmt.Errorf("priority must be a non-negative number")
		}
		priority = parsed
	}
	options.Priority = priority
	if err := validateLinkMetadata(options.Provider, options.Channel, options.Campaign); err != nil {
		return options, err
	}
	return options, nil
}

func (s *Server) handleCreateLinkUI(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	url := strings.TrimSpace(r.FormValue("url"))
	desc := strings.TrimSpace(r.FormValue("description"))
	cat := strings.TrimSpace(r.FormValue("category"))
	options, optionsErr := parseLinkOptions(r)
	q, filterCat, filterStatus := dashboardFilters(r)
	date := dashboardDateFilter(r)

	links, _ := s.db.ListLinks()
	cats := extractCategories(links)

	formErr := func(msg string, link *db.Link) {
		w.Header().Set("HX-Retarget", "#modal-body")
		w.Header().Set("HX-Reswap", "innerHTML")
		s.renderTemplate(w, "link-form", linkFormData{
			Link: link, Categories: cats, Error: msg, Query: q, CategoryFilter: filterCat,
			StatusFilter: filterStatus, DateFilter: date, AdminPath: s.cfg.AdminPath,
		})
	}

	if slug == "" || url == "" {
		formErr("slug and url are required", &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign, Featured: options.Featured, Unlisted: options.Unlisted, Priority: options.Priority})
		return
	}
	if isReservedSlug(slug, s.cfg.AdminPath) {
		formErr("slug is reserved", &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign, Featured: options.Featured, Unlisted: options.Unlisted, Priority: options.Priority})
		return
	}
	if optionsErr != nil {
		formErr(optionsErr.Error(), &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign, Featured: options.Featured, Unlisted: options.Unlisted, Priority: options.Priority})
		return
	}

	if _, err := s.db.CreateLinkWithOptions(slug, url, desc, cat, options); err != nil {
		msg := "failed to create link"
		if strings.Contains(err.Error(), "UNIQUE") {
			msg = "slug already exists"
		}
		formErr(msg, &db.Link{Slug: slug, URL: url, Description: desc, Category: cat, Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign, Featured: options.Featured, Unlisted: options.Unlisted, Priority: options.Priority})
		return
	}

	w.Header().Set("HX-Trigger", "closeModal")
	s.serveLinksSection(w, r)
}

func (s *Server) handleUpdateLinkUI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	url := strings.TrimSpace(r.FormValue("url"))
	desc := strings.TrimSpace(r.FormValue("description"))
	cat := strings.TrimSpace(r.FormValue("category"))
	options, optionsErr := parseLinkOptions(r)
	q, filterCat, filterStatus := dashboardFilters(r)
	date := dashboardDateFilter(r)

	links, _ := s.db.ListLinks()
	cats := extractCategories(links)

	formErr := func(msg string) {
		w.Header().Set("HX-Retarget", "#modal-body")
		w.Header().Set("HX-Reswap", "innerHTML")
		link := &db.Link{ID: id, Slug: slug, URL: url, Description: desc, Category: cat, Provider: options.Provider, Channel: options.Channel, Campaign: options.Campaign, Featured: options.Featured, Unlisted: options.Unlisted, Priority: options.Priority}
		for _, current := range links {
			if current.ID == id {
				link.OfferID = current.OfferID
				break
			}
		}
		s.renderTemplate(w, "link-form", linkFormData{
			Link:       link,
			Editing:    true,
			Categories: cats, Error: msg, Query: q, CategoryFilter: filterCat,
			StatusFilter: filterStatus, DateFilter: date, AdminPath: s.cfg.AdminPath,
		})
	}

	if slug == "" || url == "" {
		formErr("slug and url are required")
		return
	}

	if optionsErr != nil {
		formErr(optionsErr.Error())
		return
	}
	for _, current := range links {
		if current.ID == id {
			if isReservedSlug(slug, s.cfg.AdminPath) && slug != current.Slug {
				formErr("slug is reserved")
				return
			}
			if current.OfferID > 0 {
				if err := validateOfferSlug(slug, s.cfg.AdminPath); err != nil {
					formErr(err.Error())
					return
				}
			}
			break
		}
	}

	if err := s.db.UpdateLinkWithOptions(id, slug, url, desc, cat, options); err != nil {
		msg := "failed to update link"
		if strings.Contains(err.Error(), "UNIQUE") {
			msg = "slug already exists"
		}
		formErr(msg)
		return
	}

	w.Header().Set("HX-Trigger", "closeModal")
	s.serveLinksSection(w, r)
}

func (s *Server) handleDeleteLinkUI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteLink(id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.serveLinksSection(w, r)
}

func (s *Server) handleToggleLinkUI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.db.ToggleLink(id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.serveLinksSection(w, r)
}
