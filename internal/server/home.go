package server

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/srmdn/plink/internal/db"
)

type homeData struct {
	Links           []db.PublicLink
	Offers          []db.Offer
	FeaturedOffers  []db.Offer
	OfferCategories []string
	Categories      []string
	SiteName        string
	SiteDesc        string
	IsLoggedIn      bool
	AdminPath       string
	Production      bool
}

type catalogPageLink struct {
	Number  int
	URL     string
	Current bool
}

type offerCatalogData struct {
	Offers      []db.Offer
	Categories  []string
	Providers   []string
	Category    string
	Provider    string
	Query       string
	Page        int
	Pages       []catalogPageLink
	PreviousURL string
	NextURL     string
	Total       int
	SiteName    string
	SiteDesc    string
}

type linkCatalogData struct {
	Links       []db.PublicLink
	Categories  []string
	Category    string
	Query       string
	Page        int
	Pages       []catalogPageLink
	PreviousURL string
	NextURL     string
	Total       int
	SiteName    string
	SiteDesc    string
}

func isOfferAvailable(offer db.Offer, today string) bool {
	return offer.Active && offer.HomeActive &&
		(offer.StartsOn == "" || offer.StartsOn <= today) &&
		(offer.EndsOn == "" || offer.EndsOn >= today)
}

func publicOffers(offers []db.Offer, today string) []db.Offer {
	result := make([]db.Offer, 0, len(offers))
	for _, offer := range offers {
		if isOfferAvailable(offer, today) {
			result = append(result, offer)
		}
	}
	return result
}

func categoriesForOffers(offers []db.Offer) []string {
	seen := make(map[string]bool)
	var categories []string
	for _, offer := range offers {
		category := strings.TrimSpace(offer.Category)
		if category != "" && !seen[category] {
			seen[category] = true
			categories = append(categories, category)
		}
	}
	sort.Strings(categories)
	return categories
}

func providersForOffers(offers []db.Offer) []string {
	seen := make(map[string]bool)
	var providers []string
	for _, offer := range offers {
		provider := strings.TrimSpace(offer.Provider)
		if provider != "" && !seen[provider] {
			seen[provider] = true
			providers = append(providers, provider)
		}
	}
	sort.Strings(providers)
	return providers
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	all, err := s.db.ListPublicLinks()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	offers, err := s.db.ListOffers()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	activeOffers := publicOffers(offers, today)
	featuredOffers := make([]db.Offer, 0, 3)
	for _, offer := range activeOffers {
		if offer.Featured && len(featuredOffers) < 3 {
			featuredOffers = append(featuredOffers, offer)
		}
	}
	if len(activeOffers) > 6 {
		activeOffers = activeOffers[:6]
	}

	// Extract unique categories
	seen := make(map[string]bool)
	var categories []string
	for _, l := range all {
		if l.Category != "" && !seen[l.Category] {
			seen[l.Category] = true
			categories = append(categories, l.Category)
		}
	}
	sort.Strings(categories)

	links := all
	if len(links) > 6 {
		links = links[:6]
	}

	loggedIn := false
	if cookie, err := r.Cookie(cookieName); err == nil && s.sessions.valid(cookie.Value) {
		loggedIn = true
	}

	s.renderTemplate(w, "home", homeData{
		Links:           links,
		Offers:          activeOffers,
		FeaturedOffers:  featuredOffers,
		OfferCategories: categoriesForOffers(publicOffers(offers, today)),
		Categories:      categories,
		SiteName:        s.cfg.SiteName,
		SiteDesc:        s.cfg.SiteDesc,
		IsLoggedIn:      loggedIn,
		AdminPath:       s.cfg.AdminPath,
		Production:      s.cfg.Production,
	})
}

