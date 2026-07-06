package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/parser"
	"github.com/nkanaev/yarr/src/storage"
)

const bilibiliRankingRSS = `
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
	<title>bilibili 排行榜-全站</title>
	<link>https://www.bilibili.com/v/popular/rank/all</link>
	<item>
		<title>视频标题一</title>
		<description><![CDATA[<img src="https://i1.hdslb.com/cover-from-content.jpg"><br>简介]]></description>
		<link>https://www.bilibili.com/video/BV1/</link>
		<guid isPermaLink="false">https://www.bilibili.com/video/BV1/</guid>
		<pubDate>Sun, 05 Jul 2026 13:24:27 GMT</pubDate>
		<author>作者一</author>
		<enclosure url="http://i1.hdslb.com/cover.jpg" type="image/jpeg"></enclosure>
	</item>
	<item>
		<title>视频标题二</title>
		<description><![CDATA[简介]]></description>
		<link>https://www.bilibili.com/video/BV2/</link>
		<guid isPermaLink="false">https://www.bilibili.com/video/BV2/</guid>
		<pubDate>Sun, 05 Jul 2026 12:29:47 GMT</pubDate>
		<author>作者二</author>
		<enclosure url="http://i2.hdslb.com/cover.jpg" type="image/jpeg"></enclosure>
	</item>
</channel>
</rss>`

func TestBilibiliRankingEntries(t *testing.T) {
	feed, err := parser.ParseAndFix(strings.NewReader(bilibiliRankingRSS), BilibiliTopSourceLink, "")
	if err != nil {
		t.Fatal(err)
	}
	entries := BilibiliRankingEntries(feed)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Rank != 1 || entries[0].Title != "视频标题一" || entries[0].Author != "作者一" {
		t.Fatalf("invalid first entry: %#v", entries[0])
	}
	if entries[0].CoverURL != "http://i1.hdslb.com/cover.jpg" {
		t.Fatalf("invalid cover url: %q", entries[0].CoverURL)
	}
	if entries[0].VideoURL != "https://www.bilibili.com/video/BV1/" {
		t.Fatalf("invalid video url: %q", entries[0].VideoURL)
	}
}

func TestBilibiliRankingEntriesEmpty(t *testing.T) {
	entries := BilibiliRankingEntries(&parser.Feed{})
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
}

func TestSaveBilibiliTop(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	w := NewWorker(db)
	now := time.Date(2026, 7, 6, 2, 30, 0, 0, time.UTC)

	if !w.SaveBilibiliTop(bilibiliRankingRSS, now) {
		t.Fatal("failed to save bilibili top")
	}
	if !w.SaveBilibiliTop(strings.Replace(bilibiliRankingRSS, "视频标题一", "视频标题一更新", 1), now) {
		t.Fatal("failed to update bilibili top")
	}

	items := db.ListGeneratedRSSItems(BilibiliTopSourceKey, 10)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].GUID != "bilibili_top:2026070610" {
		t.Fatalf("invalid guid: %q", items[0].GUID)
	}
	if items[0].Title != "2026 年 07 月 06 日 10 时 B站全站热榜" {
		t.Fatalf("invalid title: %q", items[0].Title)
	}
	if !strings.Contains(items[0].Content, "视频标题一更新") {
		t.Fatalf("content not updated: %s", items[0].Content)
	}
}

func TestSaveBilibiliZhishi(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	w := NewWorker(db)
	now := time.Date(2026, 7, 6, 2, 30, 0, 0, time.UTC)

	if !w.SaveBilibiliTopHubSource(BilibiliZhishiSource, bilibiliRankingRSS, now) {
		t.Fatal("failed to save bilibili zhishi")
	}

	items := db.ListGeneratedRSSItems(BilibiliZhishiSourceKey, 10)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].GUID != "bilibili_zhishi:2026070610" {
		t.Fatalf("invalid guid: %q", items[0].GUID)
	}
	if items[0].Title != "2026 年 07 月 06 日 10 时 B站知识榜" {
		t.Fatalf("invalid title: %q", items[0].Title)
	}
	if items[0].Link != BilibiliZhishiSourceLink {
		t.Fatalf("invalid link: %q", items[0].Link)
	}
}

