package server

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadRotatesJPEGExifOrientation(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 50), G: uint8(y * 90), A: 255})
		}
	}
	var raw bytes.Buffer
	if err := jpeg.Encode(&raw, img, nil); err != nil {
		t.Fatal(err)
	}
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 0x01, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	var input bytes.Buffer
	input.Write(raw.Bytes()[:2])
	input.Write([]byte{0xff, 0xe1})
	_ = binary.Write(&input, binary.BigEndian, uint16(len(exif)+2))
	input.Write(exif)
	input.Write(raw.Bytes()[2:])
	s := &Server{uploadsDir: t.TempDir()}
	path, err := s.storeImage(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(s.uploadsDir, strings.TrimPrefix(path, "/media/")))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 2 || cfg.Height != 4 {
		t.Fatalf("orientation: got %dx%d, want 2x4", cfg.Width, cfg.Height)
	}
}

func TestAnimatedFormatsRejected(t *testing.T) {
	pngData := append([]byte("\x89PNG\r\n\x1a\n"), []byte{0, 0, 0, 0, 'a', 'c', 'T', 'L', 0, 0, 0, 0}...)
	webpData := append([]byte("RIFF\x16\x00\x00\x00WEBP"), []byte{'V', 'P', '8', 'X', 1, 0, 0, 0, 2, 0}...)
	if !animatedImage(pngData, ".png") {
		t.Fatal("APNG not detected")
	}
	if !animatedImage(webpData, ".webp") {
		t.Fatal("animated WebP not detected")
	}
}

func TestArticleRequestLimitBeforeHandler(t *testing.T) {
	s := &Server{}
	reached := false
	handler := s.limitArticleBody(func(w http.ResponseWriter, r *http.Request) { reached = true })
	req := httptest.NewRequest(http.MethodPost, "/articles", nil)
	req.ContentLength = maxFormBytes + 1
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge || reached {
		t.Fatalf("oversized request: status %d reached=%v", rec.Code, reached)
	}
}
