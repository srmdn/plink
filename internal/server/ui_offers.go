package server

// Offers dashboard handlers and offer form parsing.

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/srmdn/plink/internal/db"
)

type offersDashboardData struct {
	Saved         bool
	CatalogQuery  string
	ItemType      string
	Visibility    string
	ReuseHomeSlug bool
	Offers        []db.Offer
	Offer         *db.Offer
	Links         []db.Link
	LinkGroups    []offerLinkGroup
	LinkQuery     string
	LinkChannel   string
	LinkCampaign  string
	LinkChannels  []offerLinkFilterOption
	LinkCampaigns []offerLinkFilterOption
	ActiveCount   int
	TotalClicks   int64
	IsForm        bool
	IsNew         bool
	Error         string
	FieldErrors   map[string]string
	Today         string
	AdminPath     string
	PublicURL     string
	Production    bool
}

type offerLinkGroup struct {
	Key    string
	Label  string
	Links  []db.Link
	Clicks int64
	Open   bool
}

type offerLinkFilterOption struct {
	Value string
	Label string
}

// statusLabel renders lifecycle and visibility states for the admin UI.
func statusLabel(value string) string {
	switch value {
	case "active":
		return "Active"
	case "paused":
		return "Paused"
	case "ended":
		return "Ended"
	case "expired":
		return "Expired"
	case "upcoming":
		return "Upcoming"
	case "hidden":
		return "Hidden"
	case "needs homepage link":
		return "No card link"
	case "card link paused":
		return "Card link paused"
	}
	return value
}

func offerStatus(offer db.Offer, today string) string {
	if status := offer.LifecycleStatus(today); status != "active" {
		return status
	}
	if !offer.Active {
		return "hidden"
	}
	if offer.StartsOn != "" && offer.StartsOn > today {
		return "upcoming"
	}
	if offer.EndsOn != "" && offer.EndsOn < today {
		return "expired"
	}
	if offer.HomeSlug == "" {
		return "needs homepage link"
	}
	if !offer.HomeActive {
		return "card link paused"
	}
	return "active"
}

func validateOfferSlug(slug, adminPath string) error {
	if slug == "" {
		return fmt.Errorf("homepage slug is required")
	}
	if len(slug) > 120 {
		return fmt.Errorf("slug must be 120 characters or fewer")
	}
	if strings.TrimSpace(slug) != slug || slug == "." || slug == ".." {
		return fmt.Errorf("slug may not have surrounding spaces or be a dot path")
	}
	for _, char := range slug {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("-_.", char) {
			continue
		}
		return fmt.Errorf("slug may contain only letters, numbers, hyphens, underscores, and dots")
	}
	if isReservedSlug(slug, adminPath) {
		return fmt.Errorf("slug is reserved")
	}
	return nil
}

