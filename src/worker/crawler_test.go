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

func TestFindFeedIconPrefersFeedImage(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
		switch r.URL.Path {
		case "/feed-image.png":
			w.WriteHeader(http.StatusInternalServerError)
		case "/":
			io.WriteString(w, `<html><head><link rel="icon" href="/favicon.ico"></head></html>`)
		case "/favicon.ico":
			w.Header().Set("Content-Type", "image/x-icon")
			w.Write([]byte("\x00\x00\x01\x00favicon"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	iconURL, err := findFeedIconURL(server.URL+"/feed-image.png", server.URL, server.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if iconURL != server.URL+"/feed-image.png" {
		t.Fatalf("got %q", iconURL)
	}
	if len(requested) != 0 {
		t.Fatalf("got requests %#v", requested)
	}
}

func TestFindFeedIconSkipsNonOKSitePage(t *testing.T) {
	const challengeIcon = "challenge-icon"
	requested := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `<html><head><link rel="icon" href="/challenge-icon.png"></head></html>`)
		case "/challenge-icon.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte(challengeIcon))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	iconURL, err := findFeedIconURL("", server.URL, server.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if iconURL != "" {
		t.Fatalf("got %q", iconURL)
	}
	for _, path := range requested {
		if path == "/challenge-icon.png" {
			t.Fatalf("requested challenge icon: %#v", requested)
		}
	}
}

func TestFindFeedIconDoesNotStoreEmptyIcon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			io.WriteString(w, `<html><head><link rel="icon" href="/empty-icon.png"></head></html>`)
		case "/empty-icon.png", "/favicon.ico":
			w.Header().Set("Content-Type", "image/png")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	db := testStorage(t)
	feed := db.CreateFeed("Test Feed", "", server.URL, server.URL+"/feed.xml", nil)
	worker := NewWorker(db)

	worker.findFeedIcon(*feed, "", server.URL+"/feed.xml")

	feed = db.GetFeed(feed.Id)
	if feed.IconURL != "" {
		t.Fatalf("icon url got %q", feed.IconURL)
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

func TestRefreshPreservesSavedFeedMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Fresh Title 的 bilibili 动态</title>
    <link>https://example.com/fresh</link>
    <item>
      <title>Article</title>
      <link>https://example.com/article</link>
      <guid>article-1</guid>
    </item>
  </channel>
</rss>`)
	}))
	defer server.Close()

	db := testStorage(t)
	feed := db.CreateFeed("Stale Title", "", "https://example.com/stale", server.URL+"/feed.xml", nil)
	worker := NewWorker(db)

	worker.refresher([]storage.Feed{*feed})

	feed = db.GetFeed(feed.Id)
	if feed.Title != "Stale Title" {
		t.Fatalf("title got %q", feed.Title)
	}
	if feed.Link != "https://example.com/stale" {
		t.Fatalf("link got %q", feed.Link)
	}
	if feed.FeedLink != server.URL+"/feed.xml" {
		t.Fatalf("feed_link got %q", feed.FeedLink)
	}
}

func TestRefreshFillsWhitespaceTitleDespiteHTTPState(t *testing.T) {
	var ifNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ifNoneMatch = r.Header.Get("If-None-Match")
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Header().Set("Etag", `"new"`)
		io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Fresh Title</title>
    <link>https://example.com/fresh</link>
    <item>
      <title>Article</title>
      <link>https://example.com/article</link>
      <guid>article-1</guid>
    </item>
  </channel>
</rss>`)
	}))
	defer server.Close()

	db := testStorage(t)
	feed := db.CreateFeed("Stale Title", "", "https://example.com/stale", server.URL+"/feed.xml", nil)
	db.RenameFeed(feed.Id, " ")
	db.SetHTTPState(feed.Id, "", `"old"`)
	feed = db.GetFeed(feed.Id)
	worker := NewWorker(db)

	worker.refresher([]storage.Feed{*feed})

	feed = db.GetFeed(feed.Id)
	if ifNoneMatch != "" {
		t.Fatalf("If-None-Match got %q", ifNoneMatch)
	}
	if feed.Title != "Fresh Title" {
		t.Fatalf("title got %q", feed.Title)
	}
	if feed.Link != "https://example.com/stale" {
		t.Fatalf("link got %q", feed.Link)
	}
}

func TestRefreshAddsFeedIconFromImageURLWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, rssBodyWithImage("Test Feed", serverURL(r)+"/icon.png"))
		case "/icon.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("image-icon"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	db := testStorage(t)
	feed := db.CreateFeed("Test Feed", "", server.URL, server.URL+"/feed.xml", nil)
	worker := NewWorker(db)

	worker.refresher([]storage.Feed{*feed})

	feed = db.GetFeed(feed.Id)
	if feed.IconURL != server.URL+"/icon.png" {
		t.Fatalf("icon url got %q", feed.IconURL)
	}
}

func TestRefreshKeepsUserIconURLWhenImageURLChanges(t *testing.T) {
	requestedNewIcon := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, rssBodyWithImage("Test Feed", serverURL(r)+"/new-icon.png"))
		case "/new-icon.png":
			requestedNewIcon = true
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("new-icon"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	db := testStorage(t)
	feed := db.CreateFeed("Test Feed", "", server.URL, server.URL+"/feed.xml", nil)
	db.UpdateFeedIconURL(feed.Id, "https://example.com/custom-icon.png")
	worker := NewWorker(db)

	worker.refresher([]storage.Feed{*feed})

	feed = db.GetFeed(feed.Id)
	if feed.IconURL != "https://example.com/custom-icon.png" {
		t.Fatalf("icon url got %q", feed.IconURL)
	}
	if requestedNewIcon {
		t.Fatal("requested new icon despite user icon url")
	}
}

func TestRefreshAddsFeedIconURLAfterUserClearsIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, rssBodyWithImage("Test Feed", serverURL(r)+"/new-icon.png"))
		case "/new-icon.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("new-icon"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	db := testStorage(t)
	feed := db.CreateFeed("Test Feed", "", server.URL, server.URL+"/feed.xml", nil)
	db.UpdateFeedIconURL(feed.Id, "https://example.com/custom-icon.png")
	db.UpdateFeedIconURL(feed.Id, "")
	worker := NewWorker(db)

	worker.refresher([]storage.Feed{*feed})

	feed = db.GetFeed(feed.Id)
	if feed.IconURL != server.URL+"/new-icon.png" {
		t.Fatalf("icon url got %q", feed.IconURL)
	}
}

func TestRefreshKeepsIconURLEmptyOnFailedFallbackIconUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, rssBody("Test Feed"))
		case "/new-icon.png":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	db := testStorage(t)
	feed := db.CreateFeed("Test Feed", "", server.URL, server.URL+"/feed.xml", nil)
	worker := NewWorker(db)

	worker.refresher([]storage.Feed{*feed})

	feed = db.GetFeed(feed.Id)
	if feed.IconURL != "" {
		t.Fatalf("icon url got %q", feed.IconURL)
	}
}

func TestRefreshFeedIconURLsOverwritesExistingIconURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, rssBodyWithImage("Test Feed", serverURL(r)+"/new-icon.png"))
		case "/new-icon.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("new-icon"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	db := testStorage(t)
	feed := db.CreateFeed("Test Feed", "", server.URL, server.URL+"/feed.xml", nil)
	db.UpdateFeedIconURL(feed.Id, "https://example.com/old-icon.png")
	worker := NewWorker(db)

	worker.RefreshFeedIconURLs()

	feed = db.GetFeed(feed.Id)
	if feed.IconURL != server.URL+"/new-icon.png" {
		t.Fatalf("icon url got %q", feed.IconURL)
	}
}

func TestRefreshFeedIconURLsKeepsExistingIconURLOnFailedFallbackDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, rssBody("Test Feed"))
		case "/new-icon.png":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	db := testStorage(t)
	feed := db.CreateFeed("Test Feed", "", server.URL, server.URL+"/feed.xml", nil)
	db.UpdateFeedIconURL(feed.Id, "https://example.com/old-icon.png")
	worker := NewWorker(db)

	worker.RefreshFeedIconURLs()

	feed = db.GetFeed(feed.Id)
	if feed.IconURL != "https://example.com/old-icon.png" {
		t.Fatalf("icon url got %q", feed.IconURL)
	}
}

func TestRefreshFeedIconURLUpdatesOneFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, rssBodyWithImage("Test Feed", serverURL(r)+"/new-icon.png"))
		case "/new-icon.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("new-icon"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	db := testStorage(t)
	target := db.CreateFeed("Target Feed", "", server.URL, server.URL+"/feed.xml", nil)
	other := db.CreateFeed("Other Feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	db.UpdateFeedIconURL(target.Id, "https://example.com/old-icon.png")
	db.UpdateFeedIconURL(other.Id, "https://example.com/other-icon.png")
	worker := NewWorker(db)

	worker.RefreshFeedIconURL(*target)

	target = db.GetFeed(target.Id)
	other = db.GetFeed(other.Id)
	if target.IconURL != server.URL+"/new-icon.png" {
		t.Fatalf("target icon url got %q", target.IconURL)
	}
	if other.IconURL != "https://example.com/other-icon.png" {
		t.Fatalf("other icon url got %q", other.IconURL)
	}
}

