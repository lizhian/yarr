package worker

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html"
	"log"
	"regexp"
	"time"

	"github.com/nkanaev/yarr/src/parser"
	"github.com/nkanaev/yarr/src/storage"
)

const rankingModeCacheTTL = 2 * time.Hour

type RankingEntry struct {
	Rank       int
	RankChange string
	Title      string
	Author     string
	CoverURL   string
	URL        string
	Content    string
	Date       time.Time
}

type RankingModeItem struct {
	Item storage.Item
	MD5  string
}

var rankingModeTimeZone = time.FixedZone("UTC+8", 8*60*60)
var rankingEntryURLCommentRe = regexp.MustCompile(`<!-- ranking-entry-url:([^>]*) -->`)

func (w *Worker) appendRankingModeItem(feed storage.Feed, result *FeedRefreshResult, now time.Time) {
	if feed.RankingMode == storage.FeedRankingModeOff || result == nil || result.Feed == nil || len(result.Feed.Items) == 0 {
		return
	}

	item, ok := w.rankingModeItem(feed, result.Feed, now)
	if !ok {
		return
	}
	result.RankingItem = &item
}

func (w *Worker) rankingModeItem(feed storage.Feed, feedData *parser.Feed, now time.Time) (RankingModeItem, bool) {
	entries := RankingEntries(feedData)
	if len(entries) == 0 {
		log.Printf("ranking mode feed %s has no entries", feed.Title)
		return RankingModeItem{}, false
	}

	localNow := now.In(rankingModeTimeZone).Truncate(time.Hour)
	guid := fmt.Sprintf("ranking:%s", localNow.Format("2006010215"))
	if w.rankingModeGUIDCached(feed.Id, guid, now) {
		return RankingModeItem{}, false
	}
	if w.db.ItemGUIDExists(feed.Id, guid) {
		w.cacheRankingModeGUID(feed.Id, guid, now)
		return RankingModeItem{}, false
	}

	rankingMD5 := RankingMD5(entries)
	if feed.LastRankingMD5 == rankingMD5 {
		return RankingModeItem{}, false
	}
	applyRankingChanges(entries, w.previousRankingPositions(feed))

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
		Title:      fmt.Sprintf("🔥️%04d年%02d月%02d日%02d时", localNow.Year(), localNow.Month(), localNow.Day(), localNow.Hour()),
		Link:       link,
		Content:    RenderRankingContent(entries, feed.RankingMode, now),
		Date:       localNow,
		MediaLinks: RankingMediaLinks(entries, feed.RankingMode),
	}
	return RankingModeItem{Item: item, MD5: rankingMD5}, true
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

func RankingMD5(entries []RankingEntry) string {
	hash := md5.New()
	for _, entry := range entries {
		hash.Write([]byte(entry.URL))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (w *Worker) previousRankingPositions(feed storage.Feed) map[string]int {
	if feed.LastRankingItem == "" {
		return nil
	}
	item := w.db.GetItemByGUID(feed.Id, feed.LastRankingItem)
	if item == nil {
		return nil
	}
	return RankingPositionsFromContent(item.Content)
}

func RankingPositionsFromContent(content string) map[string]int {
	matches := rankingEntryURLCommentRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	positions := make(map[string]int, len(matches))
	for i, match := range matches {
		if len(match) < 2 || match[1] == "" {
			continue
		}
		positions[html.UnescapeString(match[1])] = i + 1
	}
	return positions
}

func applyRankingChanges(entries []RankingEntry, previous map[string]int) {
	for i := range entries {
		previousRank, ok := previous[entries[i].URL]
		switch {
		case !ok:
			entries[i].RankChange = "🌟"
		case previousRank == entries[i].Rank:
			entries[i].RankChange = "➡️"
		case previousRank > entries[i].Rank:
			entries[i].RankChange = fmt.Sprintf("📈%d", previousRank-entries[i].Rank)
		default:
			entries[i].RankChange = fmt.Sprintf("📉%d", entries[i].Rank-previousRank)
		}
	}
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
	newEntries := make([]RankingEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.RankChange == "🌟" {
			newEntries = append(newEntries, entry)
		}
	}
	if len(newEntries) > 0 {
		buffer.WriteString(`<h2>新上榜</h2>`)
		for _, entry := range newEntries {
			renderRankingEntry(&buffer, entry, mode, now, false)
		}
	}
	buffer.WriteString(`<h2>完整榜单</h2>`)
	for _, entry := range entries {
		renderRankingEntry(&buffer, entry, mode, now, true)
	}
	return buffer.String()
}

func renderRankingEntry(buffer *bytes.Buffer, entry RankingEntry, mode string, now time.Time, includePosition bool) {
	title := html.EscapeString(entry.Title)
	entryURL := html.EscapeString(entry.URL)
	coverURL := html.EscapeString(entry.CoverURL)
	author := html.EscapeString(entry.Author)
	dateTime := html.EscapeString(entry.Date.Format(time.RFC3339))
	dateText := html.EscapeString(rankingDateRepr(entry.Date, now))

	if includePosition {
		buffer.WriteString(`<!-- ranking-entry-url:`)
		buffer.WriteString(entryURL)
		buffer.WriteString(` -->`)
	}
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
	buffer.WriteString(entry.RankChange)
	buffer.WriteString(`｜`)
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
	buffer.WriteString(`<div class="bilibili-ranking-content">`)
	if entry.Content != "" {
		buffer.WriteString(entry.Content)
	}
	buffer.WriteString(`<p><a href="`)
	buffer.WriteString(entryURL)
	buffer.WriteString(`" class="bilibili-ranking-open-original">打开原文</a></p>`)
	buffer.WriteString(`</div>`)
	buffer.WriteString(`</details>`)
	buffer.WriteString(`</div>`)
	buffer.WriteString(`</article>`)
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