func validOfferDestination(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func validOfferImage(raw string) bool {
	if raw == "" {
		return true
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

// offerFieldError ties a validation failure to the form field that caused it.
// The message is also used as the form-level banner, so a failure keeps exactly
// one wording wherever it is shown.

type offerFieldError struct {
	Field   string
	Message string
}

func (e *offerFieldError) Error() string { return e.Message }

func offerFieldErrorf(field, format string, args ...any) error {
	return &offerFieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// offerErrorData splits a validation error into the form-level banner message
// and the per-field map the offer form renders. Errors without an owning field
// stay banner-only, so a database or storage failure never marks an input.

func offerErrorData(err error) (string, map[string]string) {
	if err == nil {
		return "", nil
	}
	var fieldErr *offerFieldError
	if errors.As(err, &fieldErr) {
		if fieldErr.Field == "" {
			return fieldErr.Message, nil
		}
		return fieldErr.Message, map[string]string{fieldErr.Field: fieldErr.Message}
	}
	return err.Error(), nil
}

func parseOfferInput(r *http.Request) (db.OfferInput, error) {
	input := db.OfferInput{
		ItemType:        strings.TrimSpace(r.FormValue("item_type")),
		ProgramStatus:   strings.TrimSpace(r.FormValue("program_status")),
		EndedBehavior:   strings.TrimSpace(r.FormValue("ended_behavior")),
		NoticeMessage:   strings.TrimSpace(r.FormValue("notice_message")),
		NoticeSourceURL: strings.TrimSpace(r.FormValue("notice_source_url")),
		StatusChangedOn: strings.TrimSpace(r.FormValue("status_changed_on")),
		VerifiedOn:      strings.TrimSpace(r.FormValue("verified_on")),
		Title:           strings.TrimSpace(r.FormValue("title")),
		Provider:        strings.TrimSpace(r.FormValue("provider")),
		Description:     strings.TrimSpace(r.FormValue("description")),
		Category:        strings.TrimSpace(r.FormValue("category")),
		ImageURL:        strings.TrimSpace(r.FormValue("image_url")),
		ButtonLabel:     strings.TrimSpace(r.FormValue("button_label")),
		FallbackURL:     strings.TrimSpace(r.FormValue("fallback_url")),
		StartsOn:        strings.TrimSpace(r.FormValue("starts_on")),
		EndsOn:          strings.TrimSpace(r.FormValue("ends_on")),
		Active:          r.FormValue("active") == "1" || r.FormValue("active") == "on",
		Featured:        r.FormValue("featured") == "1" || r.FormValue("featured") == "on",
	}
	product, productErr := parseProductDetails(r)
	input.Product = product
	if productErr != nil {
		return input, productErr
	}
	if input.ItemType == "" {
		input.ItemType = "referral"
	}
	if input.ItemType != "product" && input.ItemType != "referral" && input.ItemType != "service" && input.ItemType != "resource" && input.ItemType != "article" && input.ItemType != "project" {
		return input, offerFieldErrorf("item_type", "invalid item type")
	}
	if input.ProgramStatus == "" {
		input.ProgramStatus = "active"
	}
	if input.EndedBehavior == "" {
		input.EndedBehavior = "notice"
	}
	if input.ProgramStatus != "active" && input.ProgramStatus != "paused" && input.ProgramStatus != "ended" {
		return input, offerFieldErrorf("program_status", "invalid program status")
	}
	if input.EndedBehavior != "notice" && input.EndedBehavior != "redirect" {
		return input, offerFieldErrorf("ended_behavior", "invalid behavior after program ends")
	}
	if input.EndedBehavior == "redirect" && input.FallbackURL == "" {
		return input, offerFieldErrorf("fallback_url", "a destination URL is required for redirect after the program ends")
	}
	if input.NoticeSourceURL != "" && !validOfferDestination(input.NoticeSourceURL) {
		return input, offerFieldErrorf("notice_source_url", "source URL must start with http:// or https://")
	}
	priorityValue := strings.TrimSpace(r.FormValue("priority"))
	if priorityValue != "" {
		priority, err := strconv.Atoi(priorityValue)
		if err != nil || priority < 0 {
			return input, offerFieldErrorf("priority", "priority must be a non-negative number")
		}
		input.Priority = priority
	}
	if input.Title == "" {
		return input, offerFieldErrorf("title", "title is required")
	}
	for _, field := range []struct {
		name  string
		key   string
		value string
		limit int
	}{
		{name: "title", key: "title", value: input.Title, limit: 160},
		{name: "provider", key: "provider", value: input.Provider, limit: 120},
		{name: "description", key: "description", value: input.Description, limit: 500},
		{name: "category", key: "category", value: input.Category, limit: 80},
		{name: "image URL", key: "image_url", value: input.ImageURL, limit: 2048},
		{name: "button label", key: "button_label", value: input.ButtonLabel, limit: 48},
		{name: "fallback URL", key: "fallback_url", value: input.FallbackURL, limit: 2048},
		{name: "notice message", key: "notice_message", value: input.NoticeMessage, limit: 1000},
		{name: "source URL", key: "notice_source_url", value: input.NoticeSourceURL, limit: 2048},
	} {
		if len(field.value) > field.limit {
			return input, offerFieldErrorf(field.key, "%s must be %d characters or fewer", field.name, field.limit)
		}
	}
	if !validOfferImage(input.ImageURL) {
		return input, offerFieldErrorf("image_url", "image URL must be an HTTPS URL or a site path")
	}
	if input.FallbackURL != "" && !validOfferDestination(input.FallbackURL) {
		return input, offerFieldErrorf("fallback_url", "fallback URL must start with http:// or https://")
	}
	for _, field := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "start date", key: "starts_on", value: input.StartsOn},
		{name: "end date", key: "ends_on", value: input.EndsOn},
		{name: "status effective date", key: "status_changed_on", value: input.StatusChangedOn},
		{name: "last verified date", key: "verified_on", value: input.VerifiedOn},
	} {
		if field.value != "" {
			parsed, err := time.Parse(dashboardDateLayout, field.value)
			if err != nil || parsed.Format(dashboardDateLayout) != field.value {
				return input, offerFieldErrorf(field.key, "%s must be a valid date", field.name)
			}
		}
	}
	if input.StartsOn != "" && input.EndsOn != "" && input.StartsOn > input.EndsOn {
		return input, offerFieldErrorf("ends_on", "end date must be on or after start date")
	}
	return input, nil
}

func offerFromInput(input db.OfferInput, id int64, homeSlug, homeURL string, homeActive bool) *db.Offer {
	offer := &db.Offer{
		ID: id, ItemType: input.ItemType, Title: input.Title, Provider: input.Provider, Description: input.Description, Category: input.Category,
		ProgramStatus: input.ProgramStatus, EndedBehavior: input.EndedBehavior, NoticeMessage: input.NoticeMessage,
		NoticeSourceURL: input.NoticeSourceURL, StatusChangedOn: input.StatusChangedOn, VerifiedOn: input.VerifiedOn,
		ImageURL: input.ImageURL, ButtonLabel: input.ButtonLabel, FallbackURL: input.FallbackURL,
		StartsOn: input.StartsOn, EndsOn: input.EndsOn, Active: input.Active, Featured: input.Featured,
		Priority: input.Priority, HomeSlug: homeSlug, HomeURL: homeURL, HomeActive: homeActive,
	}
	if input.Product != nil {
		offer.ProductPage, offer.ProductKind, offer.PriceMinor = input.Product.Enabled, input.Product.Kind, input.Product.PriceMinor
		offer.Currency, offer.ProductDetails, offer.Fulfillment = input.Product.Currency, input.Product.Details, input.Product.Fulfillment
		if input.Product.WhatsApp != nil {
			offer.WhatsAppEnabled, offer.WhatsAppNumber, offer.WhatsAppMessage = input.Product.WhatsApp.Enabled, input.Product.WhatsApp.Number, input.Product.WhatsApp.Message
		}
	}
	return offer
}

const offerHomepageLinkFilter = "__homepage__"

const offerUnassignedFilter = "__unassigned__"

func offerLinkChannelFilterValue(link db.Link) string {
	if link.OfferHome {
		return offerHomepageLinkFilter
	}
	value := strings.ToLower(strings.TrimSpace(link.Channel))
	if value == "" {
		return offerUnassignedFilter
	}
	return value
}

func offerLinkCampaignFilterValue(link db.Link) string {
	value := strings.ToLower(strings.TrimSpace(link.Campaign))
	if value == "" {
		return offerUnassignedFilter
	}
	return value
}

func sortOfferLinkOptions(options []offerLinkFilterOption) {
	sort.Slice(options, func(i, j int) bool {
		return strings.ToLower(options[i].Label) < strings.ToLower(options[j].Label)
	})
}

func groupOfferLinks(links []db.Link, query, channelFilter, campaignFilter string) ([]offerLinkGroup, []offerLinkFilterOption, []offerLinkFilterOption) {
	channelsByValue := make(map[string]string)
	campaignsByValue := make(map[string]string)
	for _, link := range links {
		channelValue := offerLinkChannelFilterValue(link)
		if _, ok := channelsByValue[channelValue]; !ok {
			switch channelValue {
			case offerHomepageLinkFilter:
				channelsByValue[channelValue] = "Homepage"
			case offerUnassignedFilter:
				channelsByValue[channelValue] = "Unassigned"
			default:
				channelsByValue[channelValue] = strings.TrimSpace(link.Channel)
			}
		}
		campaignValue := offerLinkCampaignFilterValue(link)
		if _, ok := campaignsByValue[campaignValue]; !ok {
			if campaignValue == offerUnassignedFilter {
				campaignsByValue[campaignValue] = "Unassigned"
			} else {
				campaignsByValue[campaignValue] = strings.TrimSpace(link.Campaign)
			}
		}
	}

	channelOptions := make([]offerLinkFilterOption, 0, len(channelsByValue))
	for value, label := range channelsByValue {
		channelOptions = append(channelOptions, offerLinkFilterOption{Value: value, Label: label})
	}
	sortOfferLinkOptions(channelOptions)
	campaignOptions := make([]offerLinkFilterOption, 0, len(campaignsByValue))
	for value, label := range campaignsByValue {
		campaignOptions = append(campaignOptions, offerLinkFilterOption{Value: value, Label: label})
	}
	sortOfferLinkOptions(campaignOptions)

	query = strings.ToLower(strings.TrimSpace(query))
	channelFilter = strings.ToLower(strings.TrimSpace(channelFilter))
	campaignFilter = strings.ToLower(strings.TrimSpace(campaignFilter))
	groupsByKey := make(map[string]*offerLinkGroup)
	groupKeys := make([]string, 0)
	for _, link := range links {
		channelValue := offerLinkChannelFilterValue(link)
		campaignValue := offerLinkCampaignFilterValue(link)
		if channelFilter != "" && channelFilter != channelValue {
			continue
		}
		if campaignFilter != "" && campaignFilter != campaignValue {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(link.Slug + " " + link.URL + " " + link.Channel + " " + link.Campaign)
			if !strings.Contains(haystack, query) {
				continue
			}
		}

		groupKey := channelValue
		label := strings.TrimSpace(link.Channel)
		if link.OfferHome {
			groupKey, label = offerHomepageLinkFilter, "Homepage link"
		} else if channelValue == offerUnassignedFilter {
			label = "Unassigned"
		}
		group, ok := groupsByKey[groupKey]
		if !ok {
			group = &offerLinkGroup{Key: groupKey, Label: label, Open: link.OfferHome || query != "" || channelFilter != "" || campaignFilter != ""}
			groupsByKey[groupKey] = group
			groupKeys = append(groupKeys, groupKey)
		}
		group.Links = append(group.Links, link)
		group.Clicks += link.Clicks
	}
	sort.SliceStable(groupKeys, func(i, j int) bool {
		if groupKeys[i] == offerHomepageLinkFilter {
			return groupKeys[j] != offerHomepageLinkFilter
		}
		if groupKeys[j] == offerHomepageLinkFilter {
			return false
		}
		return strings.ToLower(groupsByKey[groupKeys[i]].Label) < strings.ToLower(groupsByKey[groupKeys[j]].Label)
	})
	groups := make([]offerLinkGroup, 0, len(groupKeys))
	for _, key := range groupKeys {
		groups = append(groups, *groupsByKey[key])
	}
	return groups, channelOptions, campaignOptions
}

func (s *Server) renderOffersDashboard(w http.ResponseWriter, r *http.Request, data offersDashboardData) {
	data.AdminPath = s.cfg.AdminPath
	data.PublicURL = s.publicURL(r)
	data.Production = s.cfg.Production
	data.Today = time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	if data.Offer != nil {
		if data.Error != "" && r.Method == http.MethodPost {
			data.Offer.PriceInput = r.FormValue("product_price")
		} else {
			data.Offer.PriceInput = productPriceInput(data.Offer.PriceMinor)
		}
	}
	if data.Offer != nil && !data.IsNew {
		data.LinkQuery = strings.TrimSpace(r.URL.Query().Get("q"))
		data.LinkChannel = strings.TrimSpace(r.URL.Query().Get("channel"))
		data.LinkCampaign = strings.TrimSpace(r.URL.Query().Get("campaign"))
		data.LinkGroups, data.LinkChannels, data.LinkCampaigns = groupOfferLinks(data.Links, data.LinkQuery, data.LinkChannel, data.LinkCampaign)
	}
	s.renderTemplate(w, "offers-dashboard", data)
}

func (s *Server) handleOffersDashboard(w http.ResponseWriter, r *http.Request) {
	offers, err := s.db.ListOffers()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	query, kind := strings.TrimSpace(r.URL.Query().Get("q")), r.URL.Query().Get("item_type")
	visibility := r.URL.Query().Get("visibility")
	if visibility != "shown" && visibility != "hidden" {
		visibility = ""
	}
	var slugMatches map[int64]bool
	if query != "" {
		slugMatches, err = s.db.OfferIDsMatchingSlug(query)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
	}
	filtered := make([]db.Offer, 0, len(offers))
	for _, offer := range offers {
		if kind != "" && offer.ItemType != kind {
			continue
		}
		if visibility == "shown" && !offer.Active || visibility == "hidden" && offer.Active {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(offer.Title+" "+offer.Provider+" "+offer.Category), strings.ToLower(query)) && !slugMatches[offer.ID] {
			continue
		}
		filtered = append(filtered, offer)
	}
	offers = filtered
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	activeCount := 0
	var totalClicks int64
	for _, offer := range offers {
		if offerStatus(offer, today) == "active" {
			activeCount++
		}
		totalClicks += offer.Clicks
	}
	s.renderOffersDashboard(w, r, offersDashboardData{Offers: offers, CatalogQuery: query, ItemType: kind, Visibility: visibility, ActiveCount: activeCount, TotalClicks: totalClicks})
}

func (s *Server) handleNewOffer(w http.ResponseWriter, r *http.Request) {
	today := time.Now().In(s.cfg.ReportLocation()).Format(dashboardDateLayout)
	offer := &db.Offer{ItemType: "referral", Active: true, ProgramStatus: "active", EndedBehavior: "notice", ButtonLabel: "View offer", StartsOn: today}
	s.renderOffersDashboard(w, r, offersDashboardData{Offer: offer, IsForm: true, IsNew: true})
}

func (s *Server) handleOfferDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid offer id", http.StatusBadRequest)
		return
	}
	offer, err := s.db.GetOfferByID(id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if offer == nil {
		http.NotFound(w, r)
		return
	}
	links, err := s.db.ListOfferLinks(id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.renderOffersDashboard(w, r, offersDashboardData{Offer: offer, Links: links, IsForm: true, Error: r.URL.Query().Get("error"), Saved: r.URL.Query().Get("saved") == "1"})
}

// parseOfferForm parses a urlencoded or multipart offer form with a bounded
// body size, so a large image upload cannot exhaust memory. It reports whether
// the request is usable.

func (s *Server) parseOfferForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseMultipartForm(maxFormBytes); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return false
	}
	return true
}

