package worker

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nkanaev/yarr/src/storage"
)

func TestCheckRSSHubBaseAcceptsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/rsshub", http.StatusFound)
	}))
	defer server.Close()

	if got := checkRSSHubBase(server.URL); got != rsshubAvailable {
		t.Fatalf("got %v", got)
	}
}

func TestCheckRSSHubBaseRejectsErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if got := checkRSSHubBase(server.URL); got != rsshubUnavailable {
		t.Fatalf("got %v", got)
	}
}

func TestRSSHubRefreshDetailsCountLatestSuccessfulBasePerFeed(t *testing.T) {
	db := testStorage(t)
	worker := NewWorker(db)
	feedA := db.CreateFeed("A", "", "", "rsshub://bilibili/user/video/a", nil)
	feedB := db.CreateFeed("B", "", "", "rsshub://bilibili/user/video/b", nil)

	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": "https://a.example\nhttps://b.example"}) {
		t.Fatal("failed to set RSSHub base URL")
	}

	worker.recordRSSHubRefreshHit(&FeedRefreshResult{FeedID: feedA.Id, StoredFeedLink: feedA.FeedLink, RSSHubBase: "https://a.example", RSSHubLink: "https://a.example/bilibili/user/video/a"})
	worker.recordRSSHubRefreshHit(&FeedRefreshResult{FeedID: feedB.Id, StoredFeedLink: feedB.FeedLink, RSSHubBase: "https://a.example", RSSHubLink: "https://a.example/bilibili/user/video/b"})
	worker.recordRSSHubRefreshHit(&FeedRefreshResult{FeedID: feedA.Id, StoredFeedLink: feedA.FeedLink, RSSHubBase: "https://b.example", RSSHubLink: "https://b.example/bilibili/user/video/a"})

	details := worker.RSSHubRefreshDetails()
	if len(details) != 2 {
		t.Fatalf("got %d details", len(details))
	}
	if details[0].BaseURL != "https://a.example" || details[0].Feeds != 1 {
		t.Fatalf("got first detail %#v", details[0])
	}
	if details[1].BaseURL != "https://b.example" || details[1].Feeds != 1 {
		t.Fatalf("got second detail %#v", details[1])
	}
	if len(details[1].Details) != 1 {
		t.Fatalf("got %d feed details", len(details[1].Details))
	}
	if details[1].Details[0].Title != "A" {
		t.Fatalf("got title %q", details[1].Details[0].Title)
	}
	if details[1].Details[0].Link != "https://b.example/bilibili/user/video/a" {
		t.Fatalf("got link %q", details[1].Details[0].Link)
	}
}

func TestRSSHubRefreshDetailsIgnoreNormalFeeds(t *testing.T) {
	db := testStorage(t)
	worker := NewWorker(db)
	feed := db.CreateFeed("A", "", "", "https://example.com/feed.xml", nil)

	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": "https://a.example"}) {
		t.Fatal("failed to set RSSHub base URL")
	}

	worker.recordRSSHubRefreshHit(&FeedRefreshResult{FeedID: feed.Id, StoredFeedLink: feed.FeedLink, RSSHubBase: "https://a.example", RSSHubLink: "https://a.example/feed.xml"})

	details := worker.RSSHubRefreshDetails()
	if len(details) != 1 {
		t.Fatalf("got %d details", len(details))
	}
	if details[0].Feeds != 0 {
		t.Fatalf("got %d feeds", details[0].Feeds)
	}
}

func TestRSSHubRefreshDetailsIgnoreDeletedFeeds(t *testing.T) {
	db := testStorage(t)
	worker := NewWorker(db)
	feed := db.CreateFeed("A", "", "", "rsshub://bilibili/user/video/a", nil)

	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": "https://a.example"}) {
		t.Fatal("failed to set RSSHub base URL")
	}

	worker.recordRSSHubRefreshHit(&FeedRefreshResult{FeedID: feed.Id, StoredFeedLink: feed.FeedLink, RSSHubBase: "https://a.example", RSSHubLink: "https://a.example/bilibili/user/video/a"})
	db.DeleteFeed(feed.Id)

	details := worker.RSSHubRefreshDetails()
	if len(details) != 1 {
		t.Fatalf("got %d details", len(details))
	}
	if details[0].Feeds != 0 {
		t.Fatalf("got %d feeds", details[0].Feeds)
	}
}

