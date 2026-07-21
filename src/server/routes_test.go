package server

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/server/opml"
	"github.com/nkanaev/yarr/src/storage"
)

func testServerDB(t *testing.T) *storage.Storage {
	t.Helper()
	log.SetOutput(io.Discard)
	db, err := storage.New(":memory:")
	log.SetOutput(os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}

func TestStatic(t *testing.T) {
	handler := NewServer(nil, "127.0.0.1:8000").handler()
	url := "/static/javascripts/app.js"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", url, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != 200 {
		t.FailNow()
	}
}

func TestStaticWithBase(t *testing.T) {
	server := NewServer(nil, "127.0.0.1:8000")
	server.BasePath = "/sub"

	handler := server.handler()
	url := "/sub/static/javascripts/app.js"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", url, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != 200 {
		t.FailNow()
	}
}

func TestStaticBanTemplates(t *testing.T) {
	handler := NewServer(nil, "127.0.0.1:8000").handler()
	url := "/static/login.html"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", url, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != 404 {
		t.FailNow()
	}
}

func TestStatusIncludesRSSHubDetails(t *testing.T) {
	db := testServerDB(t)
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": "https://a.example"}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	server := NewServer(db, "127.0.0.1:8000")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/status", nil)
	server.handler().ServeHTTP(recorder, request)

	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatal("got", response.StatusCode)
	}

	var body struct {
		RSSHubDetails []struct {
			BaseURL string `json:"base_url"`
			Feeds   int    `json:"feeds"`
			Details []struct {
				Title string `json:"title"`
				Link  string `json:"link"`
			} `json:"details"`
		} `json:"rsshub_details"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.RSSHubDetails) != 1 {
		t.Fatalf("got %d details", len(body.RSSHubDetails))
	}
	if body.RSSHubDetails[0].BaseURL != "https://a.example" {
		t.Fatalf("got base %q", body.RSSHubDetails[0].BaseURL)
	}
}

func TestFeedListSortsByLatestItemArrival(t *testing.T) {
	db := testServerDB(t)
	older := db.CreateFeed("Alpha", "", "", "https://example.com/alpha.xml", nil)
	newer := db.CreateFeed("Zulu", "", "", "https://example.com/zulu.xml", nil)
	empty := db.CreateFeed("Middle", "", "", "https://example.com/middle.xml", nil)

	if !db.CreateItems([]storage.Item{{GUID: "older", FeedId: older.Id, Date: time.Now()}}) {
		t.Fatal("failed to create older item")
	}
	time.Sleep(10 * time.Millisecond)
	if !db.CreateItems([]storage.Item{{GUID: "newer", FeedId: newer.Id, Date: time.Now()}}) {
		t.Fatal("failed to create newer item")
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/feeds", nil)
	NewServer(db, "127.0.0.1:8000").handler().ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}

	var feeds []storage.Feed
	if err := json.NewDecoder(recorder.Result().Body).Decode(&feeds); err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 3 {
		t.Fatalf("got %d feeds", len(feeds))
	}
	if feeds[0].Id != newer.Id || feeds[1].Id != older.Id || feeds[2].Id != empty.Id {
		t.Fatalf("unexpected order: %d, %d, %d", feeds[0].Id, feeds[1].Id, feeds[2].Id)
	}
	if feeds[0].LatestItemArrivedAt == nil || feeds[1].LatestItemArrivedAt == nil {
		t.Fatal("expected arrival timestamps")
	}
	if feeds[2].LatestItemArrivedAt != nil {
		t.Fatalf("empty feed timestamp got %#v", feeds[2].LatestItemArrivedAt)
	}
}

func TestAuthConfigEndpoint(t *testing.T) {
	db := testServerDB(t)
	handler := NewServer(db, "127.0.0.1:8000").handler()

	body := bytes.NewBufferString(`{"enabled":true,"username":"username","password":"password"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", "/api/auth", body)
	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatal("got", response.StatusCode)
	}
	if len(response.Cookies()) == 0 {
		t.Fatal("expected auth cookie")
	}

	config := db.GetAuthConfig()
	if !config.Enabled || config.Username != "username" || config.Password != "password" {
		t.Fatalf("invalid auth config: %#v", config)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest("GET", "/api/status", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != http.StatusUnauthorized {
		t.Fatal("got", recorder.Result().StatusCode)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest("GET", "/api/status", nil)
	request.AddCookie(response.Cookies()[0])
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
}

func TestAuthConfigEndpointRejectsMissingCredentials(t *testing.T) {
	db := testServerDB(t)
	handler := NewServer(db, "127.0.0.1:8000").handler()

	body := bytes.NewBufferString(`{"enabled":true,"username":"username","password":""}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", "/api/auth", body)
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	if db.GetAuthConfig().Enabled {
		t.Fatal("auth should remain disabled")
	}
}

func TestAuthConfigEndpointDisablesAuth(t *testing.T) {
	db := testServerDB(t)
	if !db.SetAuthConfig(true, "username", "password") {
		t.Fatal("did not enable auth")
	}
	handler := NewServer(db, "127.0.0.1:8000").handler()

	login := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/", bytes.NewBufferString("username=username&password=password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(login, request)
	if len(login.Result().Cookies()) == 0 {
		t.Fatal("expected login cookie")
	}

	body := bytes.NewBufferString(`{"enabled":false}`)
	recorder := httptest.NewRecorder()
	request = httptest.NewRequest("PUT", "/api/auth", body)
	request.AddCookie(login.Result().Cookies()[0])
	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatal("got", response.StatusCode)
	}
	if db.GetAuthConfig().Enabled {
		t.Fatal("auth should be disabled")
	}
	if len(response.Cookies()) == 0 || response.Cookies()[0].MaxAge != -1 {
		t.Fatal("expected auth cookie to be cleared")
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest("GET", "/api/status", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
}

func TestFeverUsesStoredAuthConfig(t *testing.T) {
	db := testServerDB(t)
	if !db.SetAuthConfig(true, "username", "password") {
		t.Fatal("did not enable auth")
	}
	handler := NewServer(db, "127.0.0.1:8000").handler()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/fever/", nil)
	handler.ServeHTTP(recorder, request)

	var result map[string]interface{}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["auth"] != float64(0) {
		t.Fatalf("expected failed auth, got %#v", result["auth"])
	}

	apiKey := fmt.Sprintf("%x", md5.Sum([]byte("username:password")))
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest("GET", "/fever/?api_key="+apiKey, nil)
	handler.ServeHTTP(recorder, request)

	if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["auth"] != float64(1) {
		t.Fatalf("expected successful auth, got %#v", result["auth"])
	}
}

func TestIndexGzipped(t *testing.T) {
	db := testServerDB(t)
	handler := NewServer(db, "127.0.0.1:8000").handler()
	url := "/"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", url, nil)
	request.Header.Set("accept-encoding", "gzip")
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	if response.StatusCode != 200 {
		t.FailNow()
	}
	if response.Header.Get("content-encoding") != "gzip" {
		t.Errorf("invalid content-encoding header: %#v", response.Header.Get("content-encoding"))
	}
	if response.Header.Get("content-type") != "text/html" {
		t.Errorf("invalid content-type header: %#v", response.Header.Get("content-type"))
	}
}

func TestMissingItemReturnsNotFound(t *testing.T) {
	db := testServerDB(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/items/1178", nil)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusNotFound {
		t.Fatal("got", recorder.Result().StatusCode)
	}
}

func TestCreateRSSHubFeedWithoutBaseURL(t *testing.T) {
	db := testServerDB(t)

	body := bytes.NewBufferString(`{"url":"rsshub://bilibili/weekly","content_mode":"readability","ranking_mode":"with_image"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/feeds", body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	var result struct {
		Status string       `json:"status"`
		Feed   storage.Feed `json:"feed"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "success" {
		t.Fatalf("got %q", result.Status)
	}
	if result.Feed.FeedLink != "rsshub://bilibili/weekly" {
		t.Fatalf("got %q", result.Feed.FeedLink)
	}

	feed := db.GetFeed(result.Feed.Id)
	if feed == nil {
		t.Fatal("expected feed")
	}
	if feed.FeedLink != "rsshub://bilibili/weekly" {
		t.Fatalf("got %q", feed.FeedLink)
	}
	if feed.ContentMode != storage.FeedContentModeReadability {
		t.Fatalf("got %q", feed.ContentMode)
	}
	if feed.RankingMode != storage.FeedRankingModeWithImage {
		t.Fatalf("got %q", feed.RankingMode)
	}
}

func TestOldBilibiliRSSRoutesRemoved(t *testing.T) {
	db := testServerDB(t)
	handler := NewServer(db, "127.0.0.1:8000").handler()

	for _, path := range []string{"/rss/bilibili/top", "/rss/bilibili/zhishi", "/rss/bilibili/hot"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", path, nil)
		handler.ServeHTTP(recorder, request)
		if recorder.Result().StatusCode == http.StatusOK {
			t.Fatalf("%s returned 200", path)
		}
	}
}

func TestCreateFeedNormalizesSupportedRSSHubInputs(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "Bilibili space", url: "https://space.bilibili.com/703186600", want: "rsshub://bilibili/user/video/703186600"},
		{name: "Bilibili dynamic", url: "https://space.bilibili.com/703186600/dynamic", want: "rsshub://bilibili/user/video/703186600"},
		{name: "Bilibili upload video", url: "https://space.bilibili.com/703186600/upload/video", want: "rsshub://bilibili/user/video/703186600"},
		{name: "Telegram channel", url: "https://t.me/me888888888888", want: "rsshub://telegram/channel/me888888888888"},
		{name: "RSSHub official URL", url: "https://rsshub.app/ithome/ranking/24h", want: "rsshub://ithome/ranking/24h"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log.SetOutput(io.Discard)
			db, _ := storage.New(":memory:")
			log.SetOutput(os.Stderr)

			body := bytes.NewBufferString(fmt.Sprintf(`{"url":%q}`, test.url))
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/api/feeds", body)

			handler := NewServer(db, "127.0.0.1:8000").handler()
			handler.ServeHTTP(recorder, request)

			var result struct {
				Status string       `json:"status"`
				Feed   storage.Feed `json:"feed"`
			}
			if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "success" {
				t.Fatalf("got %q", result.Status)
			}
			if result.Feed.FeedLink != test.want {
				t.Fatalf("got %q, want %q", result.Feed.FeedLink, test.want)
			}
		})
	}
}

func TestCreateFeedRejectsUnsupportedContentMode(t *testing.T) {
	db := testServerDB(t)

	body := bytes.NewBufferString(`{"url":"rsshub://bilibili/weekly","content_mode":"invalid"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/feeds", body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatal("got", recorder.Result().StatusCode)
	}
}

func TestUpdateFeedContentSelector(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"content_selector":".content__default"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.ContentSelector != ".content__default" {
		t.Fatalf("got %q", feed.ContentSelector)
	}
}

func TestUpdateFeedContentSelectorRejectsUnsupportedSelector(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"content_selector":"main .content"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.ContentSelector != "" {
		t.Fatalf("got %q", feed.ContentSelector)
	}
}

func TestUpdateFeedContentMode(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"content_mode":"embed"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.ContentMode != storage.FeedContentModeEmbed {
		t.Fatalf("got %q", feed.ContentMode)
	}
}

func TestUpdateFeedAutoReadScroll(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	if feed.AutoReadScroll {
		t.Fatal("auto_read_scroll should default to false")
	}

	body := bytes.NewBufferString(`{"auto_read_scroll":true}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if !feed.AutoReadScroll {
		t.Fatal("auto_read_scroll should be enabled")
	}
}

func TestUpdateFolderAutoReadScroll(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	folder := db.CreateFolder("news")
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"auto_read_scroll":true}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/folders/%d", folder.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	folders := db.ListFolders()
	if len(folders) != 1 || !folders[0].AutoReadScroll {
		t.Fatalf("got %#v", folders)
	}
}

func TestUpdateFeedContentModeRejectsUnsupportedMode(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"content_mode":"invalid"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.ContentMode != storage.FeedContentModeNormal {
		t.Fatalf("got %q", feed.ContentMode)
	}
}

func TestUpdateFeedRankingMode(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"ranking_mode":"without_image"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.RankingMode != storage.FeedRankingModeWithoutImage {
		t.Fatalf("got %q", feed.RankingMode)
	}
}

func TestUpdateFeedRankingModeRejectsUnsupportedValue(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"ranking_mode":true}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.RankingMode != storage.FeedRankingModeOff {
		t.Fatal("ranking mode should not change")
	}
}

func TestUpdateFeedIconURL(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"icon_url":" https://example.com/icon.png "}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.IconURL != "https://example.com/icon.png" {
		t.Fatalf("got %q", feed.IconURL)
	}
	if !feed.CustomIcon {
		t.Fatal("configured icon should be custom")
	}

	body = bytes.NewBufferString(`{"icon_url":""}`)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.IconURL != "" || feed.CustomIcon {
		t.Fatalf("cleared icon got %#v", feed)
	}
}

func TestUpdateFeedIconURLRejectsUnsupportedURL(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"icon_url":"javascript:alert(1)"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.IconURL != "" {
		t.Fatalf("got %q", feed.IconURL)
	}
}

func TestRefreshFeedIconURLs(t *testing.T) {
	iconServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>feed</title>
    <link>https://example.com</link>
    <image>
      <url>`+serverURL(r)+`/icon.png</url>
    </image>
  </channel>
</rss>`)
		case "/icon.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("icon"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer iconServer.Close()

	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", iconServer.URL, iconServer.URL+"/feed.xml", nil)
	customFeed := db.CreateFeed("custom feed", "", iconServer.URL, iconServer.URL+"/feed.xml?custom=1", nil)
	db.UpdateFeedIconURL(feed.Id, "https://example.com/old-icon.png")
	db.UpdateFeedCustomIconURL(customFeed.Id, "https://example.com/custom-icon.png")
	log.SetOutput(os.Stderr)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/feeds/icons/refresh", nil)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.IconURL != iconServer.URL+"/icon.png" {
		t.Fatalf("got %q", feed.IconURL)
	}
	customFeed = db.GetFeed(customFeed.Id)
	if customFeed.IconURL != "https://example.com/custom-icon.png" || !customFeed.CustomIcon {
		t.Fatalf("custom icon changed: %#v", customFeed)
	}
}

func TestRefreshFeedIconURL(t *testing.T) {
	iconServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>feed</title>
    <link>https://example.com</link>
    <image>
      <url>`+serverURL(r)+`/icon.png</url>
    </image>
  </channel>
</rss>`)
		case "/icon.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("icon"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer iconServer.Close()

	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", iconServer.URL, iconServer.URL+"/feed.xml", nil)
	db.UpdateFeedCustomIconURL(feed.Id, "https://example.com/old-icon.png")
	log.SetOutput(os.Stderr)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", fmt.Sprintf("/api/feeds/%d/icon/refresh", feed.Id), nil)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	var result storage.Feed
	if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.IconURL != iconServer.URL+"/icon.png" {
		t.Fatalf("response got %q", result.IconURL)
	}
	if result.CustomIcon {
		t.Fatal("response should clear custom_icon")
	}
	feed = db.GetFeed(feed.Id)
	if feed.IconURL != iconServer.URL+"/icon.png" {
		t.Fatalf("got %q", feed.IconURL)
	}
	if feed.CustomIcon {
		t.Fatal("manual icon refresh should clear custom_icon")
	}
}

func TestRefreshFeed(t *testing.T) {
	var targetRequests, otherRequests int32
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/target.xml":
			atomic.AddInt32(&targetRequests, 1)
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>target</title>
    <link>https://example.com/target</link>
    <item>
      <guid>target-1</guid>
      <title>target item</title>
      <link>https://example.com/target/1</link>
    </item>
  </channel>
</rss>`)
		case "/other.xml":
			atomic.AddInt32(&otherRequests, 1)
			w.Header().Set("Content-Type", "application/rss+xml")
			io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>other</title>
    <link>https://example.com/other</link>
  </channel>
</rss>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer feedServer.Close()

	dbPath := filepath.Join(t.TempDir(), "storage.db")
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	target := db.CreateFeed("target", "", feedServer.URL+"/target", feedServer.URL+"/target.xml", nil)
	other := db.CreateFeed("other", "", feedServer.URL+"/other", feedServer.URL+"/other.xml", nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", fmt.Sprintf("/api/feeds/%d/refresh", target.Id), nil)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(db.ListItems(storage.ItemFilter{FeedID: &target.Id}, 10, false, false)) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&targetRequests) != 1 {
		t.Fatalf("got %d target requests", atomic.LoadInt32(&targetRequests))
	}
	if atomic.LoadInt32(&otherRequests) != 0 {
		t.Fatalf("got %d other requests", atomic.LoadInt32(&otherRequests))
	}
	if count := len(db.ListItems(storage.ItemFilter{FeedID: &target.Id}, 10, false, false)); count != 1 {
		t.Fatalf("got %d target items", count)
	}
	if count := len(db.ListItems(storage.ItemFilter{FeedID: &other.Id}, 10, false, false)); count != 0 {
		t.Fatalf("got %d other items", count)
	}
}

func TestCreateFeedFromOPMLPreservesFeedIconURLAndContentSelector(t *testing.T) {
	db := testServerDB(t)
	existing := db.CreateFeedWithContentSelector("existing", "", "https://example.com/old", "https://example.com/existing.xml", ".old", nil)
	db.UpdateFeedIconURL(existing.Id, "https://example.com/old-icon.png")

	server := NewServer(db, "127.0.0.1:8000")
	server.createFeedFromOPML(opml.Feed{
		Title:           "new",
		FeedUrl:         "https://example.com/new.xml",
		SiteUrl:         "https://example.com/new",
		ContentSelector: ".article",
		IconURL:         "https://example.com/new-icon.png",
	}, nil)
	server.createFeedFromOPML(opml.Feed{
		Title:           "existing",
		FeedUrl:         "https://example.com/existing.xml",
		SiteUrl:         "https://example.com/existing",
		ContentSelector: ".entry",
		IconURL:         "https://example.com/icon.png",
	}, nil)

	feeds := db.ListFeeds()
	if len(feeds) != 2 {
		t.Fatalf("got %d feeds", len(feeds))
	}
	for _, feed := range feeds {
		switch feed.FeedLink {
		case "https://example.com/new.xml":
			if feed.ContentSelector != ".article" {
				t.Fatalf("got new content selector %q", feed.ContentSelector)
			}
			if feed.IconURL != "https://example.com/new-icon.png" {
				t.Fatalf("got new icon url %q", feed.IconURL)
			}
		case "https://example.com/existing.xml":
			if feed.Id != existing.Id {
				t.Fatalf("got existing feed id %d", feed.Id)
			}
			if feed.ContentSelector != ".entry" {
				t.Fatalf("got existing content selector %q", feed.ContentSelector)
			}
			if feed.IconURL != "https://example.com/icon.png" {
				t.Fatalf("got existing icon url %q", feed.IconURL)
			}
		default:
			t.Fatalf("unexpected feed link %q", feed.FeedLink)
		}
	}
}

func TestCreateFeedFromOPMLIgnoresInvalidFeedIconURLAndContentSelector(t *testing.T) {
	db := testServerDB(t)

	server := NewServer(db, "127.0.0.1:8000")
	server.createFeedFromOPML(opml.Feed{
		Title:           "feed",
		FeedUrl:         "https://example.com/feed.xml",
		SiteUrl:         "https://example.com",
		ContentSelector: "main:unsupported",
		IconURL:         "javascript:alert(1)",
	}, nil)

	feeds := db.ListFeeds()
	if len(feeds) != 1 {
		t.Fatalf("got %d feeds", len(feeds))
	}
	if feeds[0].ContentSelector != "" {
		t.Fatalf("got content selector %q", feeds[0].ContentSelector)
	}
	if feeds[0].IconURL != "" {
		t.Fatalf("got icon url %q", feeds[0].IconURL)
	}
}

func TestFeverFaviconsUsesIconURLAndCaches(t *testing.T) {
	requests := 0
	iconServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("icon"))
	}))
	defer iconServer.Close()

	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	db.UpdateFeedIconURL(feed.Id, iconServer.URL+"/icon.png")
	log.SetOutput(os.Stderr)

	server := NewServer(db, "127.0.0.1:8000")
	handler := server.handler()
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/fever/?favicons", nil)
		handler.ServeHTTP(recorder, request)

		var result struct {
			Favicons []FeverFavicon `json:"favicons"`
		}
		if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if len(result.Favicons) != 1 {
			t.Fatalf("got %#v", result.Favicons)
		}
		if result.Favicons[0].Data != "data:image/png;base64,aWNvbg==" {
			t.Fatalf("got %q", result.Favicons[0].Data)
		}
	}
	if requests != 1 {
		t.Fatalf("got %d requests", requests)
	}
}

func TestFeverFaviconsFallsBackForEmptyIconURL(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/fever/?favicons", nil)
	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	var result struct {
		Favicons []FeverFavicon `json:"favicons"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Favicons) != 1 {
		t.Fatalf("got %#v", result.Favicons)
	}
	if result.Favicons[0].Data != feverBlankFavicon {
		t.Fatalf("got %q", result.Favicons[0].Data)
	}
}
