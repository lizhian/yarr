package opml

import (
	"fmt"
	"html"
	"strings"
)

type Folder struct {
	Title   string
	Folders []Folder
	Feeds   []Feed
}

type Feed struct {
	Title           string
	FeedUrl         string
	SiteUrl         string
	ContentSelector string
	IconURL         string
}

func (f Folder) AllFeeds() []Feed {
	feeds := make([]Feed, 0)
	feeds = append(feeds, f.Feeds...)
	for _, subfolder := range f.Folders {
		feeds = append(feeds, subfolder.AllFeeds()...)
	}
	return feeds
}

var e = html.EscapeString
var indent = "  "
var nl = "\n"

func (f Folder) outline(level int) string {
	builder := strings.Builder{}
	prefix := strings.Repeat(indent, level)

	if level > 0 {
		builder.WriteString(prefix + fmt.Sprintf(`<outline text="%s">`+nl, e(f.Title)))
	}
	for _, folder := range f.Folders {
		builder.WriteString(folder.outline(level + 1))
	}
	for _, feed := range f.Feeds {
		builder.WriteString(feed.outline(level + 1))
	}
	if level > 0 {
		builder.WriteString(prefix + `</outline>` + nl)
	}
	return builder.String()
}

func (f Feed) outline(level int) string {
	attrs := []string{
		fmt.Sprintf(`type="rss"`),
		fmt.Sprintf(`text="%s"`, e(f.Title)),
		fmt.Sprintf(`xmlUrl="%s"`, e(f.FeedUrl)),
		fmt.Sprintf(`htmlUrl="%s"`, e(f.SiteUrl)),
	}
	if f.IconURL != "" {
		attrs = append(attrs, fmt.Sprintf(`icon_url="%s"`, e(f.IconURL)))
	}
	if f.ContentSelector != "" {
		attrs = append(attrs, fmt.Sprintf(`content_selector="%s"`, e(f.ContentSelector)))
	}
	return strings.Repeat(indent, level) + fmt.Sprintf(
		`<outline %s/>`+nl,
		strings.Join(attrs, " "),
	)
}

func (f Folder) OPML() string {
	builder := strings.Builder{}
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + nl)
	builder.WriteString(`<opml version="1.1">` + nl)
	builder.WriteString(`<head><title>subscriptions</title></head>` + nl)
	builder.WriteString(`<body>` + nl)
	builder.WriteString(f.outline(0))
	builder.WriteString(`</body>` + nl)
	builder.WriteString(`</opml>` + nl)
	return builder.String()
}
