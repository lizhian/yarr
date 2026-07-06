package worker

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"strconv"
	"time"

	"github.com/nkanaev/yarr/src/parser"
	"github.com/nkanaev/yarr/src/rsshub"
	"github.com/nkanaev/yarr/src/storage"
)

const (
	BilibiliTopSourceKey         = "bilibili_top"
	BilibiliTopSourceTitle       = "B站全站热榜"
	BilibiliTopSourceLink        = "rsshub://bilibili/ranking/0"
	BilibiliTopSourceDescription = "B站全站热榜小时快照"

	BilibiliZhishiSourceKey         = "bilibili_zhishi"
	BilibiliZhishiSourceTitle       = "B站知识榜"
	BilibiliZhishiSourceLink        = "rsshub://bilibili/ranking/knowledge"
	BilibiliZhishiSourceDescription = "B站知识榜小时快照"
)

type BilibiliTopEntry struct {
	Rank     int
	Title    string
	Author   string
	CoverURL string
	VideoURL string
}

type BilibiliTopHubSource struct {
	Key         string
	Title       string
	Link        string
	Description string
}

var BilibiliTopSource = BilibiliTopHubSource{
	Key:         BilibiliTopSourceKey,
	Title:       BilibiliTopSourceTitle,
	Link:        BilibiliTopSourceLink,
	Description: BilibiliTopSourceDescription,
}

var BilibiliZhishiSource = BilibiliTopHubSource{
	Key:         BilibiliZhishiSourceKey,
	Title:       BilibiliZhishiSourceTitle,
	Link:        BilibiliZhishiSourceLink,
	Description: BilibiliZhishiSourceDescription,
}

var BilibiliTopHubSources = []BilibiliTopHubSource{
	BilibiliTopSource,
	BilibiliZhishiSource,
}

var bilibiliTopHubTimeZone = time.FixedZone("UTC+8", 8*60*60)

func (w *Worker) RefreshBilibiliTopHubSources() {
	for _, source := range BilibiliTopHubSources {
		w.RefreshBilibiliTopHubSource(source)
	}
}

func (w *Worker) RefreshBilibiliTop() {
	w.RefreshBilibiliTopHubSource(BilibiliTopSource)
}

func (w *Worker) RefreshBilibiliTopHubSource(source BilibiliTopHubSource) {
	EnsureBilibiliTopHubSource(w.db, source)

	feed, err := w.fetchBilibiliTopHubFeed(source)
	if err != nil {
		log.Printf("failed to fetch %s: %s", source.Title, err)
		return
	}
	if !w.SaveBilibiliTopHubFeed(source, feed, time.Now()) {
		log.Printf("failed to save %s", source.Title)
	}
}

func (w *Worker) SaveBilibiliTop(body string, now time.Time) bool {
	return w.SaveBilibiliTopHubSource(BilibiliTopSource, body, now)
}

func (w *Worker) SaveBilibiliTopHubSource(source BilibiliTopHubSource, body string, now time.Time) bool {
	feed, err := parser.ParseAndFix(bytes.NewBufferString(body), source.Link, "")
	if err != nil {
		log.Printf("failed to parse %s: %s", source.Title, err)
		return false
	}
	return w.SaveBilibiliTopHubFeed(source, feed, now)
}

func (w *Worker) SaveBilibiliTopHubFeed(source BilibiliTopHubSource, feed *parser.Feed, now time.Time) bool {
	entries := BilibiliRankingEntries(feed)
	if len(entries) == 0 {
		log.Printf("%s has no entries", source.Title)
		return false
	}

	localNow := now.In(bilibiliTopHubTimeZone).Truncate(time.Hour)
	title := fmt.Sprintf("%04d 年 %02d 月 %02d 日 %02d 时 %s",
		localNow.Year(), localNow.Month(), localNow.Day(), localNow.Hour(), source.Title,
	)
	guid := fmt.Sprintf("%s:%s", source.Key, localNow.Format("2006010215"))
	content := RenderBilibiliTopContent(entries)
	EnsureBilibiliTopHubSource(w.db, source)
	return w.db.UpsertGeneratedRSSItem(
		source.Key,
		guid,
		title,
		source.Link,
		content,
		localNow,
	)
}