func (s *Server) handleCreateOffer(w http.ResponseWriter, r *http.Request) {
	if !s.parseOfferForm(w, r) {
		return
	}
	input, inputErr := parseOfferInput(r)
	slug := strings.TrimSpace(r.FormValue("home_slug"))
	urlValue := strings.TrimSpace(r.FormValue("home_url"))
	reuse := r.FormValue("reuse_home_slug") == "1"
	offer := offerFromInput(input, 0, slug, urlValue, true)
	if inputErr == nil {
		if err := validateOfferSlug(slug, s.cfg.AdminPath); err != nil {
			inputErr = &offerFieldError{Field: "home_slug", Message: err.Error()}
		}
	}
	if inputErr == nil && !reuse && urlValue == "" && input.ItemType == "product" && input.Product != nil && input.Product.WhatsApp != nil && input.Product.WhatsApp.Enabled {
		urlValue = "https://wa.me/" + input.Product.WhatsApp.Number
		offer.HomeURL = urlValue
	}
	if inputErr == nil && !reuse && !validOfferDestination(urlValue) {
		inputErr = offerFieldErrorf("home_url", "affiliate URL must start with http:// or https://")
	}
	if inputErr == nil {
		if imageURL, err := s.applyUploadedImage(r); err != nil {
			inputErr = &offerFieldError{Field: "image_file", Message: err.Error()}
		} else if imageURL != "" {
			input.ImageURL = imageURL
			offer.ImageURL = imageURL
		}
	}
	if inputErr != nil {
		message, fieldErrors := offerErrorData(inputErr)
		s.renderOffersDashboard(w, r, offersDashboardData{Offer: offer, ReuseHomeSlug: reuse, IsForm: true, IsNew: true, Error: message, FieldErrors: fieldErrors})
		return
	}
	var created *db.Offer
	var err error
	if reuse {
		created, err = s.db.CreateOfferWithExistingHomepageLink(input, slug)
	} else {
		created, err = s.db.CreateOfferWithHomepageLink(input, slug, urlValue)
	}
	if err != nil {
		msg := "failed to create offer"
		if reuse {
			msg = "existing slug must be active and not already attached to an offer"
		}
		if strings.Contains(err.Error(), "UNIQUE") {
			msg = "slug already exists"
		}
		data := offersDashboardData{Offer: offer, ReuseHomeSlug: reuse, IsForm: true, IsNew: true, Error: msg}
		if reuse || strings.Contains(err.Error(), "UNIQUE") {
			data.FieldErrors = map[string]string{"home_slug": msg}
		}
		s.renderOffersDashboard(w, r, data)
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(created.ID, 10)+"?saved=1", http.StatusSeeOther)
}

