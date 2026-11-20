package server

import (
	"errors"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/srmdn/plink/internal/db"
)

// mediaLibraryData is the view model of the admin media page. Files are the
// stored uploads, newest first, each marked with whether any stored row still
// points at it.
type mediaLibraryData struct {
	Files     []mediaLibraryFile
	AdminPath string
	Active    string
}

type mediaLibraryFile struct {
	Name       string
	URL        string
	SizeKB     int64
	ModifiedAt string
	Referenced bool
}

func (s *Server) handleMediaLibrary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	files, err := mediaLibraryFiles(s.db, s.uploadsDir, s.cfg.ReportLocation())
	if err != nil {
		http.Error(w, "media library error", http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, "media-dashboard", mediaLibraryData{
		Files:     files,
		AdminPath: s.cfg.AdminPath,
		Active:    "media",
	})
}

// mediaLibraryFiles lists the stored, content-addressed images and marks them
// with the same reference inventory the orphan sweep uses, so the page and the
// GC can never disagree about what "referenced" means.
func mediaLibraryFiles(database *db.DB, uploadsDir string, location *time.Location) ([]mediaLibraryFile, error) {
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
	if location == nil {
		location = time.Local
	}
	type storedFile struct {
		file    mediaLibraryFile
		modTime time.Time
	}
	stored := make([]storedFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !mediaNamePattern.MatchString(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		stored = append(stored, storedFile{
			file: mediaLibraryFile{
				Name:       name,
				URL:        "/media/" + name,
				SizeKB:     (info.Size() + 1023) / 1024,
				ModifiedAt: info.ModTime().In(location).Format("2006-01-02 15:04"),
				Referenced: references[name],
			},
			modTime: info.ModTime(),
		})
	}
	// Newest first, with the content-addressed name as a stable tie-breaker.
	sort.Slice(stored, func(i, j int) bool {
		if !stored[i].modTime.Equal(stored[j].modTime) {
			return stored[i].modTime.After(stored[j].modTime)
		}
		return stored[i].file.Name > stored[j].file.Name
	})
	files := make([]mediaLibraryFile, len(stored))
	for i := range stored {
		files[i] = stored[i].file
	}
	return files, nil
}
