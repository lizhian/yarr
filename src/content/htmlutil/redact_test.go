package htmlutil

import (
	"strings"
	"testing"
)

func TestRedactDiscordToken(t *testing.T) {
	for _, text := range []string{`Get "http://localhost/base/rss/discord/channel/123/secret": timeout`, `http://localhost/rss/discord/channel/123/Bot secret`, `https://host/rss/discord/channel/123/secret%2Fpart?x=1`} {
		got := RedactDiscordToken(text)
		if strings.Contains(got, "secret") || !strings.Contains(got, "[redacted]") {
			t.Fatal(got)
		}
	}
	original := "https://example.com/feed.xml: timeout"
	if RedactDiscordToken(original) != original {
		t.Fatal("unrelated error changed")
	}
}