func (s *Server) handleUpdateOffer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid offer id", http.StatusBadRequest)
		return
	}
	current, err := s.db.GetOfferByID(id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if current == nil {
		http.NotFound(w, r)
		return
	}
	if !s.parseOfferForm(w, r) {
		return
	}
	input, inputErr := parseOfferInput(r)
	offer := offerFromInput(input, id, current.HomeSlug, current.HomeURL, current.HomeActive)
	offer.WhatsAppClicks = current.WhatsAppClicks
	offer.DetailViews = current.DetailViews
	if input.Product == nil || input.Product.WhatsApp == nil {
		offer.WhatsAppEnabled, offer.WhatsAppNumber, offer.WhatsAppMessage = current.WhatsAppEnabled, current.WhatsAppNumber, current.WhatsAppMessage
	}
	if inputErr != nil {
		links, _ := s.db.ListOfferLinks(id)
		message, fieldErrors := offerErrorData(inputErr)
		s.renderOffersDashboard(w, r, offersDashboardData{Offer: offer, Links: links, IsForm: true, Error: message, FieldErrors: fieldErrors})
		return
	}
	if imageURL, err := s.applyUploadedImage(r); err != nil {
		links, _ := s.db.ListOfferLinks(id)
		message, fieldErrors := offerErrorData(&offerFieldError{Field: "image_file", Message: err.Error()})
		s.renderOffersDashboard(w, r, offersDashboardData{Offer: offer, Links: links, IsForm: true, Error: message, FieldErrors: fieldErrors})
		return
	} else if imageURL != "" {
		input.ImageURL = imageURL
	}
	if err := s.db.UpdateOffer(id, input); err != nil {
		http.Error(w, "failed to update offer", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10)+"?saved=1", http.StatusSeeOther)
}