func (w *Worker) fetchBilibiliTopHubFeed(source BilibiliTopHubSource) (*parser.Feed, error) {
	requests, err := w.generatedRSSHubRequests(source)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, request := range requests {
		res, err := client.get(request.link)
		if err != nil {
			logCandidateFailure(source.Link, request.link, err)
			lastErr = err
			continue
		}
		if res.StatusCode < 200 || res.StatusCode > 399 {
			err := fmt.Errorf("status code %d", res.StatusCode)
			res.Body.Close()
			logCandidateFailure(source.Link, request.link, err)
			lastErr = err
			continue
		}
		feed, err := parser.ParseAndFix(res.Body, request.link, getCharset(res))
		res.Body.Close()
		if err != nil {
			logCandidateFailure(source.Link, request.link, err)
			lastErr = err
			continue
		}
		return feed, nil
	}
	return nil, lastErr
}

type generatedRSSHubRequest struct {
	base string
	link string
}

func (w *Worker) generatedRSSHubRequests(source BilibiliTopHubSource) ([]generatedRSSHubRequest, error) {
	if !rsshub.IsLink(source.Link) {
		return []generatedRSSHubRequest{{link: source.Link}}, nil
	}

	bases, err := w.enabledRSSHubBasesForRequest()
	if err != nil {
		return nil, err
	}

	requests := make([]generatedRSSHubRequest, 0, len(bases))
	for _, base := range bases {
		link, err := rsshub.Resolve(source.Link, base)
		if err != nil {
			return nil, err
		}
		requests = append(requests, generatedRSSHubRequest{base: base, link: link})
	}
	return requests, nil
}

func BilibiliRankingEntries(feed *parser.Feed) []BilibiliTopEntry {
	if feed == nil {
		return nil
	}
	entries := make([]BilibiliTopEntry, 0, len(feed.Items))
	for i, item := range feed.Items {
		entries = append(entries, BilibiliTopEntry{
			Rank:     i + 1,
			Title:    item.Title,
			Author:   item.Author,
			CoverURL: firstImageMediaLink(item.MediaLinks),
			VideoURL: item.URL,
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

func RenderBilibiliTopContent(entries []BilibiliTopEntry) string {
	var buffer bytes.Buffer
	for i, entry := range entries {
		title := html.EscapeString(entry.Title)
		videoURL := html.EscapeString(entry.VideoURL)
		coverURL := html.EscapeString(entry.CoverURL)
		author := html.EscapeString(entry.Author)

		if i > 0 {
			buffer.WriteString(`<hr>`)
		}
		buffer.WriteString(`<article>`)
		if coverURL != "" {
			buffer.WriteString(`<p><a href="`)
			buffer.WriteString(videoURL)
			buffer.WriteString(`"><img src="`)
			buffer.WriteString(coverURL)
			buffer.WriteString(`" alt="`)
			buffer.WriteString(title)
			buffer.WriteString(`" width="320"></a></p>`)
		}
		buffer.WriteString(`<p>`)
		buffer.WriteString(`排名 `)
		buffer.WriteString(strconv.Itoa(entry.Rank))
		if author != "" {
			buffer.WriteString(`&nbsp;&nbsp; 作者 `)
			buffer.WriteString(author)
		}
		buffer.WriteString(`</p>`)
		buffer.WriteString(`<p><a style="text-decoration: none;" href="`)
		buffer.WriteString(videoURL)
		buffer.WriteString(`">《`)
		buffer.WriteString(title)
		buffer.WriteString(`》</a></p>`)
		buffer.WriteString(`</article>`)
	}
	return buffer.String()
}

func EnsureBilibiliTopSource(db *storage.Storage) {
	EnsureBilibiliTopHubSource(db, BilibiliTopSource)
}

func EnsureBilibiliZhishiSource(db *storage.Storage) {
	EnsureBilibiliTopHubSource(db, BilibiliZhishiSource)
}

func EnsureBilibiliTopHubSource(db *storage.Storage, source BilibiliTopHubSource) {
	db.UpsertGeneratedRSSSource(
		source.Key,
		source.Title,
		source.Link,
		source.Description,
	)
}
