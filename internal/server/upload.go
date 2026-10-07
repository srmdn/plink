package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	_ "golang.org/x/image/webp"
)

const (
	maxUploadBytes  = 5 << 20
	maxUploadPixels = 25_000_000
	maxUploadDim    = 6000
	maxFormBytes    = maxUploadBytes + 1<<20
)

// Uploaded files are served by name only; the name is a content hash plus a
// safe raster extension, so it can never escape the uploads directory.
var mediaNamePattern = regexp.MustCompile(`^[a-f0-9]{64}\.(png|jpg|jpeg|webp)$`)

func uploadExt(data []byte) (string, bool) {
	switch http.DetectContentType(data) {
	case "image/png":
		return ".png", true
	case "image/jpeg":
		return ".jpg", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

// storeImage validates an uploaded raster image and writes it under a
// content-addressed name. It returns the public /media path.
func (s *Server) storeImage(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("the image file is empty")
	}
	if len(data) > maxUploadBytes {
		return "", fmt.Errorf("the image must be 5 MB or smaller")
	}
	ext, ok := uploadExt(data)
	if !ok {
		return "", fmt.Errorf("the image must be a PNG, JPEG, or WebP file")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxUploadDim || cfg.Height > maxUploadDim || int64(cfg.Width)*int64(cfg.Height) > maxUploadPixels {
		return "", fmt.Errorf("the image dimensions are not supported")
	}
	if err := os.MkdirAll(s.uploadsDir, 0o755); err != nil {
		return "", fmt.Errorf("image storage is unavailable")
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:]) + ext
	dst := filepath.Join(s.uploadsDir, name)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return "", fmt.Errorf("the image could not be saved")
		}
	}
	return "/media/" + name, nil
}

// applyUploadedImage stores the optional image_file field from a form and
// returns its public path, or an empty string when no file was submitted.
func (s *Server) applyUploadedImage(r *http.Request) (string, error) {
	return s.applyUploadedFile(r, "image_file")
}

// applyUploadedFile stores the optional named file field from a form and
// returns its public path, or an empty string when no file was submitted.
func (s *Server) applyUploadedFile(r *http.Request, field string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", nil
	}
	defer file.Close()
	if header == nil || header.Filename == "" {
		return "", nil
	}
	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		return "", fmt.Errorf("the image could not be read")
	}
	return s.storeImage(data)
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mediaNamePattern.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(s.uploadsDir, name))
}
