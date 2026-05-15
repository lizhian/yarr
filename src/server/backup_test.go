package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/storage"
)

func TestBackupServiceWritesOPMLAndJSON(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "storage.db")
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	folder := db.CreateFolder("folder")
	db.CreateFeed("root feed", "", "https://example.com", "https://example.com/root.xml", nil)
	db.CreateFeed("folder feed", "", "https://example.com/folder", "https://example.com/folder.xml", &folder.Id)

	backups := NewBackupService(db, dbPath)
	backups.now = func() time.Time {
		return time.Date(2026, 5, 15, 1, 2, 3, 0, time.FixedZone("CST", 8*60*60))
	}
	result, err := backups.Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.FeedCount != 2 {
		t.Fatalf("got %d feeds", result.FeedCount)
	}
	if result.TableCounts["feeds"] != 2 {
		t.Fatalf("got %d feed rows", result.TableCounts["feeds"])
	}

	backupDir := filepath.Join(dir, "backups", "2026-05-15")
	if result.Path != backupDir {
		t.Fatalf("got path %q, want %q", result.Path, backupDir)
	}
	opmlBody, err := os.ReadFile(filepath.Join(backupDir, backupOPMLFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(opmlBody) != BuildOPML(db).OPML() {
		t.Fatal("backup opml should match export opml")
	}

	jsonBody, err := os.ReadFile(filepath.Join(backupDir, backupJSONFile))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Version   int                                 `json:"version"`
		CreatedAt string                              `json:"created_at"`
		Tables    map[string][]map[string]interface{} `json:"tables"`
	}
	if err := json.Unmarshal(jsonBody, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != 1 {
		t.Fatalf("got version %d", payload.Version)
	}
	if payload.CreatedAt != "2026-05-15T01:02:03+08:00" {
		t.Fatalf("got created_at %q", payload.CreatedAt)
	}
	if _, ok := payload.Tables["feeds"]; !ok {
		t.Fatal("feeds table missing")
	}
	if _, ok := payload.Tables["search"]; ok {
		t.Fatal("search table should not be exported")
	}
}

func TestBackupEndpointTriggersBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "storage.db")
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)

	s := NewServer(db, "127.0.0.1:8000")
	backups := NewBackupService(db, dbPath)
	backups.now = func() time.Time {
		return time.Date(2026, 5, 15, 0, 0, 0, 0, time.Local)
	}
	s.SetBackupService(backups)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/backups", nil)
	s.handler().ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	var result BackupResult
	if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.FeedCount != 1 {
		t.Fatalf("got %d feeds", result.FeedCount)
	}
	if result.TableCounts["feeds"] != 1 {
		t.Fatalf("got %d feed rows", result.TableCounts["feeds"])
	}
	if _, err := os.Stat(filepath.Join(dir, "backups", "2026-05-15", backupJSONFile)); err != nil {
		t.Fatal(err)
	}
}

func TestOPMLExportMatchesBackupOPML(t *testing.T) {
	db := testServerDB(t)
	folder := db.CreateFolder("folder")
	db.CreateFeed("root feed", "", "https://example.com", "https://example.com/root.xml", nil)
	db.CreateFeed("folder feed", "", "https://example.com/folder", "https://example.com/folder.xml", &folder.Id)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/opml/export", nil)
	NewServer(db, "127.0.0.1:8000").handler().ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	got := recorder.Body.String()
	want := BuildOPML(db).OPML()
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("opml mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestBackupDBPath(t *testing.T) {
	tests := []struct {
		name    string
		dbPath  string
		want    string
		wantErr bool
	}{
		{name: "plain path", dbPath: "/tmp/storage.db", want: "/tmp/storage.db"},
		{name: "path with params", dbPath: "/tmp/storage.db?_journal=WAL", want: "/tmp/storage.db"},
		{name: "file uri", dbPath: "file:/tmp/storage.db?cache=shared", want: "/tmp/storage.db"},
		{name: "memory", dbPath: ":memory:", wantErr: true},
		{name: "file memory", dbPath: "file::memory:?cache=shared", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := backupDBPath(test.dbPath)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
