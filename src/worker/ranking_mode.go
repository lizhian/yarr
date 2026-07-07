package worker

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"time"

	"github.com/nkanaev/yarr/src/parser"
	"github.com/nkanaev/yarr/src/storage"
)

const rankingModeCacheTTL = 2 * time.Hour

type RankingEntry struct {
	Rank     int
	Title    string
	Author   string
	CoverURL string
	URL      string
	Content  string
	Date     time.Time
}

var rankingModeTimeZone = time.FixedZone("UTC+8", 8*60*60)

func (w *Worker) appendRankingModeItem(feed storage.Feed, result *FeedRefreshResult, now time.Time) {
	if feed.RankingMode == storage.FeedRankingModeOff || result == nil || result.Feed == nil || len(result.Feed.Items) == 0 {
		return
	}

	item, ok := w.rankingModeItem(feed, result.Feed, now)
	if !ok {
		return
	}
	result.Items = append(result.Items, item)
}

func (w *Worker) rankingModeItem(feed storage.Feed, feedData *parser.Feed, now time.Time) (storage.Item, bool) {
	entries := RankingEntries(feedData)
	if len(entries) == 0 {
		log.Printf("ranking mode feed %s has no entries", feed.Title)
		return storage.Item{}, false
	}

	localNow := now.In(rankingModeTimeZone).Truncate(time.Hour)
	guid := fmt.Sprintf("ranking:%s", localNow.Format("2006010215"))
	if w.rankingModeGUIDCached(feed.Id, guid, now) {
		return storage.Item{}, false
	}
	if w.db.ItemGUIDExists(feed.Id, guid) {
		w.cacheRankingModeGUID(feed.Id, guid, now)
		return storage.Item{}, false
	}

	link := feed.Link
	if link == "" {
		link = feedData.SiteURL
	}
	if link == "" {
		link = feed.FeedLink
	}
	item := storage.Item{
		GUID:       guid,
		FeedId:     feed.Id,
		Title:      fmt.Sprintf("🔥️ %04d 年 %02d 月 %02d 日 %02d 时 实时榜单", localNow.Year(), localNow.Month(), localNow.Day(), localNow.Hour()),
		Link:       link,
		Content:    RenderRankingContent(entries, feed.RankingMode, now),
		Date:       localNow,
		MediaLinks: RankingMediaLinks(entries, feed.RankingMode),
	}
	return item, true
}

func (w *Worker) cacheInsertedRankingModeItems(feedID int64, items []storage.Item, now time.Time) {
	for _, item := range items {
		if item.FeedId == feedID && isRankingModeGUID(item.GUID) {
			w.cacheRankingModeGUID(feedID, item.GUID, now)
		}
	}
}

func (w *Worker) rankingModeGUIDCached(feedID int64, guid string, now time.Time) bool {
	w.rankingModeMu.Lock()
	defer w.rankingModeMu.Unlock()

	w.cleanupRankingModeCacheLocked(now)
	expiresAt, ok := w.rankingModeExists[rankingModeCacheKey(feedID, guid)]
	return ok && expiresAt.After(now)
}

func (w *Worker) cacheRankingModeGUID(feedID int64, guid string, now time.Time) {
	w.rankingModeMu.Lock()
	defer w.rankingModeMu.Unlock()

	w.cleanupRankingModeCacheLocked(now)
	w.rankingModeExists[rankingModeCacheKey(feedID, guid)] = now.Add(rankingModeCacheTTL)
}

func (w *Worker) cleanupRankingModeCacheLocked(now time.Time) {
	for key, expiresAt := range w.rankingModeExists {
		if !expiresAt.After(now) {
			delete(w.rankingModeExists, key)
		}
	}
}

func rankingModeCacheKey(feedID int64, guid string) string {
	return fmt.Sprintf("%d:%s", feedID, guid)
}

func isRankingModeGUID(guid string) bool {
	return len(guid) == len("ranking:2006010215") && guid[:len("ranking:")] == "ranking:"
}

