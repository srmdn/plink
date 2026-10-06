package plink

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/srmdn/plink/internal/config"
	"github.com/srmdn/plink/internal/db"
	"github.com/srmdn/plink/internal/server"
)

func uploadTestServer(t *testing.T) (*db.DB, http.Handler, *config.Config) {
	t.Helper()
	database, err := db.Init(filepath.Join(t.TempDir(), "upload.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	cfg := &config.Config{AdminPath: "admin", AdminPassword: "test-password", SiteName: "Test", Timezone: "Asia/Jakarta", UploadsDir: t.TempDir()}
	return database, server.New(cfg, database, webFS), cfg
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func loginAs(t *testing.T, handler http.Handler, cfg *config.Config) []*http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{"password": {cfg.AdminPassword}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	return rec.Result().Cookies()
}

func createOfferRequest(t *testing.T, cookies []*http.Cookie, fields map[string]string, fileName string, fileData []byte, withCSRF bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if fileName != "" {
		fw, err := mw.CreateFormFile("image_file", fileName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(fileData); err != nil {
			t.Fatal(err)
		}
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/admin/offers", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, c := range cookies {
		if withCSRF || c.Name != "plink_csrf" {
			req.AddCookie(c)
		}
		if withCSRF && c.Name == "plink_csrf" {
			req.Header.Set("X-CSRF-Token", c.Value)
		}
	}
	return req
}

func TestOfferImageUpload(t *testing.T) {
	database, handler, cfg := uploadTestServer(t)
	cookies := loginAs(t, handler, cfg)

	fields := map[string]string{"title": "Produk unggulan", "item_type": "product", "home_slug": "produk-unggulan", "home_url": "https://shop.example/p/1", "button_label": "Lihat", "active": "1"}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, createOfferRequest(t, cookies, fields, "produk.png", testPNG(t), true))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create with image: %d %s", rec.Code, rec.Body.String())
	}

	offer, err := database.GetOfferByID(1)
	if err != nil || offer == nil {
		t.Fatalf("offer lookup: %v", err)
	}
	if !strings.HasPrefix(offer.ImageURL, "/media/") || !strings.HasSuffix(offer.ImageURL, ".png") {
		t.Fatalf("image url = %q, want /media/<hash>.png", offer.ImageURL)
	}

	name := strings.TrimPrefix(offer.ImageURL, "/media/")
	media := httptest.NewRecorder()
	handler.ServeHTTP(media, httptest.NewRequest("GET", "/media/"+name, nil))
	if media.Code != http.StatusOK || media.Header().Get("Content-Type") != "image/png" || media.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("media response: %d %q nosniff=%q", media.Code, media.Header().Get("Content-Type"), media.Header().Get("X-Content-Type-Options"))
	}
	if !strings.Contains(media.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("media cache-control = %q", media.Header().Get("Cache-Control"))
	}

	bad := httptest.NewRecorder()
	handler.ServeHTTP(bad, httptest.NewRequest("GET", "/media/notahash.png", nil))
	if bad.Code != http.StatusNotFound {
		t.Fatalf("invalid media name returned %d, want 404", bad.Code)
	}
}

func TestOfferImageUploadRejectsNonImage(t *testing.T) {
	database, handler, cfg := uploadTestServer(t)
	cookies := loginAs(t, handler, cfg)

	fields := map[string]string{"title": "Nope", "home_slug": "nope", "home_url": "https://shop.example/p/2"}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, createOfferRequest(t, cookies, fields, "note.txt", []byte("this is not an image at all"), true))
	if rec.Code != http.StatusOK {
		t.Fatalf("invalid upload should re-render the form, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "PNG") {
		t.Fatal("expected a format error message on the form")
	}
	offers, err := database.ListOffers()
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 0 {
		t.Fatalf("invalid upload created %d offers", len(offers))
	}
}

func TestOfferImageUploadRequiresCSRF(t *testing.T) {
	_, handler, cfg := uploadTestServer(t)
	cookies := loginAs(t, handler, cfg)

	fields := map[string]string{"title": "No csrf", "home_slug": "no-csrf", "home_url": "https://shop.example/p/3"}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, createOfferRequest(t, cookies, fields, "p.png", testPNG(t), false))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("upload without CSRF returned %d, want 403", rec.Code)
	}
}
