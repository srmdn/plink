package db

import (
	"database/sql"
	"errors"
	"time"
)

// Article is a short native article hosted by Plink itself. Drafts are visible
// only in the admin; published articles get a canonical page on the site.
type Article struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Excerpt     string `json:"excerpt"`
	CoverURL    string `json:"cover_url"`
	BodyMD      string `json:"body_md"`
	Status      string `json:"status"`
	SourceURL   string `json:"source_url"`
	PublishedAt string `json:"published_at"`
	Views       int64  `json:"views"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type ArticleInput struct {
	Slug        string
	Title       string
	Excerpt     string
	CoverURL    string
	BodyMD      string
	Status      string
	SourceURL   string
	PublishedAt string
}

const articleColumns = `id, slug, title, excerpt, cover_url, body_md, status, source_url, published_at, views, created_at, updated_at`

func scanArticle(row interface{ Scan(...any) error }) (*Article, error) {
	var a Article
	err := row.Scan(&a.ID, &a.Slug, &a.Title, &a.Excerpt, &a.CoverURL, &a.BodyMD,
		&a.Status, &a.SourceURL, &a.PublishedAt, &a.Views, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (db *DB) ListArticles() ([]Article, error) {
	rows, err := db.Query(`SELECT ` + articleColumns + ` FROM articles ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	articles := []Article{}
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, *a)
	}
	return articles, rows.Err()
}

func (db *DB) ListPublishedArticles() ([]Article, error) {
	rows, err := db.Query(`SELECT ` + articleColumns + ` FROM articles WHERE status = 'published' ORDER BY published_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	articles := []Article{}
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, *a)
	}
	return articles, rows.Err()
}

func (db *DB) GetArticleByID(id int64) (*Article, error) {
	row := db.QueryRow(`SELECT `+articleColumns+` FROM articles WHERE id = ?`, id)
	a, err := scanArticle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (db *DB) GetArticleBySlug(slug string) (*Article, error) {
	row := db.QueryRow(`SELECT `+articleColumns+` FROM articles WHERE slug = ?`, slug)
	a, err := scanArticle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (db *DB) CreateArticle(in ArticleInput) (*Article, error) {
	now := time.Now().Unix()
	publishedAt := in.PublishedAt
	if in.Status == "published" && publishedAt == "" {
		publishedAt = time.Now().Format("2006-01-02")
	}
	result, err := db.Exec(`INSERT INTO articles
		(slug, title, excerpt, cover_url, body_md, status, source_url, published_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Slug, in.Title, in.Excerpt, in.CoverURL, in.BodyMD, in.Status, in.SourceURL, publishedAt, now, now)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetArticleByID(id)
}

func (db *DB) UpdateArticle(id int64, in ArticleInput) error {
	publishedAt := in.PublishedAt
	if in.Status == "published" && publishedAt == "" {
		publishedAt = time.Now().Format("2006-01-02")
	}
	_, err := db.Exec(`UPDATE articles SET
		slug = ?, title = ?, excerpt = ?, cover_url = ?, body_md = ?, status = ?,
		source_url = ?, published_at = ?, updated_at = ?
		WHERE id = ?`,
		in.Slug, in.Title, in.Excerpt, in.CoverURL, in.BodyMD, in.Status,
		in.SourceURL, publishedAt, time.Now().Unix(), id)
	return err
}

func (db *DB) DeleteArticle(id int64) error {
	_, err := db.Exec(`DELETE FROM articles WHERE id = ?`, id)
	return err
}

func (db *DB) IncrementArticleViews(id int64) error {
	_, err := db.Exec(`UPDATE articles SET views = views + 1 WHERE id = ?`, id)
	return err
}
