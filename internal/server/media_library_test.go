package server

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
)

func libraryMediaName(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:]) + ".webp"
}

func writeLibraryMedia(t *testing.T, dir, name string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("stored image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func TestMediaLibraryFilesMarksReferencesAndOrder(t *testing.T) {
	database, err := db.Init(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	uploadsDir := t.TempDir()
	now := time.Now()
	offerImage := libraryMediaName("media-library-offer")
	articleImage := libraryMediaName("media-library-article")
	orphanImage := libraryMediaName("media-library-orphan")
	for name, modTime := range map[string]time.Time{
		offerImage:   now.Add(-3 * time.Hour),
		articleImage: now.Add(-2 * time.Hour),
		orphanImage:  now.Add(-1 * time.Hour),
	} {
		writeLibraryMedia(t, uploadsDir, name, modTime)
	}
	// Files that are not content-addressed uploads stay out of the listing.
	writeLibraryMedia(t, uploadsDir, "notes.txt", now)
	if err := os.MkdirAll(filepath.Join(uploadsDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := database.CreateOfferWithHomepageLink(db.OfferInput{
		Title: "Offer", ImageURL: "/media/" + offerImage, Active: true,
	}, "media-offer", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateArticle(db.ArticleInput{
		Slug: "media-article", Title: "Article", Status: "draft",
		BodyMD: "![Shot](/media/" + articleImage + ")",
	}); err != nil {
		t.Fatal(err)
	}

	files, err := mediaLibraryFiles(database, uploadsDir, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("files = %d, want 3: %#v", len(files), files)
	}
	if files[0].Name != orphanImage || files[1].Name != articleImage || files[2].Name != offerImage {
		t.Fatalf("order = %v, %v, %v; want newest first", files[0].Name, files[1].Name, files[2].Name)
	}
	for _, file := range files {
		if file.URL != "/media/"+file.Name || file.SizeKB < 1 || file.ModifiedAt == "" {
			t.Fatalf("file metadata incomplete: %#v", file)
		}
		wantReferenced := file.Name != orphanImage
		if file.Referenced != wantReferenced {
			t.Fatalf("%s referenced = %v, want %v", file.Name, file.Referenced, wantReferenced)
		}
	}
	if files[0].ModifiedAt != now.Add(-1*time.Hour).UTC().Format("2006-01-02 15:04") {
		t.Fatalf("modified at = %q", files[0].ModifiedAt)
	}

	missing, err := mediaLibraryFiles(database, filepath.Join(t.TempDir(), "absent"), time.UTC)
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing directory = %v, %v", missing, err)
	}
	empty, err := mediaLibraryFiles(database, "", time.UTC)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty uploads dir = %v, %v", empty, err)
	}
}

func TestMediaLibraryHandlerRendersContractFields(t *testing.T) {
	database, err := db.Init(filepath.Join(t.TempDir(), "media-page.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	uploadsDir := t.TempDir()
	writeLibraryMedia(t, uploadsDir, libraryMediaName("media-page"), time.Now())

	server := &Server{
		cfg:        &config.Config{AdminPath: "admin"},
		db:         database,
		settings:   &settingsCache{},
		uploadsDir: uploadsDir,
	}
	server.tmpl = template.Must(template.New("").Parse(
		`{{define "media-dashboard"}}{{.AdminPath}}|{{.Active}}|{{range .Files}}{{.Name}};{{.URL}};{{.SizeKB}};{{.ModifiedAt}};{{.Referenced}}{{end}}{{end}}`,
	))

	response := httptest.NewRecorder()
	server.handleMediaLibrary(response, httptest.NewRequest("GET", "/admin/media", nil))
	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.HasPrefix(body, "admin|media|") {
		t.Fatalf("view model = %q", body)
	}
	if !strings.Contains(body, "/media/") || !strings.Contains(body, ";false") {
		t.Fatalf("file rows missing contract fields: %q", body)
	}
}
