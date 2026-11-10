package server

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/srmdn/plink/internal/db"
)

var (
	errArticleTitleRequired = errors.New("a title is required")
	errArticleDate          = errors.New("the publish date must be a valid date")
	errArticleSlugTaken     = errors.New("that slug is already used by another article")
	errArticleSlugTooLong   = errors.New("the slug must be 120 characters or fewer")
	errArticleSlugChars     = errors.New("the slug may contain only lowercase letters, numbers, hyphens, and underscores")
	errArticleSlugReserved  = errors.New("that slug is reserved by the application")
)

type articlesDashboardData struct {
	AdminPath  string
	Production bool
	Articles   []db.Article
	Published  int
	Drafts     int
}

type articleEditorData struct {
	AdminPath  string
	Production bool
	Article    *db.Article
	IsNew      bool
	Error      string
	Saved      bool
	Preview    template.HTML
}

type articlePageData struct {
	Settings    db.SiteSettings
	SEO         pageSEO
	Article     db.Article
	Body        template.HTML
	Published   string
	ReadingTime int
	CoverURL    string
	SourceURL   string
	SiteName    string
	SiteDesc    string
	AdminPath   string
	IsLoggedIn  bool
	View        string
}

func (s *Server) handleArticlesDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	articles, err := s.db.ListArticles()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	data := articlesDashboardData{AdminPath: s.cfg.AdminPath, Production: s.cfg.Production, Articles: articles}
	for _, a := range articles {
		if a.Status == "published" {
			data.Published++
		} else {
			data.Drafts++
		}
	}
	s.renderTemplate(w, "articles-dashboard", data)
}

func (s *Server) handleNewArticle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	s.renderTemplate(w, "article-editor", articleEditorData{
		AdminPath:  s.cfg.AdminPath,
		Production: s.cfg.Production,
		Article:    &db.Article{Status: "draft"},
		IsNew:      true,
	})
}

