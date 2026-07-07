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
		{Rank: 1, RankChange: "📈2", Title: "标题一", Author: "作者一", CoverURL: "https://example.com/1.jpg", URL: "https://example.com/1", Content: `<p>正文一</p>`, Date: now.Add(-time.Hour)},
		{Rank: 2, RankChange: "➡️", Title: "标题二", Author: "作者二", URL: "https://example.com/2", Date: now.Add(-2 * time.Hour)},
	}, storage.FeedRankingModeWithImage, now)

	for _, want := range []string{
		`<!-- ranking-entry-url:https://example.com/1 -->`,
		`<a href="https://example.com/1" class="bilibili-ranking-link">`,
		`<article class="bilibili-ranking-card">`,
		`<img src="https://example.com/1.jpg" alt="标题一" class="bilibili-ranking-image">`,
		`<p class="bilibili-ranking-meta"><span>01｜📈2｜作者一</span><time datetime="2026-07-06T02:00:00Z">1h</time></p>`,
		`<details class="bilibili-ranking-details"><summary class="bilibili-ranking-title">标题一</summary>`,
		`<div class="bilibili-ranking-content"><p>正文一</p></div>`,
		`<p class="bilibili-ranking-meta"><span>02｜➡️｜作者二</span><time datetime="2026-07-06T01:00:00Z">2h</time></p>`,
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
	if item.Item.GUID != "ranking:2026070610" {
		t.Fatalf("invalid guid: %q", item.Item.GUID)
	}
	if item.Item.Title != "🔥️ 2026 年 07 月 06 日 10 时 实时榜单" {
		t.Fatalf("invalid title: %q", item.Item.Title)
	}
	if item.Item.Link != "https://example.com" {
		t.Fatalf("invalid link: %q", item.Item.Link)
	}
	if !strings.Contains(item.Item.Content, "标题一") {
		t.Fatalf("invalid content: %s", item.Item.Content)
	}
	if item.MD5 != "92eefe2b5e3592b5843b4d23c603bc02" {
		t.Fatalf("invalid md5: %q", item.MD5)
	}
	if len(item.Item.MediaLinks) != 1 || item.Item.MediaLinks[0].URL != "https://example.com/1.jpg" {
		t.Fatalf("invalid media links: %#v", item.Item.MediaLinks)
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
	if len(item.Item.MediaLinks) != 0 {
		t.Fatalf("invalid media links: %#v", item.Item.MediaLinks)
	}
	if strings.Contains(item.Item.Content, `bilibili-ranking-image`) {
		t.Fatalf("content should not include image: %s", item.Item.Content)
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
	if !db.CreateRankingModeItem(item.Item, item.MD5) {
		t.Fatal("failed to create item")
	}
	w.cacheRankingModeGUID(feed.Id, item.Item.GUID, now)
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
	if result.RankingItem != nil {
		t.Fatal("expected disabled ranking mode to skip")
	}

	feed = db.CreateFeedWithRankingMode("feed", "", "", "https://example.com/feed.xml", "", "", storage.FeedRankingModeWithImage, nil)
	result = &FeedRefreshResult{Feed: &parser.Feed{}}
	w.appendRankingModeItem(*feed, result, time.Now())
	if result.RankingItem != nil {
		t.Fatal("expected empty feed to skip")
	}
}

func TestRankingModeItemSkipsUnchangedRankingAcrossHours(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	feed := db.CreateFeedWithRankingMode("feed", "", "", "https://example.com/feed.xml", "", "", storage.FeedRankingModeWithImage, nil)
	w := NewWorker(db)
	now := time.Date(2026, 7, 6, 2, 30, 0, 0, time.UTC)

	item, ok := w.rankingModeItem(*feed, testRankingFeed(), now)
	if !ok {
		t.Fatal("expected first ranking item")
	}
	if !db.CreateRankingModeItem(item.Item, item.MD5) {
		t.Fatal("failed to create ranking item")
	}
	feed = db.GetFeed(feed.Id)
	if _, ok := w.rankingModeItem(*feed, testRankingFeed(), now.Add(time.Hour)); ok {
		t.Fatal("expected unchanged ranking to be skipped")
	}
}

func TestRankingModeItemCreatesChangedRankingAcrossHours(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	feed := db.CreateFeedWithRankingMode("feed", "", "", "https://example.com/feed.xml", "", "", storage.FeedRankingModeWithImage, nil)
	w := NewWorker(db)
	now := time.Date(2026, 7, 6, 2, 30, 0, 0, time.UTC)

	first, ok := w.rankingModeItem(*feed, testRankingFeedWithURLs("https://example.com/1", "https://example.com/2"), now)
	if !ok {
		t.Fatal("expected first ranking item")
	}
	if !db.CreateRankingModeItem(first.Item, first.MD5) {
		t.Fatal("failed to create first ranking item")
	}

	feed = db.GetFeed(feed.Id)
	second, ok := w.rankingModeItem(*feed, testRankingFeedWithURLs("https://example.com/2", "https://example.com/1", "https://example.com/3"), now.Add(time.Hour))
	if !ok {
		t.Fatal("expected changed ranking item")
	}
	for _, want := range []string{`01｜📈1｜作者一`, `02｜📉1｜作者二`, `03｜🆕｜作者三`} {
		if !strings.Contains(second.Item.Content, want) {
			t.Fatalf("content missing %q: %s", want, second.Item.Content)
		}
	}
	if !db.CreateRankingModeItem(second.Item, second.MD5) {
		t.Fatal("failed to create second ranking item")
	}
	feed = db.GetFeed(feed.Id)
	if feed.LastRankingItem != second.Item.GUID {
		t.Fatalf("invalid last ranking item: %q", feed.LastRankingItem)
	}
	if feed.LastRankingMD5 != second.MD5 {
		t.Fatalf("invalid last ranking md5: %q", feed.LastRankingMD5)
	}
}

func TestRankingPositionsFromContent(t *testing.T) {
	content := RenderRankingContent([]RankingEntry{
		{Rank: 1, RankChange: "🆕", Title: "一", URL: "https://example.com/1?a=1&b=2"},
		{Rank: 2, RankChange: "🆕", Title: "二", URL: "https://example.com/2"},
	}, storage.FeedRankingModeWithoutImage, time.Now())
	positions := RankingPositionsFromContent(content)
	if positions["https://example.com/1?a=1&b=2"] != 1 || positions["https://example.com/2"] != 2 {
		t.Fatalf("invalid positions: %#v", positions)
	}
}

func testRankingFeed() *parser.Feed {
	return &parser.Feed{Items: []parser.Item{
		{Title: "标题一", URL: "https://example.com/1", Author: "作者一", MediaLinks: []parser.MediaLink{{Type: "image", URL: "https://example.com/1.jpg"}}},
		{Title: "标题二", URL: "https://example.com/2", Author: "作者二"},
	}}
}

func testRankingFeedWithURLs(urls ...string) *parser.Feed {
	names := []string{"一", "二", "三", "四"}
	items := make([]parser.Item, len(urls))
	for i, url := range urls {
		items[i] = parser.Item{
			Title:  "标题" + names[i],
			URL:    url,
			Author: "作者" + names[i],
		}
	}
	return &parser.Feed{Items: items}
}
