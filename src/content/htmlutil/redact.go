package htmlutil

import "regexp"

// Include decoded spaces and slashes: callers may supply an unescaped URL or
// an error containing one. Prefer hiding excess error text to exposing a token.
var discordCredential = regexp.MustCompile(`(/rss/discord/channel/[^/\s]+/)[^\r\n"<>?#]+`)

func RedactDiscordToken(text string) string {
	return discordCredential.ReplaceAllString(text, "${1}[redacted]")
}
