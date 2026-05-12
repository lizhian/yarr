package worker

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/nkanaev/yarr/src/storage"
)

func testStorage(t *testing.T) *storage.Storage {
	t.Helper()
	log.SetOutput(io.Discard)
	db, err := storage.New(":memory:")
	log.SetOutput(os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDiscoverFeedWithLinkPreservesStoredLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, rssBody("Test Feed"))
	}))
	defer server.Close()

	result, err := DiscoverFeedWithLink(server.URL+"/bilibili/weekly", "rsshub://bilibili/weekly")
	if err != nil {
		t.Fatal(err)
	}
	if result.FeedLink != "rsshub://bilibili/weekly" {
		t.Fatalf("got %q", result.FeedLink)
	}
}

func TestListItemsResolvesRSSHubLink(t *testing.T) {
	requestPath := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, rssBody("RSSHub Feed"))
	}))
	defer server.Close()

	db := testStorage(t)
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": server.URL}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	feed := db.CreateFeed("RSSHub Feed", "", "", "rsshub://bilibili/weekly", nil)

	items, err := listItems(*feed, db)
	if err != nil {
		t.Fatal(err)
	}
	if requestPath != "/bilibili/weekly" {
		t.Fatalf("got request path %q", requestPath)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items", len(items))
	}
}

func TestListItemsRequiresRSSHubBaseURL(t *testing.T) {
	db := testStorage(t)
	feed := db.CreateFeed("RSSHub Feed", "", "", "rsshub://bilibili/weekly", nil)

	_, err := listItems(*feed, db)
	if err == nil {
		t.Fatal("expected error")
	}
}

func rssBody(title string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>` + title + `</title>
    <link>https://example.com</link>
    <item>
      <title>Article</title>
      <link>https://example.com/article</link>
      <guid>article-1</guid>
    </item>
  </channel>
</rss>`
}