func (s *Server) handleCreateOfferLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid offer id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	urlValue := strings.TrimSpace(r.FormValue("url"))
	channel := strings.ToLower(strings.TrimSpace(r.FormValue("channel")))
	campaign := strings.ToLower(strings.TrimSpace(r.FormValue("campaign")))
	if err := validateOfferSlug(slug, s.cfg.AdminPath); err != nil {
		http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	if channel == "" {
		http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10)+"?error=channel+is+required+and+must+be+120+characters+or+fewer", http.StatusSeeOther)
		return
	}
	if err := validateLinkMetadata("", channel, campaign); err != nil {
		http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10)+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	offer, err := s.db.GetOfferByID(id)
	if err != nil || offer == nil {
		http.NotFound(w, r)
		return
	}
	if urlValue == "" {
		urlValue = offer.HomeURL
	}
	if !validOfferDestination(urlValue) {
		http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10)+"?error=affiliate+URL+must+start+with+http%3A%2F%2F+or+https%3A%2F%2F", http.StatusSeeOther)
		return
	}
	if _, err := s.db.CreateOfferLink(id, slug, urlValue, channel, campaign); err != nil {
		message := "failed to add link"
		if strings.Contains(err.Error(), "UNIQUE") {
			message = "slug already exists"
		}
		http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10)+"?error="+url.QueryEscape(message), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *Server) handleSetOfferHomepageLink(w http.ResponseWriter, r *http.Request) {
	offerID, offerErr := strconv.ParseInt(r.PathValue("id"), 10, 64)
	linkID, linkErr := strconv.ParseInt(r.PathValue("linkID"), 10, 64)
	if offerErr != nil || linkErr != nil || offerID <= 0 || linkID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.db.SetOfferHomepageLink(offerID, linkID); err != nil {
		http.Error(w, "link does not belong to this offer", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/offers/"+strconv.FormatInt(offerID, 10), http.StatusSeeOther)
}

func (s *Server) handleToggleOffer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid offer id", http.StatusBadRequest)
		return
	}
	if err := s.db.ToggleOffer(id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	query := url.Values{}
	for _, key := range []string{"q", "item_type", "visibility"} {
		if value := strings.TrimSpace(r.FormValue(key)); value != "" {
			query.Set(key, value)
		}
	}
	target := "/" + s.cfg.AdminPath + "/offers"
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
