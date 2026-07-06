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

type atomFeed struct {
	XMLName  xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	XMLNS    string      `xml:"xmlns,attr"`
	Title    string      `xml:"title"`
	ID       string      `xml:"id"`
	Updated  string      `xml:"updated"`
	Links    []atomLink  `xml:"link"`
	Subtitle string      `xml:"subtitle"`
	Entries  []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

type atomEntry struct {
	Title     string      `xml:"title"`
	ID        string      `xml:"id"`
	Updated   string      `xml:"updated"`
	Published string      `xml:"published"`
	Link      atomLink    `xml:"link"`
	Content   atomContent `xml:"content"`
}

type atomContent struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
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

	c.Out.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	c.Out.WriteHeader(http.StatusOK)
	c.Out.Write([]byte(xml.Header))
	xml.NewEncoder(c.Out).Encode(generatedAtomFeed(*source, s.db.ListGeneratedRSSItems(source.Key, generatedRSSItemLimit), c.Req.URL.String()))
}

func generatedAtomFeed(source storage.GeneratedRSSSource, items []storage.GeneratedRSSItem, self string) atomFeed {
	entries := make([]atomEntry, len(items))
	updated := time.Now().UTC()
	for i, item := range items {
		if i == 0 {
			updated = item.PublishedAt
		}
		timestamp := item.PublishedAt.Format(time.RFC3339)
		entries[i] = atomEntry{
			Title:     item.Title,
			ID:        item.GUID,
			Updated:   timestamp,
			Published: timestamp,
			Link: atomLink{
				Href: item.Link,
			},
			Content: atomContent{
				Type:  "html",
				Value: item.Content,
			},
		}
	}

	return atomFeed{
		XMLNS:    "http://www.w3.org/2005/Atom",
		Title:    source.Title,
		ID:       source.Key,
		Updated:  updated.Format(time.RFC3339),
		Subtitle: source.Description,
		Links: []atomLink{
			{Href: source.Link, Rel: "alternate"},
			{Href: self, Rel: "self", Type: "application/atom+xml"},
		},
		Entries: entries,
	}
}
