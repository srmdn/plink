package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/disintegration/imaging"
	encoder "github.com/gen2brain/webp"
	_ "golang.org/x/image/webp"
)

const (
	maxUploadBytes      = 5 << 20
	maxStoredImageBytes = 1_000_000
	maxUploadPixels     = 25_000_000
	maxUploadDim        = 6000
	maxFormBytes        = maxUploadBytes + 1<<20
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

// Animated formats must not silently lose frames when converted to a still image.
func animatedImage(data []byte, ext string) bool {
	if ext == ".png" && len(data) >= 8 {
		for off := 8; off+12 <= len(data); {
			n := uint64(binary.BigEndian.Uint32(data[off : off+4]))
			if n > uint64(len(data)-off-12) {
				return false
			}
			if string(data[off+4:off+8]) == "acTL" {
				return true
			}
			off += 12 + int(n)
		}
	}
	if ext == ".webp" && len(data) >= 12 {
		for off := 12; off+8 <= len(data); {
			n := uint64(binary.LittleEndian.Uint32(data[off+4 : off+8]))
			if n > uint64(len(data)-off-8) {
				return false
			}
			kind := string(data[off : off+4])
			if kind == "ANIM" || kind == "ANMF" || (kind == "VP8X" && n > 0 && data[off+8]&0x02 != 0) {
				return true
			}
			step := 8 + int(n) + int(n&1)
			if step > len(data)-off {
				return false
			}
			off += step
		}
	}
	return false
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
	if animatedImage(data, ext) {
		return "", fmt.Errorf("animated images are not supported; choose a still image")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxUploadDim || cfg.Height > maxUploadDim || int64(cfg.Width)*int64(cfg.Height) > maxUploadPixels {
		return "", fmt.Errorf("the image dimensions are not supported")
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return "", fmt.Errorf("the image could not be decoded")
	}
	var encoded bytes.Buffer
	if err := encoder.Encode(&encoded, img, encoder.Options{Lossless: true}); err != nil {
		return "", fmt.Errorf("the image could not be converted to WebP")
	}
	if encoded.Len() > maxStoredImageBytes {
		for _, quality := range []int{85, 75, 65, 55} {
			encoded.Reset()
			if err := encoder.Encode(&encoded, img, encoder.Options{Quality: quality}); err != nil {
				return "", fmt.Errorf("the image could not be converted to WebP")
			}
			if encoded.Len() <= maxStoredImageBytes {
				break
			}
		}
	}
	if encoded.Len() > maxStoredImageBytes {
		return "", fmt.Errorf("the image could not be compressed below 1 MB; try a smaller image")
	}
	data = encoded.Bytes()
	if err := os.MkdirAll(s.uploadsDir, 0o755); err != nil {
		return "", fmt.Errorf("image storage is unavailable")
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:]) + ".webp"
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

func (s *Server) handleArticleMediaUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseMultipartForm(maxFormBytes); err != nil {
		http.Error(w, "The image must be 5 MB or smaller", http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	path, err := s.applyUploadedFile(r, "image_file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if path == "" {
		http.Error(w, "Choose an image", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": path})
}

// Limit multipart forms before CSRF validation, which may itself parse a form.
func (s *Server) limitArticleBody(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxFormBytes {
			http.Error(w, "The request is too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
		next(w, r)
	}
}
