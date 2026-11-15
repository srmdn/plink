package plink

import (
	"bytes"
	"crypto/rand"
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

func settingsUploadRequest(t *testing.T, cookies []*http.Cookie, fields map[string]string, fileField, fileName string, fileData []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	base := map[string]string{"site_name": "Test", "featured_limit": "3", "hero_title": "Beranda", "service_title": "Jasa"}
	for k, v := range fields {
		base[k] = v
	}
	for k, v := range base {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if fileName != "" {
		fw, err := mw.CreateFormFile(fileField, fileName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(fileData); err != nil {
			t.Fatal(err)
		}
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/admin/settings", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, c := range cookies {
		req.AddCookie(c)
		if c.Name == "plink_csrf" {
			req.Header.Set("X-CSRF-Token", c.Value)
		}
	}
	return req
}

func TestSettingsAssetUpload(t *testing.T) {
	database, handler, cfg := uploadTestServer(t)
	cookies := loginAs(t, handler, cfg)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, settingsUploadRequest(t, cookies, nil, "logo_file", "logo.png", testPNG(t)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("settings logo upload: %d %s", rec.Code, rec.Body.String())
	}
	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(settings.LogoURL, "/media/") || !strings.HasSuffix(settings.LogoURL, ".webp") {
		t.Fatalf("logo url = %q, want /media/<hash>.webp", settings.LogoURL)
	}
	name := strings.TrimPrefix(settings.LogoURL, "/media/")
	media := httptest.NewRecorder()
	handler.ServeHTTP(media, httptest.NewRequest("GET", "/media/"+name, nil))
	if media.Code != http.StatusOK {
		t.Fatalf("uploaded logo media returned %d", media.Code)
	}
	favicon := httptest.NewRecorder()
	handler.ServeHTTP(favicon, httptest.NewRequest("GET", "/favicon.svg", nil))
	if favicon.Code != http.StatusFound || favicon.Header().Get("Location") != settings.LogoURL {
		t.Fatalf("favicon fallback: %d %q", favicon.Code, favicon.Header().Get("Location"))
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, settingsUploadRequest(t, cookies, nil, "share_image_file", "note.txt", []byte("this is not an image")))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "PNG") {
		t.Fatalf("invalid share image should re-render, got %d", rec.Code)
	}
	saved, _ := database.GetSiteSettings()
	if saved.LogoURL != settings.LogoURL || saved.ShareImageURL != settings.ShareImageURL {
		t.Fatal("invalid upload modified saved assets")
	}
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
	if !strings.HasPrefix(offer.ImageURL, "/media/") || !strings.HasSuffix(offer.ImageURL, ".webp") {
		t.Fatalf("image url = %q, want /media/<hash>.webp", offer.ImageURL)
	}

	name := strings.TrimPrefix(offer.ImageURL, "/media/")
	media := httptest.NewRecorder()
	handler.ServeHTTP(media, httptest.NewRequest("GET", "/media/"+name, nil))
	if media.Code != http.StatusOK || media.Header().Get("Content-Type") != "image/webp" || media.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("media response: %d %q nosniff=%q", media.Code, media.Header().Get("Content-Type"), media.Header().Get("X-Content-Type-Options"))
	}
	if media.Body.Len() > 1_000_000 {
		t.Fatalf("stored image exceeds 1 MB: %d bytes", media.Body.Len())
	}
	if _, format, err := image.Decode(bytes.NewReader(media.Body.Bytes())); err != nil || format != "webp" {
		t.Fatalf("stored image is not decodable WebP: format=%q err=%v", format, err)
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

func TestLargeImageIsStoredUnderOneMegabyte(t *testing.T) {
	database, handler, cfg := uploadTestServer(t)
	img := image.NewNRGBA(image.Rect(0, 0, 800, 800))
	if _, err := rand.Read(img.Pix); err != nil {
		t.Fatal(err)
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	var input bytes.Buffer
	if err := png.Encode(&input, img); err != nil {
		t.Fatal(err)
	}
	if input.Len() <= 1_000_000 {
		t.Fatalf("input too small: %d", input.Len())
	}
	fields := map[string]string{"title": "Large image", "home_slug": "large-image", "home_url": "https://example.com"}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, createOfferRequest(t, loginAs(t, handler, cfg), fields, "image.png", input.Bytes(), true))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	offer, err := database.GetOfferByID(1)
	if err != nil || offer == nil {
		t.Fatalf("offer lookup: %v", err)
	}
	media := httptest.NewRecorder()
	handler.ServeHTTP(media, httptest.NewRequest("GET", offer.ImageURL, nil))
	if media.Code != http.StatusOK || media.Body.Len() > 1_000_000 {
		t.Fatalf("stored image: status %d, bytes %d", media.Code, media.Body.Len())
	}
	if _, format, err := image.Decode(bytes.NewReader(media.Body.Bytes())); err != nil || format != "webp" {
		t.Fatalf("stored image: format %q, error %v", format, err)
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

func TestArticleMediaUploadRequiresAuthAndCSRF(t *testing.T) {
	_, handler, cfg := uploadTestServer(t)
	makeRequest := func(cookies []*http.Cookie, withToken bool) *http.Request {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		part, err := mw.CreateFormFile("image_file", "diagram.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(testPNG(t)); err != nil {
			t.Fatal(err)
		}
		if err = mw.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/admin/articles/media", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		for _, c := range cookies {
			req.AddCookie(c)
			if withToken && c.Name == "plink_csrf" {
				req.Header.Set("X-CSRF-Token", c.Value)
			}
		}
		return req
	}
	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, makeRequest(nil, false))
	if unauth.Code != http.StatusFound && unauth.Code != http.StatusSeeOther && unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload: %d", unauth.Code)
	}
	cookies := loginAs(t, handler, cfg)
	noToken := httptest.NewRecorder()
	handler.ServeHTTP(noToken, makeRequest(cookies, false))
	if noToken.Code != http.StatusForbidden {
		t.Fatalf("upload without CSRF: %d", noToken.Code)
	}
	valid := httptest.NewRecorder()
	handler.ServeHTTP(valid, makeRequest(cookies, true))
	if valid.Code != http.StatusOK || !strings.Contains(valid.Body.String(), ".webp") {
		t.Fatalf("valid upload: %d %s", valid.Code, valid.Body.String())
	}
}