func TestGeneratedRSSHubRequestsRotateAcrossRefreshes(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(db)
	bases := []string{
		"https://a.example",
		"https://b.example",
		"https://c.example",
		"https://d.example",
		"https://e.example",
		"https://f.example",
		"https://g.example",
		"https://h.example",
		"https://i.example",
		"https://j.example",
	}
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": strings.Join(bases, "\n")}) {
		t.Fatal("failed to set RSSHub base URL")
	}

	first, err := worker.generatedRSSHubRequests(BilibiliTopSource)
	if err != nil {
		t.Fatal(err)
	}
	second, err := worker.generatedRSSHubRequests(BilibiliTopSource)
	if err != nil {
		t.Fatal(err)
	}

	wantFirst := []string{
		"https://a.example/bilibili/ranking/0",
		"https://b.example/bilibili/ranking/0",
		"https://c.example/bilibili/ranking/0",
		"https://d.example/bilibili/ranking/0",
		"https://e.example/bilibili/ranking/0",
	}
	wantSecond := []string{
		"https://f.example/bilibili/ranking/0",
		"https://g.example/bilibili/ranking/0",
		"https://h.example/bilibili/ranking/0",
		"https://i.example/bilibili/ranking/0",
		"https://j.example/bilibili/ranking/0",
	}
	if got := generatedRSSHubRequestLinks(first); !sameStrings(got, wantFirst) {
		t.Fatalf("first got %#v", got)
	}
	if got := generatedRSSHubRequestLinks(second); !sameStrings(got, wantSecond) {
		t.Fatalf("second got %#v", got)
	}
}

func TestGeneratedRSSHubRequestsPreferLastSuccess(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(db)
	bases := []string{
		"https://a.example",
		"https://b.example",
		"https://c.example",
		"https://d.example",
		"https://e.example",
		"https://f.example",
	}
	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": strings.Join(bases, "\n")}) {
		t.Fatal("failed to set RSSHub base URL")
	}
	worker.recordGeneratedRSSHubSuccess(BilibiliTopSourceKey, "https://f.example")

	requests, err := worker.generatedRSSHubRequests(BilibiliTopSource)
	if err != nil {
		t.Fatal(err)
	}
	links := generatedRSSHubRequestLinks(requests)
	if len(links) != RSSHUB_MAX_ATTEMPTS {
		t.Fatalf("got %d links", len(links))
	}
	if links[0] != "https://f.example/bilibili/ranking/0" {
		t.Fatalf("got first link %q", links[0])
	}
}

func generatedRSSHubRequestLinks(requests []generatedRSSHubRequest) []string {
	links := make([]string, len(requests))
	for i, request := range requests {
		links[i] = request.link
	}
	return links
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestRenderBilibiliTopContent(t *testing.T) {
	content := RenderBilibiliTopContent([]BilibiliTopEntry{
		{
			Rank:     1,
			Title:    "视频标题一",
			Author:   "作者一",
			CoverURL: "http://i1.hdslb.com/cover.jpg",
			VideoURL: "https://www.bilibili.com/video/av1/",
		},
		{
			Rank:     2,
			Title:    "视频标题二",
			Author:   "作者二",
			CoverURL: "http://i2.hdslb.com/cover.jpg",
			VideoURL: "https://www.bilibili.com/video/av2/",
		},
	})

	for _, want := range []string{
		`<article><p><a href="https://www.bilibili.com/video/av1/"><img src="http://i1.hdslb.com/cover.jpg"`,
		`<p>排名 1&nbsp;&nbsp; 作者 作者一</p>`,
		`<p><a style="text-decoration: none;" href="https://www.bilibili.com/video/av1/">《视频标题一》</a></p>`,
		`<hr><article>`,
		`<p>排名 2&nbsp;&nbsp; 作者 作者二</p>`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("content missing %q: %s", want, content)
		}
	}
}

func TestSaveBilibiliTopEmpty(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	w := NewWorker(db)
	if w.SaveBilibiliTop(`<?xml version="1.0"?><rss><channel></channel></rss>`, time.Now()) {
		t.Fatal("expected empty save to fail")
	}
}