func (s *Server) handleOfferCatalog(w http.ResponseWriter, r *http.Request) {
	all, err := s.db.ListOffers()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	visible := publicOffers(all, today)
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	categories := categoriesForOffers(visible)
	providers := providersForOffers(visible)
	filtered := make([]db.Offer, 0, len(visible))
	for _, offer := range visible {
		if category != "" && offer.Category != category {
			continue
		}
		if provider != "" && offer.Provider != provider {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(offer.Title + " " + offer.Provider + " " + offer.Description + " " + offer.Category)
			if !strings.Contains(haystack, strings.ToLower(query)) {
				continue
			}
		}
		filtered = append(filtered, offer)
	}

	const pageSize = 12
	total := len(filtered)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageCount := (total + pageSize - 1) / pageSize
	if pageCount == 0 {
		pageCount = 1
	}
	if page > pageCount {
		page = pageCount
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	pageOffers := filtered
	if start < total {
		pageOffers = filtered[start:end]
	} else {
		pageOffers = nil
	}
	pageURL := func(number int) string {
		values := make(url.Values)
		if query != "" {
			values.Set("q", query)
		}
		if category != "" {
			values.Set("category", category)
		}
		if provider != "" {
			values.Set("provider", provider)
		}
		if number > 1 {
			values.Set("page", strconv.Itoa(number))
		}
		if encoded := values.Encode(); encoded != "" {
			return "/offers?" + encoded
		}
		return "/offers"
	}
	pages := make([]catalogPageLink, 0, pageCount)
	for number := 1; number <= pageCount; number++ {
		pages = append(pages, catalogPageLink{Number: number, URL: pageURL(number), Current: number == page})
	}
	previousURL, nextURL := "", ""
	if page > 1 {
		previousURL = pageURL(page - 1)
	}
	if page < pageCount {
		nextURL = pageURL(page + 1)
	}
	s.renderTemplate(w, "offer-catalog", offerCatalogData{
		Offers: pageOffers, Categories: categories, Providers: providers, Category: category, Provider: provider, Query: query, Page: page, Pages: pages,
		PreviousURL: previousURL, NextURL: nextURL, Total: total, SiteName: s.cfg.SiteName, SiteDesc: s.cfg.SiteDesc,
	})
}

func (s *Server) handleLinkCatalog(w http.ResponseWriter, r *http.Request) {
	all, err := s.db.ListPublicLinks()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	categories := make([]string, 0)
	seenCategories := make(map[string]bool)
	filtered := make([]db.PublicLink, 0, len(all))
	loweredQuery := strings.ToLower(query)
	for _, link := range all {
		if link.Category != "" && !seenCategories[link.Category] {
			seenCategories[link.Category] = true
			categories = append(categories, link.Category)
		}
		if category != "" && link.Category != category {
			continue
		}
		if loweredQuery != "" {
			haystack := strings.ToLower(link.Slug + " " + link.Description + " " + link.Category)
			if !strings.Contains(haystack, loweredQuery) {
				continue
			}
		}
		filtered = append(filtered, link)
	}
	sort.Strings(categories)

	const pageSize = 18
	total := len(filtered)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageCount := (total + pageSize - 1) / pageSize
	if pageCount == 0 {
		pageCount = 1
	}
	if page > pageCount {
		page = pageCount
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	pageLinks := filtered
	if start < total {
		pageLinks = filtered[start:end]
	} else {
		pageLinks = nil
	}
	pageURL := func(number int) string {
		values := make(url.Values)
		if query != "" {
			values.Set("q", query)
		}
		if category != "" {
			values.Set("category", category)
		}
		if number > 1 {
			values.Set("page", strconv.Itoa(number))
		}
		if encoded := values.Encode(); encoded != "" {
			return "/links?" + encoded
		}
		return "/links"
	}
	pages := make([]catalogPageLink, 0, pageCount)
	for number := 1; number <= pageCount; number++ {
		pages = append(pages, catalogPageLink{Number: number, URL: pageURL(number), Current: number == page})
	}
	previousURL, nextURL := "", ""
	if page > 1 {
		previousURL = pageURL(page - 1)
	}
	if page < pageCount {
		nextURL = pageURL(page + 1)
	}
	s.renderTemplate(w, "link-catalog", linkCatalogData{
		Links: pageLinks, Categories: categories, Category: category, Query: query, Page: page, Pages: pages,
		PreviousURL: previousURL, NextURL: nextURL, Total: total, SiteName: s.cfg.SiteName, SiteDesc: s.cfg.SiteDesc,
	})
}
