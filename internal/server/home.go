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

// isLoggedIn reports whether the request carries a valid admin session. It is
// only used to decide whether to show owner-only navigation on public pages.
func (s *Server) isLoggedIn(r *http.Request) bool {
	cookie, err := r.Cookie(cookieName)
	return err == nil && s.sessions.valid(cookie.Value)
}

type homeData struct {
	Settings            db.SiteSettings
	SEO                 pageSEO
	Links               []db.PublicLink
	Offers              []db.Offer
	FeaturedOffers      []db.Offer
	Services            []db.Offer
	ResourceOffers      []db.Offer
	ServiceLinks        []db.PublicLink
	OfferCategories     []string
	Categories          []string
	SiteName            string
	SiteDesc            string
	AffiliateDisclosure string
	HeroEnabled         bool
	IsLoggedIn          bool
	AdminPath           string
	Production          bool
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
	return offer.Active && offer.HomeActive && offer.LifecycleStatus(today) == "active" &&
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
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
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
	featuredOffers := make([]db.Offer, 0, settings.FeaturedLimit)
	featuredOfferIDs := make(map[int64]bool, settings.FeaturedLimit)
	for _, offer := range activeOffers {
		if settings.HeroEnabled && offer.ItemType != "service" && offer.ItemType != "resource" && offer.Featured && len(featuredOffers) < settings.FeaturedLimit {
			featuredOffers = append(featuredOffers, offer)
			featuredOfferIDs[offer.ID] = true
		}
	}
	pageOffers := make([]db.Offer, 0, 6)
	for _, offer := range activeOffers {
		if featuredOfferIDs[offer.ID] || offer.ItemType == "service" || offer.ItemType == "resource" {
			continue
		}
		pageOffers = append(pageOffers, offer)
		if len(pageOffers) == 6 {
			break
		}
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

	var services, resourceOffers []db.Offer
	for _, offer := range activeOffers {
		if offer.ItemType == "service" {
			services = append(services, offer)
		}
		if offer.ItemType == "resource" {
			resourceOffers = append(resourceOffers, offer)
		}
	}
	var serviceLinks, resources []db.PublicLink
	for _, link := range all {
		if isServiceCategory(link.Category) {
			serviceLinks = append(serviceLinks, link)
		} else {
			resources = append(resources, link)
		}
	}
	links := resources
	if len(links) > 6 {
		links = links[:6]
	}

	loggedIn := s.isLoggedIn(r)

	w.Header().Add("Vary", "Cookie")
	s.renderTemplate(w, "home", homeData{
		SEO:      s.publicSEO(settings, r, ""),
		Settings: settings,
		Links:    links,
		Services: services, ResourceOffers: resourceOffers, ServiceLinks: serviceLinks,
		Offers:              pageOffers,
		FeaturedOffers:      featuredOffers,
		OfferCategories:     categoriesForOffers(publicOffers(offers, today)),
		Categories:          categories,
		SiteName:            settings.SiteName,
		SiteDesc:            settings.SiteDesc,
		AffiliateDisclosure: settings.AffiliateDisclosure,
		HeroEnabled:         settings.HeroEnabled,
		IsLoggedIn:          loggedIn,
		AdminPath:           s.cfg.AdminPath,
		Production:          s.cfg.Production,
	})
}

func (s *Server) handleOfferCatalog(w http.ResponseWriter, r *http.Request) {
	s.handleBrowse(w, r, false)
}
func (s *Server) handleLinkCatalog(w http.ResponseWriter, r *http.Request) {
	s.handleBrowse(w, r, true)
}

func isServiceCategory(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "service", "services", "jasa":
		return true
	}
	return false
}

