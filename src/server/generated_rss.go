package server

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"time"

	"golang.org/x/net/html"

	"github.com/nkanaev/yarr/src/server/router"
	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/worker"
)

const generatedRSSItemLimit = 100

type generatedRSSFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	LastBuild   string    `xml:"lastBuildDate"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string        `xml:"title"`
	GUID        string        `xml:"guid"`
	Link        string        `xml:"link"`
	PubDate     string        `xml:"pubDate"`
	Description string        `xml:"description"`
	Enclosure   *rssEnclosure `xml:"enclosure,omitempty"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

func (s *Server) handleBilibiliTopRSS(c *router.Context) {
	s.handleBilibiliTopHubRSS(c, worker.BilibiliTopSource)
}

func (s *Server) handleBilibiliZhishiRSS(c *router.Context) {
	s.handleBilibiliTopHubRSS(c, worker.BilibiliZhishiSource)
}

func (s *Server) handleBilibiliHotRSS(c *router.Context) {
	s.handleBilibiliTopHubRSS(c, worker.BilibiliHotSource)
}

func (s *Server) handleBilibiliTopHubRSS(c *router.Context, sourceConfig worker.BilibiliTopHubSource) {
	if c.Req.Method != "GET" {
		c.Out.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	worker.EnsureBilibiliTopHubSource(s.db, sourceConfig)
	source := s.db.GetGeneratedRSSSource(sourceConfig.Key)
	if source == nil {
		c.Out.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.Out.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	c.Out.WriteHeader(http.StatusOK)
	c.Out.Write([]byte(xml.Header))
	xml.NewEncoder(c.Out).Encode(generatedRSSFeedForSource(*source, s.db.ListGeneratedRSSItems(source.Key, generatedRSSItemLimit), c.Req.URL.String()))
}

func generatedRSSFeedForSource(source storage.GeneratedRSSSource, items []storage.GeneratedRSSItem, self string) generatedRSSFeed {
	rssItems := make([]rssItem, len(items))
	updated := time.Now().UTC()
	for i, item := range items {
		if i == 0 {
			updated = item.PublishedAt
		}
		rssItems[i] = rssItem{
			Title:       item.Title,
			GUID:        item.GUID,
			Link:        item.Link,
			PubDate:     item.PublishedAt.Format(time.RFC1123Z),
			Description: item.Content,
			Enclosure:   generatedRSSEnclosure(item),
		}
	}

	channelLink := self
	if channelLink == "" {
		channelLink = source.Link
	}

	return generatedRSSFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       source.Title,
			Link:        channelLink,
			Description: source.Description,
			LastBuild:   updated.Format(time.RFC1123Z),
			Items:       rssItems,
		},
	}
}

func generatedRSSEnclosure(item storage.GeneratedRSSItem) *rssEnclosure {
	if enclosureURL := firstImageURL(item.Content); enclosureURL != "" {
		return &rssEnclosure{
			URL:  enclosureURL,
			Type: "image/jpeg",
		}
	}
	return nil
}

func firstImageURL(content string) string {
	tokenizer := html.NewTokenizer(bytes.NewBufferString(content))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			if tokenizer.Err() != io.EOF {
				return ""
			}
			return ""
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data != "img" {
				continue
			}
			for _, attr := range token.Attr {
				if attr.Key == "src" {
					return attr.Val
				}
			}
		}
	}
}
