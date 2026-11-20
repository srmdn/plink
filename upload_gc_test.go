package plink

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/srmdn/plink/internal/db"
	"github.com/srmdn/plink/internal/server"
)

func mediaTestName(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:]) + ".webp"
}

func writeMediaTestFile(t *testing.T, dir, name string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("stored image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupOrphanUploads(t *testing.T) {
	database, err := db.Init(filepath.Join(t.TempDir(), "gc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	uploadsDir := t.TempDir()
	now := time.Now()
	old := now.Add(-49 * time.Hour)

	orphan := mediaTestName("orphan")
	offerImage := mediaTestName("offer-image")
	articleBody := mediaTestName("article-body")
	settingsLogo := mediaTestName("settings-logo")
	fresh := mediaTestName("fresh-orphan")

	writeMediaTestFile(t, uploadsDir, orphan, old)
	writeMediaTestFile(t, uploadsDir, offerImage, old)
	writeMediaTestFile(t, uploadsDir, articleBody, old)
	writeMediaTestFile(t, uploadsDir, settingsLogo, old)
	writeMediaTestFile(t, uploadsDir, fresh, now.Add(-time.Hour))

	if _, err := database.CreateOfferWithHomepageLink(db.OfferInput{Title: "Offer", ImageURL: "/media/" + offerImage, Active: true}, "offer-gc", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateArticle(db.ArticleInput{
		Slug: "article-gc", Title: "Article", Status: "draft",
		BodyMD: "![Diagram](/media/" + articleBody + ")",
	}); err != nil {
		t.Fatal(err)
	}
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.LogoURL = "/media/" + settingsLogo
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}

	removed, err := server.CleanupOrphanUploads(database, uploadsDir, now)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if len(removed) != 1 || removed[0] != orphan {
		t.Fatalf("removed = %v, want only %s", removed, orphan)
	}

	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(uploadsDir, name))
		return err == nil
	}
	if exists(orphan) {
		t.Fatal("old unreferenced upload was not removed")
	}
	for label, name := range map[string]string{
		"offer image_url":   offerImage,
		"article body_md":   articleBody,
		"settings logo_url": settingsLogo,
		"fresh orphan":      fresh,
	} {
		if !exists(name) {
			t.Fatalf("file referenced by %s was removed", label)
		}
	}
}
