// Parser for RSS versions:
// - 0.91 netscape
// - 0.91 userland
// - 2.0
package parser

import (
	"encoding/xml"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

type rssFeed struct {
	XMLName xml.Name  `xml:"rss"`
	Version string    `xml:"version,attr"`
	Title   string    `xml:"channel>title"`
	Link    string    `xml:"channel>link"`
	Image   rssImage  `xml:"channel>image"`
	Items   []rssItem `xml:"channel>item"`
}

type rssImage struct {
	URL string `xml:"url"`
}

type rssItem struct {
	GUID        rssGuid        `xml:"guid"`
	Title       string         `xml:"title"`
	Link        string         `xml:"rss link"`
	Description rssDescription `xml:"rss description"`
	PubDate     string         `xml:"pubDate"`
	Enclosures  []rssEnclosure `xml:"enclosure"`

	DublinCoreDate string `xml:"http://purl.org/dc/elements/1.1/ date"`
	ContentEncoded string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`

	OrigLink          string `xml:"http://rssnamespace.org/feedburner/ext/1.0 origLink"`
	OrigEnclosureLink string `xml:"http://rssnamespace.org/feedburner/ext/1.0 origEnclosureLink"`

	media
}

type rssGuid struct {
	GUID        string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr"`
}

type rssDescription struct {
	Text string `xml:",chardata"`
	HTML string `xml:",innerxml"`
}

func (d rssDescription) String() string {
	return firstNonEmpty(d.Text, d.HTML)
}

func (d rssDescription) FirstImageSrc() string {
	if src := firstImageSrc(d.HTML); src != "" {
		return src
	}
	return firstImageSrc(d.Text)
}

type rssLink struct {
	XMLName xml.Name
	Data    string `xml:",chardata"`
	Href    string `xml:"href,attr"`
	Rel     string `xml:"rel,attr"`
}

type rssTitle struct {
	XMLName xml.Name
	Data    string `xml:",chardata"`
	Inner   string `xml:",innerxml"`
}

type rssEnclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

func rssEnclosureImageURL(e rssEnclosure) string {
	if e.URL == "" {
		return ""
	}
	if strings.HasPrefix(e.Type, "image/") {
		return e.URL
	}
	if e.Type != "" {
		return ""
	}

	path := e.URL
	if u, err := url.Parse(e.URL); err == nil && u.Path != "" {
		path = u.Path
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".avif", ".apng", ".gif", ".jpg", ".jpeg", ".png", ".svg", ".webp":
		return e.URL
	}
	return ""
}

func firstImageSrc(content string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(content))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			return ""
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}

		token := tokenizer.Token()
		if token.Data != "img" {
			continue
		}
		for _, attr := range token.Attr {
			if strings.EqualFold(attr.Key, "src") {
				return strings.TrimSpace(attr.Val)
			}
		}
	}
}

func hasImageMediaLink(links []MediaLink) bool {
	for _, link := range links {
		if link.Type == "image" {
			return true
		}
	}
	return false
}

func ParseRSS(r io.Reader) (*Feed, error) {
	srcfeed := rssFeed{}

	decoder := xmlDecoder(r)
	decoder.DefaultSpace = "rss"
	if err := decoder.Decode(&srcfeed); err != nil {
		return nil, err
	}

	dstfeed := &Feed{
		Title:    srcfeed.Title,
		SiteURL:  srcfeed.Link,
		ImageURL: srcfeed.Image.URL,
	}
	for _, srcitem := range srcfeed.Items {
		mediaLinks := srcitem.mediaLinks()
		for _, e := range srcitem.Enclosures {
			if imageURL := rssEnclosureImageURL(e); imageURL != "" {
				mediaLinks = append(mediaLinks, MediaLink{URL: imageURL, Type: "image"})
			} else if strings.HasPrefix(e.Type, "audio/") {
				podcastURL := e.URL
				if srcitem.OrigEnclosureLink != "" && strings.Contains(podcastURL, path.Base(srcitem.OrigEnclosureLink)) {
					podcastURL = srcitem.OrigEnclosureLink
				}
				mediaLinks = append(mediaLinks, MediaLink{URL: podcastURL, Type: "audio"})
				break
			}
		}
		if !hasImageMediaLink(mediaLinks) {
			if imageURL := srcitem.Description.FirstImageSrc(); imageURL != "" {
				mediaLinks = append(mediaLinks, MediaLink{URL: imageURL, Type: "image"})
			}
		}

		permalink := ""
		if srcitem.GUID.IsPermaLink == "true" {
			permalink = srcitem.GUID.GUID
		}

		dstfeed.Items = append(dstfeed.Items, Item{
			GUID:       firstNonEmpty(srcitem.GUID.GUID, srcitem.Link),
			Date:       dateParse(firstNonEmpty(srcitem.DublinCoreDate, srcitem.PubDate)),
			URL:        firstNonEmpty(srcitem.OrigLink, srcitem.Link, permalink),
			Title:      srcitem.Title,
			Content:    firstNonEmpty(srcitem.ContentEncoded, srcitem.Description.String(), srcitem.firstMediaDescription()),
			MediaLinks: mediaLinks,
		})
	}
	return dstfeed, nil
}
