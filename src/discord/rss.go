package discord

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

type channel struct {
	ID      string `json:"id"`
	GuildID string `json:"guild_id"`
	Name    string `json:"name"`
	Topic   string `json:"topic"`
	Type    *int   `json:"type"`
}
type author struct {
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
}
type attachment struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}
type embed struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Fields      []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"fields"`
}
type message struct {
	ID          string       `json:"id"`
	ChannelID   string       `json:"channel_id"`
	Content     string       `json:"content"`
	Timestamp   string       `json:"timestamp"`
	Author      author       `json:"author"`
	Attachments []attachment `json:"attachments"`
	Embeds      []embed      `json:"embeds"`
}
type thread struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type rss struct {
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
type guid struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}
type rssEnclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length int64  `xml:"length,attr"`
}

type rssItem struct {
	Enclosure   *rssEnclosure `xml:"enclosure,omitempty"`
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	GUID        guid          `xml:"guid"`
	PubDate     string        `xml:"pubDate"`
	Author      string        `xml:"http://purl.org/dc/elements/1.1/ creator,omitempty"`
	Description string        `xml:"description"`
}

func encodeRSS(r rss) ([]byte, error) {
	b, err := xml.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), b...), nil
}
func numericID(s string) bool {
	if s == "" || len(s) > 20 {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}
func newer(a, b string) bool {
	x, _ := strconv.ParseUint(a, 10, 64)
	y, _ := strconv.ParseUint(b, 10, 64)
	return x > y
}
func textHTML(s string) string { return strings.ReplaceAll(html.EscapeString(s), "\n", "<br>\n") }
func linkHTML(raw, label string) string {
	if label == "" {
		label = raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return textHTML(label)
	}
	return `<a href="` + html.EscapeString(raw) + `">` + textHTML(label) + `</a>`
}
func attachmentImageType(a attachment) string {
	u, err := url.Parse(a.URL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	if a.ContentType != "" {
		contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(a.ContentType, ";", 2)[0]))
		if strings.HasPrefix(contentType, "image/") {
			return contentType
		}
		return ""
	}
	switch strings.ToLower(path.Ext(a.Filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".png", ".gif", ".webp", ".avif", ".bmp":
		return "image/" + strings.ToLower(path.Ext(a.Filename))[1:]
	}
	return ""
}

func firstImageEnclosure(m message) *rssEnclosure {
	for _, a := range m.Attachments {
		if contentType := attachmentImageType(a); contentType != "" {
			return &rssEnclosure{URL: a.URL, Type: contentType, Length: a.Size}
		}
	}
	return nil
}

func attachmentHTML(a attachment) string {
	link := linkHTML(a.URL, a.Filename)
	if attachmentImageType(a) == "" {
		return link
	}
	return `<img src="` + html.EscapeString(a.URL) + `" alt="` + html.EscapeString(a.Filename) + `"><br>` + link
}

func bodyHTML(m message) string {
	parts := []string{textHTML(m.Content)}
	for _, e := range m.Embeds {
		parts = append(parts, linkHTML(e.URL, e.Title), textHTML(e.Description))
		for _, f := range e.Fields {
			parts = append(parts, textHTML(f.Name)+": "+textHTML(f.Value))
		}
	}
	for _, a := range m.Attachments {
		parts = append(parts, attachmentHTML(a))
	}
	return strings.Join(parts, "<br>\n")
}
func messageItem(m message, base string) (rssItem, error) {
	if !numericID(m.ID) {
		return rssItem{}, &fetchError{"invalid"}
	}
	date, err := time.Parse(time.RFC3339Nano, m.Timestamp)
	if err != nil {
		return rssItem{}, &fetchError{"invalid"}
	}
	title := strings.SplitN(m.Content, "\n", 2)[0]
	if title == "" {
		for _, e := range m.Embeds {
			if e.Title != "" {
				title = e.Title
				break
			}
		}
	}
	if title == "" {
		title = "消息 " + m.ID
	}
	name := m.Author.GlobalName
	if name == "" {
		name = m.Author.Username
	} else if m.Author.Username != "" {
		name += " (" + m.Author.Username + ")"
	}
	return rssItem{Enclosure: firstImageEnclosure(m), Title: title, Link: base + "/" + m.ID, GUID: guid{"false", m.ID}, PubDate: date.UTC().Format(time.RFC1123Z), Author: name, Description: bodyHTML(m)}, nil
}
func (c *client) fetch(ctx context.Context, id, token string) ([]byte, error) {
	var ch channel
	if err := c.get(ctx, "/channels/"+id, nil, token, &ch); err != nil {
		return nil, err
	}
	if ch.ID != id || ch.Type == nil || (ch.GuildID != "" && !numericID(ch.GuildID)) {
		return nil, &fetchError{"invalid"}
	}
	// Text, announcements, DMs, threads, forums and media channels.
	switch *ch.Type {
	case 0, 1, 3, 5, 10, 11, 12, 15, 16:
	default:
		return nil, &fetchError{"invalid"}
	}
	guild := ch.GuildID
	if guild == "" {
		guild = "@me"
	}
	base := "https://discord.com/channels/" + guild + "/" + id
	result := rss{Version: "2.0", Channel: rssChannel{Title: "#" + ch.Name + " - Discord", Link: base, Description: ch.Topic}}
	if *ch.Type == 15 || *ch.Type == 16 {
		items, err := c.forum(ctx, ch, token)
		if err != nil {
			return nil, err
		}
		result.Channel.Items = items
	} else {
		var messages []message
		if err := c.get(ctx, "/channels/"+id+"/messages", url.Values{"limit": {"100"}}, token, &messages); err != nil {
			return nil, err
		}
		if messages == nil {
			return nil, &fetchError{"invalid"}
		}
		seen := map[string]bool{}
		for _, m := range messages {
			item, err := messageItem(m, base)
			if err != nil {
				return nil, err
			}
			if !seen[m.ID] {
				seen[m.ID] = true
				result.Channel.Items = append(result.Channel.Items, item)
			}
		}
		sort.Slice(result.Channel.Items, func(i, j int) bool {
			return newer(result.Channel.Items[i].GUID.Value, result.Channel.Items[j].GUID.Value)
		})
		if len(result.Channel.Items) > 100 {
			result.Channel.Items = result.Channel.Items[:100]
		}
	}
	return encodeRSS(result)
}
func (c *client) forum(ctx context.Context, ch channel, token string) ([]rssItem, error) {
	threads := map[string]thread{}
	starters := map[string]message{}
	for _, archived := range []string{"false", "true"} {
		seen := map[string]bool{}
		offset := 0
		for len(seen) < 100 {
			var page struct {
				Threads       []thread  `json:"threads"`
				FirstMessages []message `json:"first_messages"`
				HasMore       *bool     `json:"has_more"`
			}
			q := url.Values{"archived": {archived}, "sort_by": {"creation_time"}, "sort_order": {"desc"}, "limit": {"25"}, "offset": {strconv.Itoa(offset)}, "tag_setting": {"match_some"}}
			if err := c.get(ctx, "/channels/"+ch.ID+"/threads/search", q, token, &page); err != nil {
				return nil, err
			}
			if page.Threads == nil {
				return nil, &fetchError{"invalid"}
			}
			if len(page.Threads) == 0 {
				break
			}
			before := len(seen)
			for _, t := range page.Threads {
				if !numericID(t.ID) {
					return nil, &fetchError{"invalid"}
				}
				threads[t.ID] = t
				seen[t.ID] = true
			}
			for _, m := range page.FirstMessages {
				if !numericID(m.ChannelID) || !numericID(m.ID) {
					return nil, &fetchError{"invalid"}
				}
				starters[m.ChannelID] = m
			}
			if len(seen) == before {
				return nil, &fetchError{"invalid"}
			}
			if archived == "true" && len(threads) >= 100 {
				ids := sortedThreads(threads)
				cutoff := ids[99]
				stop := false
				for _, t := range page.Threads {
					if !newer(t.ID, cutoff) {
						stop = true
					}
				}
				if stop {
					break
				}
			}
			if page.HasMore != nil && !*page.HasMore {
				break
			}
			offset += len(page.Threads)
		}
	}
	ids := sortedThreads(threads)
	if len(ids) > 100 {
		ids = ids[:100]
	}
	items := make([]rssItem, 0, len(ids))
	for _, id := range ids {
		n, _ := strconv.ParseUint(id, 10, 64)
		created := time.UnixMilli(int64(n>>22) + 1420070400000)
		item := rssItem{Title: threads[id].Name, Link: "https://discord.com/channels/" + ch.GuildID + "/" + id, GUID: guid{"false", id}, PubDate: created.UTC().Format(time.RFC1123Z)}
		if m, ok := starters[id]; ok {
			item.Description = bodyHTML(m)
			item.Enclosure = firstImageEnclosure(m)
			item.Author = m.Author.GlobalName
			if item.Author == "" {
				item.Author = m.Author.Username
			}
		}
		items = append(items, item)
	}
	return items, nil
}
func sortedThreads(ts map[string]thread) []string {
	ids := make([]string, 0, len(ts))
	for id := range ts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return newer(ids[i], ids[j]) })
	return ids
}
func noticeRSS(old []byte, id, code string, at int64) ([]byte, error) {
	r := rss{Version: "2.0", Channel: rssChannel{Title: "Discord 频道 " + id, Link: "https://discord.com", Description: "Discord 订阅状态"}}
	if len(old) > 0 {
		if err := xml.Unmarshal(old, &r); err != nil {
			return nil, err
		}
	}
	notice := rssItem{Title: failureText(code), Description: failureText(code), GUID: guid{"false", fmt.Sprintf("discord-notice:%s:%s:%d", id, code, at)}, Link: r.Channel.Link, PubDate: time.Unix(at, 0).UTC().Format(time.RFC1123Z)}
	r.Channel.Items = append([]rssItem{notice}, r.Channel.Items...)
	return encodeRSS(r)
}
