package server

import "testing"

func TestPreviewDetectionDoesNotCaptureSearchBotsOrBrowsers(t *testing.T) {
	for _, agent := range []string{"", "Mozilla/5.0", "Googlebot/2.1", "bingbot/2.0", "DuckDuckBot", "Mozilla/5.0 Instagram 300", "Slack/4.0", "Telegram/10.0", "facebookexternalhit/1.1", "WhatsApp/2.0", "Discordbot/2.0"} {
		want := agent == "facebookexternalhit/1.1" || agent == "WhatsApp/2.0" || agent == "Discordbot/2.0"
		if got := isSocialPreviewBot(agent); got != want {
			t.Errorf("agent %q: got %t, want %t", agent, got, want)
		}
	}
}