func (s *Server) handleArticleEditor(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	article, err := s.db.GetArticleByID(id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if article == nil {
		http.NotFound(w, r)
		return
	}
	s.renderTemplate(w, "article-editor", articleEditorData{
		AdminPath:  s.cfg.AdminPath,
		Production: s.cfg.Production,
		Article:    article,
		Saved:      r.URL.Query().Get("saved") == "1",
	})
}

func (s *Server) handleCreateArticle(w http.ResponseWriter, r *http.Request) {
	input, err := s.parseArticleForm(r)
	if err != nil {
		s.renderTemplate(w, "article-editor", articleEditorData{
			AdminPath: s.cfg.AdminPath, Production: s.cfg.Production,
			Article: articleFromInput(input, 0), IsNew: true, Error: err.Error(),
			Preview: articlePreview(r),
		})
		return
	}
	if r.FormValue("preview") == "1" {
		s.renderTemplate(w, "article-editor", articleEditorData{
			AdminPath: s.cfg.AdminPath, Production: s.cfg.Production,
			Article: articleFromInput(input, 0), IsNew: true, Preview: articlePreview(r),
		})
		return
	}
	article, err := s.db.CreateArticle(input)
	if err != nil {
		s.renderTemplate(w, "article-editor", articleEditorData{
			AdminPath: s.cfg.AdminPath, Production: s.cfg.Production,
			Article: articleFromInput(input, 0), IsNew: true, Error: articleError(err),
		})
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/articles/"+strconv.FormatInt(article.ID, 10)+"?saved=1", http.StatusSeeOther)
}

func (s *Server) handleUpdateArticle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	current, err := s.db.GetArticleByID(id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if current == nil {
		http.NotFound(w, r)
		return
	}
	input, err := s.parseArticleForm(r)
	if err != nil {
		s.renderTemplate(w, "article-editor", articleEditorData{
			AdminPath: s.cfg.AdminPath, Production: s.cfg.Production,
			Article: articleFromInput(input, id), Error: err.Error(), Preview: articlePreview(r),
		})
		return
	}
	if r.FormValue("preview") == "1" {
		s.renderTemplate(w, "article-editor", articleEditorData{
			AdminPath: s.cfg.AdminPath, Production: s.cfg.Production,
			Article: articleFromInput(input, id), Preview: articlePreview(r),
		})
		return
	}
	if err := s.db.UpdateArticle(id, input); err != nil {
		s.renderTemplate(w, "article-editor", articleEditorData{
			AdminPath: s.cfg.AdminPath, Production: s.cfg.Production,
			Article: articleFromInput(input, id), Error: articleError(err),
		})
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/articles/"+strconv.FormatInt(id, 10)+"?saved=1", http.StatusSeeOther)
}

func (s *Server) handleDeleteArticle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if err := s.db.DeleteArticle(id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/articles", http.StatusSeeOther)
}

func (s *Server) handleToggleArticle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	article, err := s.db.GetArticleByID(id)
	if err != nil || article == nil {
		http.NotFound(w, r)
		return
	}
	status := "published"
	if article.Status == "published" {
		status = "draft"
	}
	if err := s.db.UpdateArticle(id, db.ArticleInput{
		Slug: article.Slug, Title: article.Title, Excerpt: article.Excerpt,
		CoverURL: article.CoverURL, BodyMD: article.BodyMD, Status: status,
		SourceURL: article.SourceURL, PublishedAt: article.PublishedAt,
	}); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+s.cfg.AdminPath+"/articles/"+strconv.FormatInt(id, 10)+"?saved=1", http.StatusSeeOther)
}

func (s *Server) parseArticleForm(r *http.Request) (db.ArticleInput, error) {
	input := db.ArticleInput{
		Slug:        strings.TrimSpace(r.FormValue("slug")),
		Title:       strings.TrimSpace(r.FormValue("title")),
		Excerpt:     strings.TrimSpace(r.FormValue("excerpt")),
		CoverURL:    strings.TrimSpace(r.FormValue("cover_url")),
		BodyMD:      strings.TrimSpace(r.FormValue("body_md")),
		Status:      strings.TrimSpace(r.FormValue("status")),
		SourceURL:   strings.TrimSpace(r.FormValue("source_url")),
		PublishedAt: strings.TrimSpace(r.FormValue("published_at")),
	}
	if cover, err := s.applyUploadedFile(r, "cover_file"); err != nil {
		return input, err
	} else if cover != "" {
		input.CoverURL = cover
	}
	if input.Status != "published" {
		input.Status = "draft"
	}
	if input.Title == "" {
		return input, errArticleTitleRequired
	}
	if input.Slug == "" {
		input.Slug = slugify(input.Title)
	}
	if err := validateArticleSlug(input.Slug, s.cfg.AdminPath); err != nil {
		return input, err
	}
	if input.PublishedAt != "" {
		if _, ok := parseDashboardDate(input.PublishedAt); !ok {
			return input, errArticleDate
		}
	}
	existing, err := s.db.GetArticleBySlug(input.Slug)
	if err != nil {
		return input, err
	}
	if existing != nil && strconv.FormatInt(existing.ID, 10) != r.PathValue("id") {
		return input, errArticleSlugTaken
	}
	return input, nil
}

func articleFromInput(input db.ArticleInput, id int64) *db.Article {
	return &db.Article{
		ID: id, Slug: input.Slug, Title: input.Title, Excerpt: input.Excerpt,
		CoverURL: input.CoverURL, BodyMD: input.BodyMD, Status: input.Status,
		SourceURL: input.SourceURL, PublishedAt: input.PublishedAt,
	}
}

func articlePreview(r *http.Request) template.HTML {
	body := strings.TrimSpace(r.FormValue("body_md"))
	if body == "" {
		return ""
	}
	return renderMarkdown(body)
}

func (s *Server) handleArticlePage(w http.ResponseWriter, r *http.Request) {
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if !settings.StorefrontEnabled || !settings.ArticlesEnabled {
		http.NotFound(w, r)
		return
	}
	slug := strings.TrimSpace(r.PathValue("slug"))
	article, err := s.db.GetArticleBySlug(slug)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if article == nil || article.Status != "published" {
		http.NotFound(w, r)
		return
	}
	_ = s.db.IncrementArticleViews(article.ID)

	base := s.publicURL(r)
	excerpt := article.Excerpt
	if excerpt == "" {
		excerpt = articleExcerpt(article.BodyMD, 160)
	}
	cover := ""
	if article.CoverURL != "" {
		if parsed, err := url.Parse(article.CoverURL); err == nil {
			cover = mustBaseURL(base).ResolveReference(parsed).String()
		}
	}
	if cover == "" {
		cover = shareImage(settings, base, "")
	}
	seo := pageSEO{
		Title:       article.Title + " · " + settings.SiteName,
		Description: excerpt,
		URL:         base + "/a/" + article.Slug,
		Image:       cover,
		SiteName:    settings.SiteName,
	}
	s.renderTemplate(w, "article-page", articlePageData{
		Settings:    settings,
		SEO:         seo,
		Article:     *article,
		Body:        renderMarkdown(article.BodyMD),
		Published:   article.PublishedAt,
		ReadingTime: articleReadingMinutes(article.BodyMD),
		CoverURL:    article.CoverURL,
		SourceURL:   article.SourceURL,
		SiteName:    settings.SiteName,
		SiteDesc:    settings.SiteDesc,
		AdminPath:   s.cfg.AdminPath,
		IsLoggedIn:  s.isLoggedIn(r),
		View:        "articles",
	})
}

func validateArticleSlug(slug, adminPath string) error {
	if len(slug) > 120 {
		return errArticleSlugTooLong
	}
	for _, char := range slug {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return errArticleSlugChars
	}
	if isReservedSlug(slug, adminPath) {
		return errArticleSlugReserved
	}
	return nil
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(value) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			b.WriteRune(char)
			lastDash = false
		case char == ' ' || char == '-' || char == '_':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func articleError(err error) string {
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return errArticleSlugTaken.Error()
	}
	return err.Error()
}
