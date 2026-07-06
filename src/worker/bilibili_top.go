package worker

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/nkanaev/yarr/src/content/htmlutil"
	"github.com/nkanaev/yarr/src/storage"
	xhtml "golang.org/x/net/html"
)

const (
	BilibiliTopSourceKey         = "bilibili_top"
	BilibiliTopSourceTitle       = "B站全站热榜"
	BilibiliTopSourceLink        = "https://tophub.today/n/74KvxwokxM"
	BilibiliTopSourceDescription = "B站全站热榜小时快照"

	BilibiliZhishiSourceKey         = "bilibili_zhishi"
	BilibiliZhishiSourceTitle       = "B站知识榜"
	BilibiliZhishiSourceLink        = "https://tophub.today/n/Ywv47GzoPa"
	BilibiliZhishiSourceDescription = "B站知识榜小时快照"
)

type BilibiliTopEntry struct {
	Rank     int
	Title    string
	Views    string
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

	body, err := GetBody(source.Link)
	if err != nil {
		log.Printf("failed to fetch %s: %s", source.Title, err)
		return
	}
	if !w.SaveBilibiliTopHubSource(source, body, time.Now()) {
		log.Printf("failed to save %s", source.Title)
	}
}

func (w *Worker) SaveBilibiliTop(body string, now time.Time) bool {
	return w.SaveBilibiliTopHubSource(BilibiliTopSource, body, now)
}

func (w *Worker) SaveBilibiliTopHubSource(source BilibiliTopHubSource, body string, now time.Time) bool {
	entries, err := ParseBilibiliTop(body)
	if err != nil {
		log.Printf("failed to parse %s: %s", source.Title, err)
		return false
	}
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

func ParseBilibiliTop(body string) ([]BilibiliTopEntry, error) {
	root, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	entries := make([]BilibiliTopEntry, 0)
	for _, tr := range htmlutil.Query(root, "tr") {
		entry, ok := parseBilibiliTopRow(tr)
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func parseBilibiliTopRow(row *xhtml.Node) (BilibiliTopEntry, bool) {
	cells := childElements(row, "td")
	if len(cells) < 3 {
		return BilibiliTopEntry{}, false
	}

	rankText := strings.TrimSuffix(strings.TrimSpace(htmlutil.Text(cells[0])), ".")
	rank, err := strconv.Atoi(rankText)
	if err != nil {
		return BilibiliTopEntry{}, false
	}

	var coverURL string
	if imgs := htmlutil.Query(cells[1], "img"); len(imgs) > 0 {
		coverURL = strings.TrimSpace(htmlutil.Attr(imgs[0], "src"))
	}

	links := htmlutil.Query(cells[2], "a")
	if len(links) == 0 {
		return BilibiliTopEntry{}, false
	}
	videoURL := strings.TrimSpace(htmlutil.Attr(links[0], "href"))
	title := strings.TrimSpace(htmlutil.Text(links[0]))
	if title == "" || videoURL == "" {
		return BilibiliTopEntry{}, false
	}

	views := ""
	for _, div := range htmlutil.Query(cells[2], "div") {
		if strings.Contains(" "+htmlutil.Attr(div, "class")+" ", " item-desc ") {
			views = strings.TrimSpace(htmlutil.Text(div))
			break
		}
	}

	return BilibiliTopEntry{
		Rank:     rank,
		Title:    title,
		Views:    views,
		CoverURL: coverURL,
		VideoURL: videoURL,
	}, true
}

func childElements(node *xhtml.Node, name string) []*xhtml.Node {
	result := make([]*xhtml.Node, 0)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && child.Data == name {
			result = append(result, child)
		}
	}
	return result
}

func RenderBilibiliTopContent(entries []BilibiliTopEntry) string {
	var buffer bytes.Buffer
	for i, entry := range entries {
		title := html.EscapeString(entry.Title)
		videoURL := html.EscapeString(entry.VideoURL)
		coverURL := html.EscapeString(entry.CoverURL)
		views := html.EscapeString(entry.Views)

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
		if views != "" {
			buffer.WriteString(`&nbsp;&nbsp; 播放量 `)
			buffer.WriteString(views)
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
