package storage

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestBackupTablesExportsApplicationTables(t *testing.T) {
	db := testDB()
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
	db.SetAuthConfig(true, "username", "password")
	db.SetHTTPState(feed.Id, "modified", "etag")
	db.SetFeedError(feed.Id, errString("failed"))
	db.SetFeedSize(feed.Id, 10)
	db.SyncSearch()

	tables, err := db.BackupTables()
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	wantNames := append([]string(nil), backupTables...)
	sort.Strings(wantNames)
	if len(names) != len(wantNames) {
		t.Fatalf("got %d tables, want %d", len(names), len(wantNames))
	}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("got tables %#v, want %#v", names, wantNames)
	}
	if _, ok := tables["search"]; ok {
		t.Fatal("search table should not be exported")
	}
	if _, ok := tables["sqlite_sequence"]; ok {
		t.Fatal("sqlite internal table should not be exported")
	}

	settings := tables["settings"]
	if len(settings) == 0 {
		t.Fatal("expected settings rows")
	}
	for _, row := range settings {
		if row["key"] == authPasswordKey && row["val"] != "password" {
			t.Fatalf("auth password setting should be decoded, got %#v", row["val"])
		}
	}
}

type errString string

func (e errString) Error() string {
	return string(e)
}
