package storage

import (
	"database/sql"
	"testing"
	"time"
)

func TestMigrationAddsLatestItemArrivedAt(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for version := int64(1); version <= 21; version++ {
		if err := migrateVersion(version, db); err != nil {
			t.Fatalf("migrate to version %d: %v", version, err)
		}
	}

	result, err := db.Exec(`insert into feeds (title, feed_link) values ('feed', 'https://example.com/feed.xml')`)
	if err != nil {
		t.Fatal(err)
	}
	feedID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	backfilledAt := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`insert into items (guid, feed_id, date_arrived) values ('existing', ?, ?)`, feedID, backfilledAt); err != nil {
		t.Fatal(err)
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	assertFeedLatestItemArrivedAt(t, db, feedID, backfilledAt)
	assertSchemaObjectExists(t, db, "index", "idx_feed_latest_item_arrived_at")
	assertSchemaObjectExists(t, db, "trigger", "update_feed_latest_item_arrived_at")

	latestAt := backfilledAt.Add(time.Hour)
	if _, err := db.Exec(`insert into items (guid, feed_id, date_arrived) values ('new', ?, ?)`, feedID, latestAt); err != nil {
		t.Fatal(err)
	}
	assertFeedLatestItemArrivedAt(t, db, feedID, latestAt)

	if _, err := db.Exec(`
		insert into items (guid, feed_id, date_arrived)
		values ('new', ?, ?)
		on conflict (feed_id, guid) do nothing`, feedID, latestAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertFeedLatestItemArrivedAt(t, db, feedID, latestAt)

	if _, err := db.Exec(`insert into items (guid, feed_id, date_arrived) values ('older', ?, ?)`, feedID, backfilledAt.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`delete from items where feed_id = ?`, feedID); err != nil {
		t.Fatal(err)
	}
	assertFeedLatestItemArrivedAt(t, db, feedID, latestAt)
}

func assertFeedLatestItemArrivedAt(t *testing.T, db *sql.DB, feedID int64, want time.Time) {
	t.Helper()
	var got time.Time
	if err := db.QueryRow(`select latest_item_arrived_at from feeds where id = ?`, feedID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Fatalf("latest_item_arrived_at got %s, want %s", got, want)
	}
}

func assertSchemaObjectExists(t *testing.T, db *sql.DB, objectType, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`select count(*) from sqlite_master where type = ? and name = ?`, objectType, name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected %s %q", objectType, name)
	}
}
