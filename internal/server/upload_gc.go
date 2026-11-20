package server

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/srmdn/plink/internal/db"
)

// Uploads are content-addressed files written before the draft that uses them
// is saved, so an abandoned draft leaves a file nothing references. The sweep
// waits 48 hours so an image uploaded mid-draft is never collected, and runs
// daily like the session and rate-limit cleanups.
const (
	uploadGracePeriod  = 48 * time.Hour
	uploadCleanupEvery = 24 * time.Hour
)

// mediaReferencePattern finds a stored media path inside any text value, which
// covers bare cover/image columns and Markdown bodies that embed an image.
var mediaReferencePattern = regexp.MustCompile(`/media/([a-f0-9]{64}\.(?:png|jpg|jpeg|webp))`)

func addMediaReferences(references map[string]bool, values ...string) {
	for _, value := range values {
		for _, match := range mediaReferencePattern.FindAllStringSubmatch(value, -1) {
			references[match[1]] = true
		}
	}
}

// referencedMediaNames is the GC inventory: every stored location whose value
// can point at a file under /media/. A read failure aborts the sweep so a
// partial inventory can never be mistaken for "unreferenced".
func referencedMediaNames(database *db.DB) (map[string]bool, error) {
	references := make(map[string]bool)

	offers, err := database.ListOffers()
	if err != nil {
		return nil, err
	}
	for _, offer := range offers {
		addMediaReferences(references, offer.ImageURL, offer.FallbackURL, offer.NoticeSourceURL)
	}

	articles, err := database.ListArticles()
	if err != nil {
		return nil, err
	}
	for _, article := range articles {
		addMediaReferences(references, article.CoverURL, article.BodyMD)
	}

	links, err := database.ListLinks()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		addMediaReferences(references, link.URL)
	}

	settings, err := database.GetSiteSettings()
	if err != nil {
		return nil, err
	}
	addMediaReferences(references,
		settings.LogoURL,
		settings.FaviconURL,
		settings.AvatarURL,
		settings.ShareImageURL,
		settings.SocialLinks,
		settings.ContactLinks,
	)
	return references, nil
}

// CleanupOrphanUploads deletes stored media older than the grace period that no
// current row references. It returns the removed file names; one failing file
// is reported but never stops the remaining files from being collected.
func CleanupOrphanUploads(database *db.DB, uploadsDir string, now time.Time) ([]string, error) {
	if uploadsDir == "" {
		return nil, nil
	}
	references, err := referencedMediaNames(database)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(uploadsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var removed []string
	var firstErr error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !mediaNamePattern.MatchString(name) || references[name] {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if now.Sub(info.ModTime()) <= uploadGracePeriod {
			continue
		}
		if err := os.Remove(filepath.Join(uploadsDir, name)); err != nil {
			if !errors.Is(err, os.ErrNotExist) && firstErr == nil {
				firstErr = err
			}
			continue
		}
		removed = append(removed, name)
	}
	return removed, firstErr
}

// runUploadCleanup sweeps once at startup and then once a day for the life of
// the process.
func (s *Server) runUploadCleanup() {
	s.cleanupUploadsOnce()
	for range time.Tick(uploadCleanupEvery) {
		s.cleanupUploadsOnce()
	}
}

func (s *Server) cleanupUploadsOnce() {
	removed, err := CleanupOrphanUploads(s.db, s.uploadsDir, time.Now())
	if len(removed) > 0 {
		log.Printf("plink: removed %d orphan upload(s): %s", len(removed), strings.Join(removed, ", "))
	}
	if err != nil {
		log.Printf("plink: clean orphan uploads: %v", err)
	}
}
