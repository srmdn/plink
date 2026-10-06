package server

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Parsed fonts are immutable; each image receives its own drawing faces.
var ogFonts = sync.OnceValues(func() (*opentype.Font, *opentype.Font) {
	regular, _ := opentype.Parse(goregular.TTF)
	bold, _ := opentype.Parse(gobold.TTF)
	return regular, bold
})

func ogColor(value string) color.RGBA {
	n, _ := strconv.ParseUint(strings.TrimPrefix(value, "#"), 16, 32)
	return color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255}
}

func ogWrapped(text string, face font.Face, width, count int) []string {
	words := strings.Fields(text)
	var lines []string
	line := ""
	for _, word := range words {
		candidate := strings.TrimSpace(line + " " + word)
		if font.MeasureString(face, candidate).Ceil() > width && line != "" {
			lines = append(lines, line)
			line = word
		} else {
			line = candidate
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	if len(lines) > count {
		lines = lines[:count]
		lines[count-1] += "…"
	}
	for i, line := range lines {
		runes := []rune(line)
		if font.MeasureString(face, line).Ceil() > width {
			for len(runes) > 0 && font.MeasureString(face, string(runes)+"…").Ceil() > width {
				runes = runes[:len(runes)-1]
			}
			lines[i] = string(runes) + "…"
		}
	}
	return lines
}
func (s *Server) handleOGImage(w http.ResponseWriter, r *http.Request) {
	settings, err := s.currentSiteSettings()
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	normalizePalette(&settings)
	title, description := seoIdentity(settings)
	switch r.URL.Query().Get("view") {
	case "services":
		title = settings.ServiceTitle
		description = settings.ServiceDescription
	case "categories":
		title = "Katalog pilihan"
	case "resources":
		title = "Resource pilihan"
	}
	host := "Plink"
	if parsed, e := url.Parse(s.publicURL(r)); e == nil {
		host = parsed.Host
	}
	cacheKey := fmt.Sprintf("%x", sha256.Sum256([]byte(title+"\x00"+description+"\x00"+host+"\x00"+settings.PublicAccent+settings.PublicBackground+settings.LogoURL+settings.HeroEyebrow)))
	s.ogCacheMu.Lock()
	if body, ok := s.ogCache[cacheKey]; ok {
		s.ogCacheMu.Unlock()
		writeOGImage(w, body)
		return
	}
	regular, bold := ogFonts()
	heading, _ := opentype.NewFace(bold, &opentype.FaceOptions{Size: 64, DPI: 72, Hinting: font.HintingFull})
	defer heading.Close()
	copyFace, _ := opentype.NewFace(regular, &opentype.FaceOptions{Size: 30, DPI: 72, Hinting: font.HintingFull})
	defer copyFace.Close()
	small, _ := opentype.NewFace(bold, &opentype.FaceOptions{Size: 24, DPI: 72, Hinting: font.HintingFull})
	defer small.Close()
	img := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	fill := func(rect image.Rectangle, c color.Color) {
		draw.Draw(img, rect, image.NewUniform(c), image.Point{}, draw.Src)
	}
	accent := ogColor(settings.PublicAccent)
	fill(img.Bounds(), ogColor(settings.PublicBackground))
	fill(image.Rect(0, 0, 16, 630), accent)
	ink := color.RGBA{23, 30, 29, 255}
	muted := color.RGBA{82, 96, 91, 255}
	text := func(value string, x, y int, face font.Face, c color.Color) {
		d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x, y)}
		d.DrawString(value)
	}
	text(strings.ToUpper(settings.HeroEyebrow), 64, 85, small, accent)
	for i, line := range ogWrapped(title, heading, 840, 3) {
		text(line, 64, 185+i*76, heading, ink)
	}
	for i, line := range ogWrapped(description, copyFace, 850, 2) {
		text(line, 64, 440+i*40, copyFace, muted)
	}
	for _, line := range ogWrapped(host, small, 900, 1) {
		text(line, 64, 566, small, muted)
	}
	if logo := s.ogLogo(settings.LogoURL); logo != nil {
		bounds := logo.Bounds()
		width, height := 126, 126
		if bounds.Dx() > bounds.Dy() {
			height = 126 * bounds.Dy() / bounds.Dx()
		} else {
			width = 126 * bounds.Dx() / bounds.Dy()
		}
		if width < 1 {
			width = 1
		}
		if height < 1 {
			height = 1
		}
		rect := image.Rect(1010+(126-width)/2, 64+(126-height)/2, 1010+(126-width)/2+width, 64+(126-height)/2+height)
		xdraw.CatmullRom.Scale(img, rect, logo, bounds, draw.Over, nil)
	} else {
		fill(image.Rect(1010, 64, 1136, 190), accent)
		white := color.RGBA{255, 255, 255, 255}
		fill(image.Rect(1040, 90, 1108, 99), white)
		fill(image.Rect(1099, 90, 1108, 158), white)
		for i := 0; i < 61; i++ {
			fill(image.Rect(1040+i, 150-i, 1050+i, 160-i), white)
		}
	}
	var body bytes.Buffer
	if err := png.Encode(&body, img); err != nil {
		s.ogCacheMu.Unlock()
		http.Error(w, "image error", 500)
		return
	}
	// Keep only a few current variants; arbitrary query strings cannot grow the cache.
	if len(s.ogCache) >= 4 || s.ogCache == nil {
		s.ogCache = make(map[string][]byte)
	}
	s.ogCache[cacheKey] = body.Bytes()
	s.ogCacheMu.Unlock()
	writeOGImage(w, body.Bytes())
}

func writeOGImage(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.Write(body)
}