func RankingEntries(feed *parser.Feed) []RankingEntry {
	if feed == nil {
		return nil
	}
	entries := make([]RankingEntry, 0, len(feed.Items))
	for i, item := range feed.Items {
		if item.Title == "" || item.URL == "" {
			continue
		}
		entries = append(entries, RankingEntry{
			Rank:     i + 1,
			Title:    item.Title,
			Author:   item.Author,
			CoverURL: firstImageMediaLink(item.MediaLinks),
			URL:      item.URL,
			Content:  item.Content,
			Date:     item.Date,
		})
	}
	return entries
}

func firstImageMediaLink(links []parser.MediaLink) string {
	for _, link := range links {
		if link.Type == "image" && link.URL != "" {
			return link.URL
		}
	}
	return ""
}

func RankingMediaLinks(entries []RankingEntry, mode string) storage.MediaLinks {
	if mode == storage.FeedRankingModeWithoutImage {
		return nil
	}
	for _, entry := range entries {
		if entry.CoverURL != "" {
			return storage.MediaLinks{{Type: "image", URL: entry.CoverURL}}
		}
	}
	return nil
}

func RenderRankingContent(entries []RankingEntry, mode string, now time.Time) string {
	var buffer bytes.Buffer
	for _, entry := range entries {
		title := html.EscapeString(entry.Title)
		entryURL := html.EscapeString(entry.URL)
		coverURL := html.EscapeString(entry.CoverURL)
		author := html.EscapeString(entry.Author)
		dateTime := html.EscapeString(entry.Date.Format(time.RFC3339))
		dateText := html.EscapeString(rankingDateRepr(entry.Date, now))

		buffer.WriteString(`<article class="bilibili-ranking-card">`)
		if mode == storage.FeedRankingModeWithImage && coverURL != "" {
			buffer.WriteString(`<a href="`)
			buffer.WriteString(entryURL)
			buffer.WriteString(`" class="bilibili-ranking-link">`)
			buffer.WriteString(`<img src="`)
			buffer.WriteString(coverURL)
			buffer.WriteString(`" alt="`)
			buffer.WriteString(title)
			buffer.WriteString(`" class="bilibili-ranking-image">`)
			buffer.WriteString(`</a>`)
		}
		buffer.WriteString(`<div class="bilibili-ranking-body">`)
		buffer.WriteString(`<p class="bilibili-ranking-meta">`)
		buffer.WriteString(`<span>`)
		buffer.WriteString(fmt.Sprintf(`%02d｜`, entry.Rank))
		if author != "" {
			buffer.WriteString(author)
		}
		buffer.WriteString(`</span>`)
		if !entry.Date.IsZero() {
			buffer.WriteString(`<time datetime="`)
			buffer.WriteString(dateTime)
			buffer.WriteString(`">`)
			buffer.WriteString(dateText)
			buffer.WriteString(`</time>`)
		}
		buffer.WriteString(`</p><details class="bilibili-ranking-details">`)
		buffer.WriteString(`<summary class="bilibili-ranking-title">`)
		buffer.WriteString(title)
		buffer.WriteString(`</summary>`)
		if entry.Content != "" {
			buffer.WriteString(`<br/><div class="bilibili-ranking-content">`)
			buffer.WriteString(entry.Content)
			buffer.WriteString(`</div>`)
		}
		buffer.WriteString(`</details>`)
		buffer.WriteString(`</div>`)
		buffer.WriteString(`</article>`)
	}
	return buffer.String()
}

func rankingDateRepr(date, now time.Time) string {
	if date.IsZero() {
		return ""
	}
	sec := now.Sub(date).Seconds()
	neg := sec < 0
	if neg {
		sec = -sec
	}

	var out string
	switch {
	case sec < 2700:
		out = fmt.Sprintf("%dm", int(sec/60+0.5))
	case sec < 86400:
		out = fmt.Sprintf("%dh", int(sec/3600+0.5))
	case sec < 604800:
		out = fmt.Sprintf("%dd", int(sec/86400+0.5))
	default:
		out = date.Format("2006年1月2日")
	}
	if neg {
		return "-" + out
	}
	return out
}
