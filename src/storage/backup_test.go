package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBackupToWritesSQLiteSnapshot(t *testing.T) {
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "storage.db"))
	if err != nil {
		t.Fatal(err)
	}
	folder := db.CreateFolder("folder")
	feed := db.CreateFeed("feed", "description", "https://example.com", "https://example.com/feed.xml", &folder.Id)
	db.CreateItems([]Item{{
		GUID:    "item",
		FeedId:  feed.Id,
		Title:   "title",
		Date:    time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC),
		Content: "content",
	}})
	db.UpdateSettings(map[string]interface{}{"toolbar_display": "text"})

	backupPath := filepath.Join(dir, "backup.db")
	if err := db.BackupTo(backupPath); err != nil {
		t.Fatal(err)
	}

	backup, err := New(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	feeds := backup.ListFeeds()
	if len(feeds) != 1 || feeds[0].Title != "feed" {
		t.Fatalf("invalid backed up feeds: %#v", feeds)
	}
	items := backup.ListItems(ItemFilter{FeedID: &feeds[0].Id}, 10, true, true)
	if len(items) != 1 || items[0].Content != "content" {
		t.Fatalf("invalid backed up items: %#v", items)
	}
}
