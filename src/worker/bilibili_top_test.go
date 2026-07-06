package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/storage"
)

const bilibiliTopHTML = `
<html><body><table><tbody>
<tr>
	<td align="center">1.</td>
	<td class="al" align="center"><img src="http://i1.hdslb.com/cover.jpg" /></td>
	<td class="al">
		<div><a href="https://www.bilibili.com/video/av1/" target="_blank">视频标题一</a></div>
		<div class="item-desc">269.4万</div>
	</td>
	<td align="right"><a href="https://www.bilibili.com/video/av1/">查看详细</a></td>
</tr>
<tr>
	<td align="center">2.</td>
	<td class="al" align="center"><img src="http://i2.hdslb.com/cover.jpg" /></td>
	<td class="al">
		<div><a href="https://www.bilibili.com/video/av2/" target="_blank">视频标题二</a></div>
		<div class="item-desc">772.8万</div>
	</td>
	<td align="right"><a href="https://www.bilibili.com/video/av2/">查看详细</a></td>
</tr>
</tbody></table></body></html>`

func TestParseBilibiliTop(t *testing.T) {
	entries, err := ParseBilibiliTop(bilibiliTopHTML)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Rank != 1 || entries[0].Title != "视频标题一" || entries[0].Views != "269.4万" {
		t.Fatalf("invalid first entry: %#v", entries[0])
	}
	if entries[0].CoverURL != "http://i1.hdslb.com/cover.jpg" {
		t.Fatalf("invalid cover url: %q", entries[0].CoverURL)
	}
	if entries[0].VideoURL != "https://www.bilibili.com/video/av1/" {
		t.Fatalf("invalid video url: %q", entries[0].VideoURL)
	}
}

func TestParseBilibiliTopEmpty(t *testing.T) {
	entries, err := ParseBilibiliTop(`<html><body><p>empty</p></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
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
	now := time.Date(2026, 7, 6, 10, 30, 0, 0, time.Local)

	if !w.SaveBilibiliTop(bilibiliTopHTML, now) {
		t.Fatal("failed to save bilibili top")
	}
	if !w.SaveBilibiliTop(strings.Replace(bilibiliTopHTML, "视频标题一", "视频标题一更新", 1), now) {
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
	now := time.Date(2026, 7, 6, 10, 30, 0, 0, time.Local)

	if !w.SaveBilibiliTopHubSource(BilibiliZhishiSource, bilibiliTopHTML, now) {
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

func TestRenderBilibiliTopContent(t *testing.T) {
	content := RenderBilibiliTopContent([]BilibiliTopEntry{
		{
			Rank:     1,
			Title:    "视频标题一",
			Views:    "269.4万",
			CoverURL: "http://i1.hdslb.com/cover.jpg",
			VideoURL: "https://www.bilibili.com/video/av1/",
		},
		{
			Rank:     2,
			Title:    "视频标题二",
			Views:    "772.8万",
			CoverURL: "http://i2.hdslb.com/cover.jpg",
			VideoURL: "https://www.bilibili.com/video/av2/",
		},
	})

	for _, want := range []string{
		`<article><p><a href="https://www.bilibili.com/video/av1/"><img src="http://i1.hdslb.com/cover.jpg"`,
		`<p>排名 1&nbsp;&nbsp; 播放量 269.4万</p>`,
		`<p><a style="text-decoration: none;" href="https://www.bilibili.com/video/av1/">《视频标题一》</a></p>`,
		`<hr><article>`,
		`<p>排名 2&nbsp;&nbsp; 播放量 772.8万</p>`,
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
	if w.SaveBilibiliTop(`<html></html>`, time.Now()) {
		t.Fatal("expected empty save to fail")
	}
}
