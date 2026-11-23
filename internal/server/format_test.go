package server

import (
	"strings"
	"testing"
)

func TestInitialsOf(t *testing.T) {
	cases := map[string]string{
		"Curlog":                     "CU",
		"Islami.click":               "IC",
		"The supply chain behind AI": "TS",
		"7-Zip":                      "7Z",
		"Éclair":                     "ÉC",
		"":                           "",
		"   ":                        "",
		"!!!":                        "",
	}
	for title, want := range cases {
		if got := initialsOf(title); got != want {
			t.Errorf("initialsOf(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestArticleCardMeta(t *testing.T) {
	body := strings.Repeat("word ", 400)
	if got, want := articleCardMeta("2026-11-10", body), "Nov 10, 2026 · 2 min read"; got != want {
		t.Errorf("dated article meta = %q, want %q", got, want)
	}
	if got, want := articleCardMeta("", body), "2 min read"; got != want {
		t.Errorf("undated article meta = %q, want %q", got, want)
	}
}
