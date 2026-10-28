package server

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/parser"
	"github.com/nkanaev/yarr/src/storage"
)

func TestFromRSSLocalArticles(t *testing.T) {
	db := testServerDB(t)
	feed := db.CreateFeed("订阅 & 标题", "本地描述", "https://example.com", "https://example.com/feed", nil)
	other := db.CreateFeed("Other", "", "", "https://example.org/feed", nil)
	date := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	items := []storage.Item{{GUID: "other", FeedId: other.Id, Title: "Other", Date: date.Add(time.Hour)}}
	for i := 0; i < 101; i++ {
		items = append(items, storage.Item{
			GUID: fmt.Sprintf("item-%03d", i), FeedId: feed.Id,
			Title: fmt.Sprintf("文章 %d", i), Link: fmt.Sprintf("https://example.com/%d", i),
			Date: date.Add(time.Duration(i) * time.Minute), Author: "作者 & name",
			Content: `<p>正文 &amp; <a href="https://example.com/?a=1&amp;b=2">原文</a></p>`,
			MediaLinks: storage.MediaLinks{
				{URL: "https://example.com/image.jpg", Type: "image", Description: "封面"},
				{URL: "https://example.com/audio.mp3", Type: "audio"},
			},
		})
	}
	if !db.CreateItems(items) {
		t.Fatal("create items failed")
	}
	read := db.GetItemByGUID(feed.Id, "item-100")
	db.UpdateItemStatus(read.Id, storage.READ)
	db.UpdateItemFavorite(read.Id, true)
	w := httptest.NewRecorder()
	NewServer(db, "").handler().ServeHTTP(w, httptest.NewRequest("GET", "/rss/from/"+feed.FeedLink, nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/rss+xml; charset=utf-8" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Header(), w.Body.String())
	}
	parsed, err := parser.Parse(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Title != feed.Title || parsed.SiteURL != feed.Link || len(parsed.Items) != 100 {
		t.Fatalf("unexpected RSS channel: %+v", parsed)
	}
	for i, item := range parsed.Items {
		expected := fmt.Sprintf("item-%03d", 100-i)
		if item.GUID != expected {
			t.Fatalf("item %d: got %q, want %q", i, item.GUID, expected)
		}
	}
	item := parsed.Items[0]
	if item.Title != "文章 100" || item.URL != read.Link || item.Author != read.Author || !item.Date.Equal(read.Date) || item.Content != read.Content {
		t.Fatalf("article content changed: %+v", item)
	}
	if len(item.MediaLinks) != 2 || item.MediaLinks[0].URL != read.MediaLinks[0].URL || item.MediaLinks[0].Type != "image" || item.MediaLinks[0].Description != "封面" || item.MediaLinks[1].Type != "audio" {
		t.Fatalf("lost media: %+v", item.MediaLinks)
	}
	if got := db.GetItem(read.Id); got.Status != storage.READ || !got.Favorite {
		t.Fatal("export changed reading state")
	}
}

func TestFromRSSLinksAndPublicAccess(t *testing.T) {
	for _, base := range []string{"", "/reader"} {
		t.Run(base, func(t *testing.T) {
			db := testServerDB(t)
			db.SetAuthConfig(true, "user", "password")
			links := []string{
				"https://linux.do/tag/2234-tag/2234.rss",
				"http://example.com/feed",
				"rsshub://telegram/channel/example?limit=100",
				"https://example.com/a%2Fb/feed?z=%2F&a=1&a=2",
				"https://example.com/feed?",
			}
			s := NewServer(db, "")
			s.BasePath = base
			handler := s.handler()
			for _, link := range links {
				feed := db.CreateFeed(link, "", "https://example.com", link, nil)
				for _, path := range []string{link, url.PathEscape(link)} {
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, httptest.NewRequest("GET", base+"/rss/from/"+path, nil))
					parsed, err := parser.Parse(bytes.NewReader(w.Body.Bytes()))
					if w.Code != 200 || err != nil {
						t.Fatalf("%s: status %d, parse error %v", path, w.Code, err)
					}
					if parsed.Title != feed.Title || len(parsed.Items) != 0 {
						t.Fatal("expected matching empty RSS", parsed)
					}
				}
			}
			for _, tc := range []struct {
				method, path string
				status       int
			}{
				{"POST", "/rss/from/" + links[0], 405},
				{"GET", "/rss/from/ftp://example.com/feed", 400},
				{"GET", "/rss/from/https:///feed", 400},
				{"GET", "/rss/from/https://example.com/missing", 404},
				{"GET", "/rss/from/https://example.com/a/b/feed?z=%2F&a=1&a=2", 404},
				{"GET", "/rss/from/https://example.com/a%2Fb/feed?a=1&a=2&z=%2F", 404},
				{"GET", "/api/settings", 401},
			} {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, httptest.NewRequest(tc.method, base+tc.path, nil))
				if w.Code != tc.status {
					t.Fatalf("%s %s: got %d, want %d", tc.method, tc.path, w.Code, tc.status)
				}
				if tc.status == 405 && w.Header().Get("Allow") != "GET" {
					t.Fatal("missing Allow: GET")
				}
			}
		})
	}
}

func TestFromRSSCache(t *testing.T) {
	db := testServerDB(t)
	s := NewServer(db, "")
	handler := s.handler()
	link := "https://example.com/feed"
	request := func() *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/rss/from/"+link, nil))
		return w
	}
	if w := request(); w.Code != 404 {
		t.Fatal(w.Code)
	}
	feed := db.CreateFeed("Feed", "", "https://example.com", link, nil)
	first := request()
	if first.Code != 200 {
		t.Fatal("404 was cached", first.Code)
	}
	key := "rss/from:" + link
	cached := s.cache[key].(fromRSSCache)
	if remaining := time.Until(cached.expiresAt); remaining < 4*time.Minute || remaining > 5*time.Minute {
		t.Fatal("expected five-minute cache", remaining)
	}
	if !db.CreateItems([]storage.Item{{GUID: "new", Title: "New", FeedId: feed.Id, Date: time.Now()}}) {
		t.Fatal("create item failed")
	}
	if w := request(); w.Code != 200 || !bytes.Equal(w.Body.Bytes(), first.Body.Bytes()) {
		t.Fatal("cache did not preserve response")
	}
	expire := func() {
		cached := s.cache[key].(fromRSSCache)
		cached.expiresAt = time.Now().Add(-time.Second)
		s.cache[key] = cached
	}
	expire()
	updated := request()
	parsed, err := parser.Parse(bytes.NewReader(updated.Body.Bytes()))
	if updated.Code != 200 || err != nil || len(parsed.Items) != 1 || parsed.Items[0].GUID != "new" {
		t.Fatal("expired cache did not refresh", updated.Code, err)
	}
	db.DeleteFeed(feed.Id)
	if w := request(); w.Code != 200 || !bytes.Equal(w.Body.Bytes(), updated.Body.Bytes()) {
		t.Fatal("deleted subscription cache did not survive until expiry")
	}
	w := httptest.NewRecorder()
	NewServer(db, "").handler().ServeHTTP(w, httptest.NewRequest("GET", "/rss/from/"+link, nil))
	if w.Code != 404 {
		t.Fatal("cache survived server restart")
	}
	expire()
	if w := request(); w.Code != 404 {
		t.Fatal("deleted subscription still exported after expiry", w.Code)
	}
}
