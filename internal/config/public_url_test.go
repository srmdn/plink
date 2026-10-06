package config

import "testing"

func TestPublicURLOrigins(t *testing.T) {
	for _, raw := range []string{"https://example.com/path", "https://u:p@example.com", "https://example.com?q=1", "https://example.com/#a", "//example.com", "javascript:alert(1)", "https://example.com:99999", "https://example.com:", "https://..", "https://-bad.test", "https://example.com\\path"} {
		if _, err := NormalizePublicURL(raw, false); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	for _, raw := range []string{"https://example.com", "https://example.com/", "http://127.0.0.1:18096", "http://[::1]:8080", ""} {
		if _, err := NormalizePublicURL(raw, false); err != nil {
			t.Errorf("rejected %q: %v", raw, err)
		}
	}
	if _, err := NormalizePublicURL("http://example.com", true); err == nil {
		t.Fatal("production HTTP accepted")
	}
	if got, err := NormalizePublicURL(" https://EXAMPLE.com/ ", true); err != nil || got != "https://example.com" {
		t.Fatal("normalization incorrect")
	}
}
