package server

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

// The renderer escapes raw HTML by default, so a document cannot inject markup.
// Links with an active scheme are neutralised below.
var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

var (
	unsafeHref       = regexp.MustCompile(`(?i)(href|src)="(?:javascript|data|vbscript):[^"]*"`)
	markdownHeading  = regexp.MustCompile(`(?m)^#{1,6}\s*`)
	markdownImage    = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	markdownLink     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	markdownMarkers  = regexp.MustCompile("[*_`>#~]")
	markdownLineNote = regexp.MustCompile(`(?m)^\s*[-+]\s+`)
)

func renderMarkdown(source string) template.HTML {
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(source), &buf); err != nil {
		return template.HTML("<p>This article could not be rendered.</p>")
	}
	return template.HTML(unsafeHref.ReplaceAllString(buf.String(), `$1="#"`))
}

// articleExcerpt turns markdown into a short plain-text summary. It is only a
// fallback for articles without an explicit excerpt.
func articleExcerpt(body string, limit int) string {
	text := markdownHeading.ReplaceAllString(body, "")
	text = markdownImage.ReplaceAllString(text, "")
	text = markdownLink.ReplaceAllString(text, "$1")
	text = markdownLineNote.ReplaceAllString(text, "")
	text = markdownMarkers.ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= limit {
		return text
	}
	cut := text[:limit]
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}

func articleReadingMinutes(body string) int {
	words := len(strings.Fields(body))
	minutes := words / 200
	if words%200 != 0 {
		minutes++
	}
	if minutes < 1 {
		minutes = 1
	}
	return minutes
}
