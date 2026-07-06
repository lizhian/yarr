package storage

import (
	"testing"
	"time"
)

func TestGeneratedRSSTablesExist(t *testing.T) {
	db := testDB()
	for _, table := range []string{"generated_rss_sources", "generated_rss_items"} {
		var name string
		err := db.db.QueryRow(`select name from sqlite_master where type = 'table' and name = ?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("missing table %s: %s", table, err)
		}
	}
}

func TestUpsertGeneratedRSSSource(t *testing.T) {
	db := testDB()
	first := db.UpsertGeneratedRSSSource("source", "Title", "https://example.com", "Description")
	if first == nil {
		t.Fatal("expected source")
	}

	second := db.UpsertGeneratedRSSSource("source", "New Title", "https://example.com/new", "New Description")
	if second == nil {
		t.Fatal("expected updated source")
	}
	if second.Id != first.Id {
		t.Fatalf("expected same source id, got %d and %d", first.Id, second.Id)
	}
	if second.Title != "New Title" || second.Link != "https://example.com/new" || second.Description != "New Description" {
		t.Fatalf("source not updated: %#v", second)
	}
}

func TestUpsertGeneratedRSSItem(t *testing.T) {
	db := testDB()
	db.UpsertGeneratedRSSSource("source", "Title", "https://example.com", "Description")
	publishedAt := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	if !db.UpsertGeneratedRSSItem("source", "guid", "Title", "https://example.com/a", "Content", publishedAt) {
		t.Fatal("failed to insert item")
	}
	if !db.UpsertGeneratedRSSItem("source", "guid", "Updated", "https://example.com/b", "New Content", publishedAt.Add(time.Hour)) {
		t.Fatal("failed to update item")
	}

	items := db.ListGeneratedRSSItems("source", 10)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Updated" || items[0].Content != "New Content" {
		t.Fatalf("item not updated: %#v", items[0])
	}
}

func TestListGeneratedRSSItemsNewestFirst(t *testing.T) {
	db := testDB()
	db.UpsertGeneratedRSSSource("source", "Title", "https://example.com", "Description")
	older := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)

	db.UpsertGeneratedRSSItem("source", "older", "Older", "https://example.com/older", "Older", older)
	db.UpsertGeneratedRSSItem("source", "newer", "Newer", "https://example.com/newer", "Newer", newer)

	items := db.ListGeneratedRSSItems("source", 10)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].GUID != "newer" || items[1].GUID != "older" {
		t.Fatalf("items not sorted newest first: %#v", items)
	}
}
