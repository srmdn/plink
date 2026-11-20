package server

import (
	"fmt"
	"math"
	"strings"

	"github.com/srmdn/plink/internal/db"
)

// wcagAAContrast is the WCAG 2.1 AA minimum for normal-size text.
const wcagAAContrast = 4.5

// relativeLuminance implements the WCAG 2.1 definition for an sRGB color.
func relativeLuminance(r, g, b uint8) float64 {
	channel := func(value uint8) float64 {
		c := float64(value) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

// contrastRatio returns the WCAG contrast ratio between two six-digit hex
// colors. The second result is false when a value is not a plain hex color,
// which keeps the caller from warning about palette values it cannot read.
func contrastRatio(foreground, background string) (float64, bool) {
	fg, ok := parseHexRGB(foreground)
	if !ok {
		return 0, false
	}
	bg, ok := parseHexRGB(background)
	if !ok {
		return 0, false
	}
	light, dark := relativeLuminance(fg[0], fg[1], fg[2]), relativeLuminance(bg[0], bg[1], bg[2])
	if light < dark {
		light, dark = dark, light
	}
	return (light + 0.05) / (dark + 0.05), true
}

func parseHexRGB(value string) ([3]uint8, bool) {
	var rgb [3]uint8
	if len(value) != 7 || value[0] != '#' {
		return rgb, false
	}
	for i := 0; i < 3; i++ {
		hi, ok := hexDigit(value[1+i*2])
		if !ok {
			return rgb, false
		}
		lo, ok := hexDigit(value[2+i*2])
		if !ok {
			return rgb, false
		}
		rgb[i] = hi<<4 | lo
	}
	return rgb, true
}

func hexDigit(char byte) (uint8, bool) {
	switch {
	case char >= '0' && char <= '9':
		return char - '0', true
	case char >= 'a' && char <= 'f':
		return char - 'a' + 10, true
	case char >= 'A' && char <= 'F':
		return char - 'A' + 10, true
	}
	return 0, false
}

// themeContrastWarning describes accent-on-background pairs that fall below
// the WCAG AA ratio. The palette is still saved: this is a hint shown on the
// settings page after a successful save, not a validation error.
func themeContrastWarning(settings db.SiteSettings) string {
	scopes := []struct {
		label      string
		accent     string
		background string
	}{
		{"public", settings.PublicAccent, settings.PublicBackground},
		{"admin", settings.AdminAccent, settings.AdminBackground},
	}
	var warnings []string
	for _, scope := range scopes {
		ratio, ok := contrastRatio(scope.accent, scope.background)
		if !ok || ratio >= wcagAAContrast {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("the %s accent %s on background %s is %.1f:1", scope.label, scope.accent, scope.background, ratio))
	}
	if len(warnings) == 0 {
		return ""
	}
	return "Contrast warning: " + strings.Join(warnings, "; ") + ". Settings were saved; choose colors with at least 4.5:1 for readable text."
}
