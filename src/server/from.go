package server

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nkanaev/yarr/src/server/router"
	"github.com/nkanaev/yarr/src/storage"
)

type fromRSSCache struct {
	body      []byte
	expiresAt time.Time
}

type fromRSS struct {
	XMLName xml.Name       `xml:"rss"`
	Version string         `xml:"version,attr"`
	Channel fromRSSChannel `xml:"channel"`
}

type fromRSSChannel struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	Items       []fromRSSItem `xml:"item"`
}

type fromRSSItem struct {
	Title       string         `xml:"title"`
	Link        string         `xml:"link"`
	GUID        fromRSSGUID    `xml:"guid"`
	PubDate     string         `xml:"pubDate"`
	Author      string         `xml:"http://purl.org/dc/elements/1.1/ creator,omitempty"`
	Description string         `xml:"description"`
	Media       []fromRSSMedia `xml:"http://search.yahoo.com/mrss/ content"`
}

type fromRSSGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type fromRSSMedia struct {
	URL         string `xml:"url,attr"`
	Medium      string `xml:"medium,attr"`
	Description string `xml:"http://search.yahoo.com/mrss/ description,omitempty"`
}

func (s *Server) handleFromRSS(c *router.Context) {
	c.Out.Header().Set("Cache-Control", "no-store")
	if c.Req.Method != http.MethodGet {
		c.Out.Header().Set("Allow", "GET")
		c.Out.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Preserve escapes in a directly appended link; also accept a whole link
	// encoded as one path component, which the router has already decoded.
	link := strings.TrimPrefix(c.Req.URL.EscapedPath(), s.BasePath+"/rss/from/")
	if !strings.Contains(link, "://") {
		link = c.Vars["url"]
	}
	if c.Req.URL.RawQuery != "" || c.Req.URL.ForceQuery {
		link += "?" + c.Req.URL.RawQuery
	}
	u, err := url.Parse(link)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "rsshub") {
		http.Error(c.Out, "订阅链接必须是 HTTP(S) 或 rsshub:// 链接。", http.StatusBadRequest)
		return
	}

	body, status := s.fromRSSBody(link)
	if status != http.StatusOK {
		c.Out.WriteHeader(status)
		return
	}
	c.Out.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	c.Out.Write(body)
}

func (s *Server) fromRSSBody(link string) ([]byte, int) {
	s.cache_mutex.Lock()
	defer s.cache_mutex.Unlock()

	key := "rss/from:" + link
	now := time.Now()
	if cached, ok := s.cache[key].(fromRSSCache); ok && now.Before(cached.expiresAt) {
		return cached.body, http.StatusOK
	}
	for key, value := range s.cache {
		if cached, ok := value.(fromRSSCache); ok && !now.Before(cached.expiresAt) {
			delete(s.cache, key)
		}
	}

	feed := s.db.GetFeedByFeedLink(link)
	if feed == nil {
		return nil, http.StatusNotFound
	}
	rss := fromRSS{Version: "2.0", Channel: fromRSSChannel{
		Title: feed.Title, Link: feed.Link, Description: feed.Description,
	}}
	for _, item := range s.db.ListItems(storage.ItemFilter{FeedID: &feed.Id}, 100, true, true) {
		guid := item.GUID
		if guid == "" {
			guid = fmt.Sprintf("yarr:%d:%d", feed.Id, item.Id)
		}
		x := fromRSSItem{
			Title: item.Title, Link: item.Link, GUID: fromRSSGUID{"false", guid},
			PubDate: item.Date.UTC().Format(time.RFC1123Z), Author: item.Author,
			Description: item.Content,
		}
		for _, media := range item.MediaLinks {
			x.Media = append(x.Media, fromRSSMedia{URL: media.URL, Medium: media.Type, Description: media.Description})
		}
		rss.Channel.Items = append(rss.Channel.Items, x)
	}
	body, err := xml.Marshal(rss)
	if err != nil {
		return nil, http.StatusInternalServerError
	}
	body = append([]byte(xml.Header), body...)
	s.cache[key] = fromRSSCache{body: body, expiresAt: time.Now().Add(5 * time.Minute)}
	return body, http.StatusOK
}
