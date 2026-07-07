package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/parser"
	"github.com/nkanaev/yarr/src/storage"
)

func TestRankingEntries(t *testing.T) {
	entries := RankingEntries(&parser.Feed{Items: []parser.Item{
		{Title: "标题一", URL: "https://example.com/1", Author: "作者一", MediaLinks: []parser.MediaLink{{Type: "image", URL: "https://example.com/1.jpg"}}},
		{Title: "", URL: "https://example.com/skip"},
		{Title: "标题三", URL: ""},
		{Title: "标题四", URL: "https://example.com/4"},
	}})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Rank != 1 || entries[0].Title != "标题一" || entries[0].CoverURL != "https://example.com/1.jpg" {
		t.Fatalf("invalid first entry: %#v", entries[0])
	}
	if entries[1].Rank != 4 || entries[1].Title != "标题四" {
		t.Fatalf("invalid second entry: %#v", entries[1])
	}
}

func TestRenderRankingContent(t *testing.T) {
	now := time.Date(2026, 7, 6, 3, 0, 0, 0, time.UTC)
	content := RenderRankingContent([]RankingEntry{
		{Rank: 1, Title: "标题一", Author: "作者一", CoverURL: "https://example.com/1.jpg", URL: "https://example.com/1", Content: `<p>正文一</p>`, Date: now.Add(-time.Hour)},
		{Rank: 2, Title: "标题二", Author: "作者二", URL: "https://example.com/2", Date: now.Add(-2 * time.Hour)},
	}, storage.FeedRankingModeWithImage, now)

	for _, want := range []string{
		`<a href="https://example.com/1" class="bilibili-ranking-link">`,
		`<article class="bilibili-ranking-card">`,
		`<img src="https://example.com/1.jpg" alt="标题一" class="bilibili-ranking-image">`,
		`<p class="bilibili-ranking-meta"><span>01｜作者一</span><time datetime="2026-07-06T02:00:00Z">1h</time></p>`,
		`<details class="bilibili-ranking-details"><summary class="bilibili-ranking-title">标题一</summary>`,
		`<div class="bilibili-ranking-content"><p>正文一</p></div>`,
		`<p class="bilibili-ranking-meta"><span>02｜作者二</span><time datetime="2026-07-06T01:00:00Z">2h</time></p>`,
		`<summary class="bilibili-ranking-title">标题二</summary>`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("content missing %q: %s", want, content)
		}
	}
}

func TestRenderRankingContentWithoutImage(t *testing.T) {
	content := RenderRankingContent([]RankingEntry{
		{Rank: 1, Title: "标题一", CoverURL: "https://example.com/1.jpg", URL: "https://example.com/1"},
	}, storage.FeedRankingModeWithoutImage, time.Now())
	if strings.Contains(content, `bilibili-ranking-image`) || strings.Contains(content, `<img`) {
		t.Fatalf("content should not include image: %s", content)
	}
}

func TestRankingModeItem(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	feed := db.CreateFeedWithRankingMode("feed", "", "https://example.com", "https://example.com/feed.xml", "", "", storage.FeedRankingModeWithImage, nil)
	w := NewWorker(db)
	now := time.Date(2026, 7, 6, 2, 30, 0, 0, time.UTC)

	item, ok := w.rankingModeItem(*feed, testRankingFeed(), now)
	if !ok {
		t.Fatal("expected ranking item")
	}
	if item.GUID != "ranking:2026070610" {
		t.Fatalf("invalid guid: %q", item.GUID)
	}
	if item.Title != "🔥️ 2026 年 07 月 06 日 10 时 实时榜单" {
		t.Fatalf("invalid title: %q", item.Title)
	}
	if item.Link != "https://example.com" {
		t.Fatalf("invalid link: %q", item.Link)
	}
	if !strings.Contains(item.Content, "标题一") {
		t.Fatalf("invalid content: %s", item.Content)
	}
	if len(item.MediaLinks) != 1 || item.MediaLinks[0].URL != "https://example.com/1.jpg" {
		t.Fatalf("invalid media links: %#v", item.MediaLinks)
	}
}

func TestRankingModeItemWithoutImageHasNoMediaLink(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	feed := db.CreateFeedWithRankingMode("feed", "", "https://example.com", "https://example.com/feed.xml", "", "", storage.FeedRankingModeWithoutImage, nil)
	w := NewWorker(db)

	item, ok := w.rankingModeItem(*feed, testRankingFeed(), time.Date(2026, 7, 6, 2, 30, 0, 0, time.UTC))
	if !ok {
		t.Fatal("expected ranking item")
	}
	if len(item.MediaLinks) != 0 {
		t.Fatalf("invalid media links: %#v", item.MediaLinks)
	}
	if strings.Contains(item.Content, `bilibili-ranking-image`) {
		t.Fatalf("content should not include image: %s", item.Content)
	}
}

func TestRankingModeItemSkipsDuplicateWithCache(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	feed := db.CreateFeedWithRankingMode("feed", "", "", "https://example.com/feed.xml", "", "", storage.FeedRankingModeWithImage, nil)
	w := NewWorker(db)
	now := time.Date(2026, 7, 6, 2, 30, 0, 0, time.UTC)

	item, ok := w.rankingModeItem(*feed, testRankingFeed(), now)
	if !ok {
		t.Fatal("expected ranking item")
	}
	if !db.CreateItems([]storage.Item{item}) {
		t.Fatal("failed to create item")
	}
	w.cacheInsertedRankingModeItems(feed.Id, []storage.Item{item}, now)
	if len(w.rankingModeExists) != 1 {
		t.Fatalf("expected cached guid, got %#v", w.rankingModeExists)
	}
	if _, ok := w.rankingModeItem(*feed, testRankingFeed(), now.Add(time.Minute)); ok {
		t.Fatal("expected cached duplicate to be skipped")
	}
}

func TestAppendRankingModeItemSkipsDisabledAndEmpty(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	feed := db.CreateFeed("feed", "", "", "https://example.com/feed.xml", nil)
	w := NewWorker(db)
	result := &FeedRefreshResult{Feed: testRankingFeed()}

	w.appendRankingModeItem(*feed, result, time.Now())
	if len(result.Items) != 0 {
		t.Fatal("expected disabled ranking mode to skip")
	}

	feed = db.CreateFeedWithRankingMode("feed", "", "", "https://example.com/feed.xml", "", "", storage.FeedRankingModeWithImage, nil)
	result = &FeedRefreshResult{Feed: &parser.Feed{}}
	w.appendRankingModeItem(*feed, result, time.Now())
	if len(result.Items) != 0 {
		t.Fatal("expected empty feed to skip")
	}
}

func testRankingFeed() *parser.Feed {
	return &parser.Feed{Items: []parser.Item{
		{Title: "标题一", URL: "https://example.com/1", Author: "作者一", MediaLinks: []parser.MediaLink{{Type: "image", URL: "https://example.com/1.jpg"}}},
		{Title: "标题二", URL: "https://example.com/2", Author: "作者二"},
	}}
}
