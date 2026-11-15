package server

import (
	"strings"
	"testing"
)

func TestArticleCaptionEscapedAndOrdinaryTitlePreserved(t *testing.T) {
	body := renderMarkdown("![Diagram](/media/test.webp \"plink-caption:Example & <unsafe>\")\n\n![Other](/media/other.webp \"Tooltip\")")
	html := string(body)
	if !strings.Contains(html, `<figcaption>Example &amp; &lt;unsafe&gt;</figcaption>`) {
		t.Fatalf("caption missing or not escaped: %s", html)
	}
	if !strings.Contains(html, `title="Tooltip"`) {
		t.Fatalf("ordinary title changed: %s", html)
	}
}