func TestRefreshRSSHubPreservesSavedMetadataAndFeedLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Fresh RSSHub Title - Telegram Channel</title>
    <link>https://example.com/rsshub-site</link>
    <item>
      <title>Article</title>
      <link>https://example.com/article</link>
      <guid>article-1</guid>
    </item>
  </channel>
</rss>`)
	}))
	defer server.Close()

	db := testStorage(t)
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": server.URL}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	feed := db.CreateFeed("Stale Title", "", "https://example.com/stale", "rsshub://telegram/channel/test", nil)
	worker := NewWorker(db)

	worker.refresher([]storage.Feed{*feed})

	feed = db.GetFeed(feed.Id)
	if feed.Title != "Stale Title" {
		t.Fatalf("title got %q", feed.Title)
	}
	if feed.Link != "https://example.com/stale" {
		t.Fatalf("link got %q", feed.Link)
	}
	if feed.FeedLink != "rsshub://telegram/channel/test" {
		t.Fatalf("feed_link got %q", feed.FeedLink)
	}
}

func TestRefreshRSSHubFillsPlaceholderMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Fresh RSSHub Title - Telegram Channel</title>
    <link>https://example.com/rsshub-site</link>
    <item>
      <title>Article</title>
      <link>https://example.com/article</link>
      <guid>article-1</guid>
    </item>
  </channel>
</rss>`)
	}))
	defer server.Close()

	db := testStorage(t)
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": server.URL}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	feed := db.CreateFeed("rsshub://telegram/channel/test", "", "rsshub://telegram/channel/test", "rsshub://telegram/channel/test", nil)
	worker := NewWorker(db)

	worker.refresher([]storage.Feed{*feed})

	feed = db.GetFeed(feed.Id)
	if feed.Title != "Fresh RSSHub Title" {
		t.Fatalf("title got %q", feed.Title)
	}
	if feed.Link != "https://example.com/rsshub-site" {
		t.Fatalf("link got %q", feed.Link)
	}
	if feed.FeedLink != "rsshub://telegram/channel/test" {
		t.Fatalf("feed_link got %q", feed.FeedLink)
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

func TestListItemsTriesMultipleRSSHubBases(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, rssBody("RSSHub Feed"))
	}))
	defer server.Close()

	db := testStorage(t)
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": server.URL + "/bad\n" + server.URL + "/good"}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	feed := db.CreateFeed("RSSHub Feed", "", "", "rsshub://bilibili/weekly", nil)

	items, err := listItems(*feed, db)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("got %d requests", requests)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items", len(items))
	}
}

func TestWorkerUsesAvailableRSSHubBasesFirst(t *testing.T) {
	requestPath := ""
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unavailable base should not be requested")
	}))
	defer serverA.Close()
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, rssBody("RSSHub Feed"))
	}))
	defer serverB.Close()

	db := testStorage(t)
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": serverA.URL + "\n" + serverB.URL}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	worker := NewWorker(db)
	worker.rsshubAvailability[serverB.URL] = rsshubAvailable
	feed := db.CreateFeed("RSSHub Feed", "", "", "rsshub://bilibili/weekly", nil)

	requestLinks, err := worker.resolveLinks(feed.FeedLink)
	if err != nil {
		t.Fatal(err)
	}
	items, err := listItemsFromLinks(*feed, requestLinks, db)
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

func TestWorkerLimitsRSSHubAttempts(t *testing.T) {
	db := testStorage(t)
	bases := ""
	for i := 0; i < RSSHUB_MAX_ATTEMPTS+2; i++ {
		if bases != "" {
			bases += "\n"
		}
		bases += "https://example" + string(rune('a'+i)) + ".com"
	}
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": bases}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	worker := NewWorker(db)

	requestLinks, err := worker.resolveLinks("rsshub://bilibili/weekly")
	if err != nil {
		t.Fatal(err)
	}
	if len(requestLinks) != RSSHUB_MAX_ATTEMPTS {
		t.Fatalf("got %d links", len(requestLinks))
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

func rssBodyWithImage(title, imageURL string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>` + title + `</title>
    <link>https://example.com</link>
    <image>
      <url>` + imageURL + `</url>
    </image>
    <item>
      <title>Article</title>
      <link>https://example.com/article</link>
      <guid>article-1</guid>
    </item>
  </channel>
</rss>`
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
