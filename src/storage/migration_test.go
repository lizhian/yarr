package storage

import (
	"database/sql"
	"testing"
	"time"
)

func TestMigrationAddsFeedRefreshTimes(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for version := int64(1); version <= 24; version++ {
		if err := migrateVersion(version, db); err != nil {
			t.Fatalf("migrate to version %d: %v", version, err)
		}
	}
	refreshedResult, err := db.Exec(`insert into feeds (title, feed_link) values ('refreshed', 'https://example.com/refreshed.xml')`)
	if err != nil {
		t.Fatal(err)
	}
	refreshedID, _ := refreshedResult.LastInsertId()
	emptyResult, err := db.Exec(`insert into feeds (title, feed_link) values ('empty', 'https://example.com/empty.xml')`)
	if err != nil {
		t.Fatal(err)
	}
	emptyID, _ := emptyResult.LastInsertId()
	historicalTime := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`
		insert into http_states (feed_id, last_refreshed, last_modified, etag)
		values (?, ?, '', '')`, refreshedID, historicalTime); err != nil {
		t.Fatal(err)
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var lastRefreshedAt, lastRefreshSucceededAt *time.Time
	if err := db.QueryRow(`
		select last_refreshed_at, last_refresh_succeeded_at from feeds where id = ?`,
		refreshedID,
	).Scan(&lastRefreshedAt, &lastRefreshSucceededAt); err != nil {
		t.Fatal(err)
	}
	if lastRefreshedAt == nil || !lastRefreshedAt.Equal(historicalTime) {
		t.Fatalf("last_refreshed_at got %#v", lastRefreshedAt)
	}
	if lastRefreshSucceededAt == nil || !lastRefreshSucceededAt.Equal(historicalTime) {
		t.Fatalf("last_refresh_succeeded_at got %#v", lastRefreshSucceededAt)
	}
	if err := db.QueryRow(`
		select last_refreshed_at, last_refresh_succeeded_at from feeds where id = ?`,
		emptyID,
	).Scan(&lastRefreshedAt, &lastRefreshSucceededAt); err != nil {
		t.Fatal(err)
	}
	if lastRefreshedAt != nil || lastRefreshSucceededAt != nil {
		t.Fatalf("empty feed refresh times got %#v, %#v", lastRefreshedAt, lastRefreshSucceededAt)
	}
	assertSchemaObjectExists(t, db, "index", "idx_feed_last_refresh_succeeded_at")
}

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

func TestMigrationAddsFeedCustomIcon(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for version := int64(1); version <= 22; version++ {
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

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var customIcon bool
	if err := db.QueryRow(`select custom_icon from feeds where id = ?`, feedID).Scan(&customIcon); err != nil {
		t.Fatal(err)
	}
	if customIcon {
		t.Fatal("custom_icon should default to false")
	}
}

func TestMigrationSeparatesFavoriteFromReadStatus(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for version := int64(1); version <= 23; version++ {
		if err := migrateVersion(version, db); err != nil {
			t.Fatalf("migrate to version %d: %v", version, err)
		}
	}
	folderResult, err := db.Exec(`insert into folders (title) values ('folder')`)
	if err != nil {
		t.Fatal(err)
	}
	folderID, _ := folderResult.LastInsertId()
	feedResult, err := db.Exec(`insert into feeds (folder_id, title, feed_link) values (?, 'feed', 'https://example.com/feed.xml')`, folderID)
	if err != nil {
		t.Fatal(err)
	}
	feedID, _ := feedResult.LastInsertId()
	searchResult, err := db.Exec(`insert into search (title, description, content) values ('title', '', 'searchable')`)
	if err != nil {
		t.Fatal(err)
	}
	searchRowID, _ := searchResult.LastInsertId()
	for status := 0; status <= 2; status++ {
		rowID := interface{}(nil)
		if status == 2 {
			rowID = searchRowID
		}
		if _, err := db.Exec(`insert into items (guid, feed_id, status, search_rowid) values (?, ?, ?, ?)`, status, feedID, status, rowID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`insert into settings (key, val) values ('filter', '"starred"')`); err != nil {
		t.Fatal(err)
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`select status, favorite from items order by id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	wantStatus := []int{0, 1, 1}
	wantFavorite := []bool{false, false, true}
	for index := 0; rows.Next(); index++ {
		var status int
		var favorite bool
		if err := rows.Scan(&status, &favorite); err != nil {
			t.Fatal(err)
		}
		if index >= len(wantStatus) || status != wantStatus[index] || favorite != wantFavorite[index] {
			t.Fatalf("row %d got status=%d favorite=%v", index, status, favorite)
		}
	}

	var unreadFirst, sortNewestFirst bool
	if err := db.QueryRow(`select unread_first, sort_newest_first from feeds where id = ?`, feedID).Scan(&unreadFirst, &sortNewestFirst); err != nil {
		t.Fatal(err)
	}
	if !unreadFirst || !sortNewestFirst {
		t.Fatal("feed order defaults should be enabled")
	}
	if err := db.QueryRow(`select unread_first, sort_newest_first from folders where id = ?`, folderID).Scan(&unreadFirst, &sortNewestFirst); err != nil {
		t.Fatal(err)
	}
	if !unreadFirst || !sortNewestFirst {
		t.Fatal("folder order defaults should be enabled")
	}
	var filter string
	if err := db.QueryRow(`select val from settings where key = 'filter'`).Scan(&filter); err != nil {
		t.Fatal(err)
	}
	if filter != `"favorite"` {
		t.Fatalf("filter got %q", filter)
	}
	var preservedSearchRowID int64
	if err := db.QueryRow(`select search_rowid from items where guid = '2'`).Scan(&preservedSearchRowID); err != nil {
		t.Fatal(err)
	}
	if preservedSearchRowID != searchRowID {
		t.Fatalf("search_rowid got %d, want %d", preservedSearchRowID, searchRowID)
	}
	assertSchemaObjectExists(t, db, "table", "search")
	assertSchemaObjectExists(t, db, "trigger", "del_item_search")
	assertSchemaObjectExists(t, db, "index", "idx_item_favorite_status_date_id")
	assertSchemaObjectExists(t, db, "index", "idx_item_feed_favorite_status_date_id")
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