func TestRSSHubRefreshDetailsResetWhenBaseListChanges(t *testing.T) {
	db := testStorage(t)
	worker := NewWorker(db)
	feed := db.CreateFeed("A", "", "", "rsshub://bilibili/user/video/a", nil)

	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": "https://a.example"}) {
		t.Fatal("failed to set RSSHub base URL")
	}

	worker.recordRSSHubRefreshHit(&FeedRefreshResult{FeedID: feed.Id, StoredFeedLink: feed.FeedLink, RSSHubBase: "https://a.example", RSSHubLink: "https://a.example/bilibili/user/video/a"})
	worker.CheckRSSHubAvailability()

	details := worker.RSSHubRefreshDetails()
	if len(details) != 1 {
		t.Fatalf("got %d details", len(details))
	}
	if details[0].Feeds != 0 {
		t.Fatalf("got %d feeds", details[0].Feeds)
	}
}

func TestRSSHubNotModifiedRefreshRecordsSuccessfulBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	db := testStorage(t)
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": server.URL}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	feed := db.CreateFeed("RSSHub Feed", "", "", "rsshub://bilibili/weekly", nil)
	db.SetHTTPState(feed.Id, "", "test-etag")

	worker := NewWorker(db)
	requestLinks, err := worker.resolveLinks(feed.FeedLink)
	if err != nil {
		t.Fatal(err)
	}
	result, err := refreshFeedFromLinks(*feed, requestLinks, db)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Feed != nil {
		t.Fatal("expected no parsed feed")
	}
	if result.RSSHubBase != server.URL {
		t.Fatalf("got base %q", result.RSSHubBase)
	}
	if result.RSSHubLink != server.URL+"/bilibili/weekly" {
		t.Fatalf("got link %q", result.RSSHubLink)
	}
}

func TestRSSHubRefreshResultRecordsSuccessfulBaseAfterFallback(t *testing.T) {
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
	worker := NewWorker(db)

	requestLinks, err := worker.resolveLinks(feed.FeedLink)
	if err != nil {
		t.Fatal(err)
	}
	result, err := refreshFeedFromLinks(*feed, requestLinks, db)
	if err != nil {
		t.Fatal(err)
	}
	if result.RSSHubBase != server.URL+"/good" {
		t.Fatalf("got base %q", result.RSSHubBase)
	}
	if result.RSSHubLink != server.URL+"/good/bilibili/weekly" {
		t.Fatalf("got link %q", result.RSSHubLink)
	}
}

func TestRefresherRecordsRSSHubFeedDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, rssBody("哔哩热榜"))
	}))
	defer server.Close()

	db := testStorage(t)
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": server.URL}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	feed := db.CreateFeed("哔哩热榜", "", "", "rsshub://bilibili/weekly", nil)
	worker := NewWorker(db)

	worker.refresher([]storage.Feed{*feed})

	details := worker.RSSHubRefreshDetails()
	if len(details) != 1 {
		t.Fatalf("got %d details", len(details))
	}
	if details[0].Feeds != 1 {
		t.Fatalf("got %d feeds", details[0].Feeds)
	}
	if len(details[0].Details) != 1 {
		t.Fatalf("got %d feed details", len(details[0].Details))
	}
	if details[0].Details[0].Title != "哔哩热榜" {
		t.Fatalf("got title %q", details[0].Details[0].Title)
	}
	if details[0].Details[0].Link != server.URL+"/bilibili/weekly" {
		t.Fatalf("got link %q", details[0].Details[0].Link)
	}
}