type publicCard struct {
	Title, Provider, Description, Category, ImageURL, ButtonLabel, HomeSlug, ItemType string
}
type browseData struct {
	Settings                                                                        db.SiteSettings
	SEO                                                                             pageSEO
	Cards                                                                           []publicCard
	Categories, Providers                                                           []string
	Category, Provider, Query, View, Title, SiteName, SiteDesc, AffiliateDisclosure string
	Total                                                                           int
	Pages                                                                           []catalogPageLink
	IsLoggedIn                                                                      bool
	AdminPath                                                                       string
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request, resourcesOnly bool) {
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	offers, err := s.db.ListOffers()
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	links, err := s.db.ListPublicLinks()
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	view := r.URL.Query().Get("view")
	if resourcesOnly {
		view = "resources"
	}
	switch view {
	case "services", "favorites", "resources":
	default:
		view = "categories"
	}
	all := []publicCard{}
	for _, o := range publicOffers(offers, time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)) {
		all = append(all, publicCard{o.Title, o.Provider, o.Description, o.Category, o.ImageURL, o.ButtonLabel, o.HomeSlug, o.ItemType})
	}
	for _, l := range links {
		kind := "resource"
		if isServiceCategory(l.Category) {
			kind = "service"
		}
		title := l.Description
		if title == "" {
			title = l.Slug
		}
		all = append(all, publicCard{Title: title, Provider: l.Provider, Category: l.Category, HomeSlug: l.Slug, ButtonLabel: "Buka tautan", ItemType: kind})
	}
	category, provider, query := strings.TrimSpace(r.URL.Query().Get("category")), strings.TrimSpace(r.URL.Query().Get("provider")), strings.TrimSpace(r.URL.Query().Get("q"))
	categories, providers := []string{}, []string{}
	seenCategory, seenProvider := map[string]bool{}, map[string]bool{}
	cards := []publicCard{}
	for _, card := range all {
		if view == "services" && card.ItemType != "service" || view == "resources" && card.ItemType != "resource" {
			continue
		}
		if card.Category != "" && !seenCategory[card.Category] {
			categories = append(categories, card.Category)
			seenCategory[card.Category] = true
		}
		if card.Provider != "" && !seenProvider[card.Provider] {
			providers = append(providers, card.Provider)
			seenProvider[card.Provider] = true
		}
		if category != "" && card.Category != category || provider != "" && card.Provider != provider {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(card.Title+" "+card.Description+" "+card.Provider+" "+card.Category+" "+card.HomeSlug), strings.ToLower(query)) {
			continue
		}
		cards = append(cards, card)
	}
	sort.Strings(categories)
	sort.Strings(providers)
	total := len(cards)
	pages := []catalogPageLink{}
	// Favorites are filtered in this browser, so all available cards must be supplied.
	effectivePage := 1
	if view != "favorites" {
		pageCount := (total + 11) / 12
		if pageCount < 1 {
			pageCount = 1
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		if page > pageCount {
			page = pageCount
		}
		effectivePage = page
		for n := 1; n <= pageCount; n++ {
			values := url.Values{"view": {view}}
			if category != "" {
				values.Set("category", category)
			}
			if provider != "" {
				values.Set("provider", provider)
			}
			if query != "" {
				values.Set("q", query)
			}
			values.Set("page", strconv.Itoa(n))
			pages = append(pages, catalogPageLink{Number: n, URL: "/offers?" + values.Encode(), Current: n == page})
		}
		start := (page - 1) * 12
		end := start + 12
		if end > total {
			end = total
		}
		cards = cards[start:end]
	}
	title := map[string]string{"categories": "Kategori", "services": settings.ServiceTitle, "favorites": "Favorit", "resources": "Resource"}[view]
	w.Header().Add("Vary", "Cookie")
	s.renderTemplate(w, "browse", browseData{Settings: settings, SEO: s.publicSEO(settings, r, view, effectivePage), Cards: cards, Categories: categories, Providers: providers, Category: category, Provider: provider, Query: query, View: view, Title: title, Total: total, Pages: pages, SiteName: settings.SiteName, SiteDesc: settings.SiteDesc, AffiliateDisclosure: settings.AffiliateDisclosure, IsLoggedIn: s.isLoggedIn(r), AdminPath: s.cfg.AdminPath})
}
