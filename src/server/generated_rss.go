package server

import (
	"encoding/xml"
	"net/http"
	"time"

	"github.com/nkanaev/yarr/src/server/router"
	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/worker"
)

const generatedRSSItemLimit = 100

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
	Description string  `xml:"description"`
}

type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

func (s *Server) handleBilibiliTopRSS(c *router.Context) {
	s.handleBilibiliTopHubRSS(c, worker.BilibiliTopSource)
}

func (s *Server) handleBilibiliZhishiRSS(c *router.Context) {
	s.handleBilibiliTopHubRSS(c, worker.BilibiliZhishiSource)
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
	xml.NewEncoder(c.Out).Encode(generatedRSSDocument(*source, s.db.ListGeneratedRSSItems(source.Key, generatedRSSItemLimit)))
}

func generatedRSSDocument(source storage.GeneratedRSSSource, items []storage.GeneratedRSSItem) rssDocument {
	rssItems := make([]rssItem, len(items))
	for i, item := range items {
		rssItems[i] = rssItem{
			Title: item.Title,
			Link:  item.Link,
			GUID: rssGUID{
				IsPermaLink: "false",
				Value:       item.GUID,
			},
			PubDate:     item.PublishedAt.Format(time.RFC1123Z),
			Description: item.Content,
		}
	}

	return rssDocument{
		Version: "2.0",
		Channel: rssChannel{
			Title:       source.Title,
			Link:        source.Link,
			Description: source.Description,
			Items:       rssItems,
		},
	}
}
