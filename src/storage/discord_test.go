package storage

import (
	"bytes"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscordCacheMigrationPersistenceAndCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for v := int64(1); v <= 25; v++ {
		if err := migrateVersion(v, raw); err != nil {
			t.Fatal(err)
		}
	}
	raw.Close()
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	original := DiscordCache{Key: "123:hash", RSS: []byte("<rss/>"), SucceededAt: 100, ExpiresAt: 200, AccessedAt: 300, Failure: "forbidden", FailureAt: 210, RetryAt: 400}
	if err := s.SaveDiscordCache(original); err != nil {
		t.Fatal(err)
	}
	if err := s.TouchDiscordCache(original.Key, 500); err != nil {
		t.Fatal(err)
	}
	// A refresh with an older access timestamp must not undo a concurrent visit.
	if err := s.SaveDiscordCache(original); err != nil {
		t.Fatal(err)
	}
	s.db.Close()
	s, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	got, err := s.GetDiscordCache(original.Key)
	if err != nil || !bytes.Equal(got.RSS, original.RSS) || got.AccessedAt != 500 || got.Failure != "forbidden" || got.RetryAt != 400 {
		t.Fatal(got, err)
	}
	if err := s.SaveDiscordCache(DiscordCache{Key: "discord-rate-limit", RetryAt: 999}); err != nil {
		t.Fatal(err)
	}
	if err := s.CleanDiscordCache(500); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetDiscordCache(original.Key); len(got.RSS) == 0 {
		t.Fatal("deleted active entry")
	}
	if err := s.CleanDiscordCache(501); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetDiscordCache(original.Key); len(got.RSS) != 0 {
		t.Fatal("expired entry retained")
	}
	if got, _ := s.GetDiscordCache("discord-rate-limit"); got.RetryAt != 999 {
		t.Fatal("deleted rate limit")
	}
}
func TestFeedErrorsRedactDiscordToken(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	feed := s.CreateFeedWithContentSelector("Discord", "", "https://discord.com", "http://localhost/rss/discord/channel/123/secret", "", nil)
	s.SetFeedError(feed.Id, errors.New(`Get "http://localhost/rss/discord/channel/123/secret": timeout`))
	got := s.GetFeedErrors()[feed.Id]
	if strings.Contains(got, "secret") || !strings.Contains(got, "[redacted]") {
		t.Fatal(got)
	}
	if s.GetFeed(feed.Id).FeedLink != "http://localhost/rss/discord/channel/123/secret" {
		t.Fatal("subscription URL changed")
	}
}
