package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nkanaev/yarr/src/content/htmlutil"
	"github.com/nkanaev/yarr/src/server/auth"
	"github.com/nkanaev/yarr/src/server/router"
	"github.com/nkanaev/yarr/src/storage"
)

const (
	greaderReadingList = "user/-/state/com.google/reading-list"
	greaderRead        = "user/-/state/com.google/read"
	greaderStarred     = "user/-/state/com.google/starred"
	greaderFresh       = "user/-/state/com.google/fresh"
	greaderAll         = "user/-/state/com.google/all"
	greaderBroadcast   = "user/-/state/com.google/broadcast"
)

const greaderItemLimit = 50

type greaderCategory struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

type greaderSubscription struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Categories []greaderCategory `json:"categories"`
	URL        string            `json:"url"`
	HTMLURL    string            `json:"htmlUrl"`
	IconURL    string            `json:"iconUrl,omitempty"`
	SortID     string            `json:"sortid"`
	FirstItem  string            `json:"firstitemmsec"`
}

type greaderOrigin struct {
	StreamID string `json:"streamId"`
	Title    string `json:"title"`
	HTMLURL  string `json:"htmlUrl"`
}

type greaderContent struct {
	Content string `json:"content"`
}

type greaderAlternate struct {
	Href string `json:"href"`
	Type string `json:"type"`
}

type greaderItem struct {
	ID            string             `json:"id"`
	CrawlTimeMsec string             `json:"crawlTimeMsec"`
	TimestampUsec string             `json:"timestampUsec"`
	Published     int64              `json:"published"`
	Updated       int64              `json:"updated"`
	Categories    []string           `json:"categories"`
	Title         string             `json:"title"`
	Alternate     []greaderAlternate `json:"alternate"`
	Summary       greaderContent     `json:"summary"`
	Content       greaderContent     `json:"content"`
	Origin        greaderOrigin      `json:"origin"`
	Canonical     []greaderAlternate `json:"canonical,omitempty"`
	Author        string             `json:"author"`
}

type greaderFeedRef struct {
	feed   storage.Feed
	folder *storage.Folder
}

func greaderFeedID(id int64) string {
	return fmt.Sprintf("feed/%d", id)
}

func greaderItemID(id int64) string {
	return fmt.Sprintf("tag:google.com,2005:reader/item/%d", id)
}

func greaderLabelID(title string) string {
	return "user/-/label/" + title
}

func greaderToken(username, password string) string {
	mac := hmac.New(sha256.New, []byte(password))
	mac.Write([]byte(username))
	return username + "/" + hex.EncodeToString(mac.Sum(nil))
}

func greaderWriteToken(authToken string) string {
	mac := hmac.New(sha256.New, []byte("greader-write-token"))
	mac.Write([]byte(authToken))
	return hex.EncodeToString(mac.Sum(nil))
}

func greaderAuthHeader(req *http.Request) string {
	header := req.Header.Get("Authorization")
	if !strings.HasPrefix(header, "GoogleLogin auth=") {
		return ""
	}
	return strings.TrimPrefix(header, "GoogleLogin auth=")
}

func greaderParseItemID(id string) (int64, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, false
	}
	if strings.HasPrefix(id, "tag:google.com,2005:reader/item/") {
		id = strings.TrimPrefix(id, "tag:google.com,2005:reader/item/")
	}
	x, err := strconv.ParseInt(id, 10, 64)
	return x, err == nil
}

func greaderParseFeedID(stream string) (int64, bool) {
	if !strings.HasPrefix(stream, "feed/") {
		return 0, false
	}
	x, err := strconv.ParseInt(strings.TrimPrefix(stream, "feed/"), 10, 64)
	return x, err == nil
}

func (s *Server) greaderAuthenticated(c *router.Context) (string, bool) {
	authConfig := s.db.GetAuthConfig()
	token := greaderAuthHeader(c.Req)
	if !authConfig.Enabled {
		return token, token != ""
	}
	if token == "" {
		return "", false
	}
	expected := greaderToken(authConfig.Username, authConfig.Password)
	return token, auth.StringsEqual(token, expected)
}

func (s *Server) handleGReader(c *router.Context) {
	path := "/" + c.Vars["path"]
	if path == "/accounts/ClientLogin" {
		s.greaderClientLogin(c)
		return
	}
	if !strings.HasPrefix(path, "/reader/api/0/") {
		c.Out.WriteHeader(http.StatusNotFound)
		return
	}
	authToken, ok := s.greaderAuthenticated(c)
	if !ok {
		c.Out.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch {
	case path == "/reader/api/0/user-info":
		s.greaderUserInfo(c)
	case path == "/reader/api/0/token":
		c.Out.Header().Set("Content-Type", "text/plain; charset=utf-8")
		c.Out.WriteHeader(http.StatusOK)
		c.Out.Write([]byte(greaderWriteToken(authToken)))
	case path == "/reader/api/0/subscription/list":
		s.greaderSubscriptionList(c)
	case path == "/reader/api/0/tag/list":
		s.greaderTagList(c)
	case path == "/reader/api/0/unread-count":
		s.greaderUnreadCount(c)
	case strings.HasPrefix(path, "/reader/api/0/stream/contents/"):
		s.greaderStreamContents(c, strings.TrimPrefix(path, "/reader/api/0/stream/contents/"))
	case path == "/reader/api/0/stream/items/ids":
		s.greaderStreamItemIDs(c)
	case path == "/reader/api/0/stream/items/contents":
		s.greaderStreamItemContents(c)
	case path == "/reader/api/0/edit-tag" || path == "/reader/api/0/stream/items/modify":
		s.greaderEditTag(c, authToken)
	default:
		c.Out.WriteHeader(http.StatusNotFound)
	}
}

func (s *Server) greaderClientLogin(c *router.Context) {
	if c.Req.Method != "POST" {
		c.Out.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	username := c.Req.FormValue("Email")
	password := c.Req.FormValue("Passwd")

	authConfig := s.db.GetAuthConfig()
	if authConfig.Enabled {
		if !auth.StringsEqual(username, authConfig.Username) || !auth.StringsEqual(password, authConfig.Password) {
			c.Out.WriteHeader(http.StatusForbidden)
			return
		}
	} else if username == "" {
		username = "yarr"
	}

	token := greaderToken(username, password)
	c.Out.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Out.WriteHeader(http.StatusOK)
	fmt.Fprintf(c.Out, "SID=%s\nLSID=%s\nAuth=%s\n", token, token, token)
}

func (s *Server) greaderUserInfo(c *router.Context) {
	authConfig := s.db.GetAuthConfig()
	username := authConfig.Username
	if username == "" {
		username = "yarr"
	}
	c.JSON(http.StatusOK, map[string]interface{}{
		"userId":        username,
		"userName":      username,
		"userProfileId": username,
		"userEmail":     username,
		"isBloggerUser": false,
		"signupTimeSec": 0,
	})
}

func (s *Server) greaderFeedRefs() map[int64]greaderFeedRef {
	folders := make(map[int64]storage.Folder)
	for _, folder := range s.db.ListFolders() {
		folders[folder.Id] = folder
	}
	refs := make(map[int64]greaderFeedRef)
	for _, feed := range s.db.ListFeeds() {
		ref := greaderFeedRef{feed: feed}
		if feed.FolderId != nil {
			if folder, ok := folders[*feed.FolderId]; ok {
				ref.folder = &folder
			}
		}
		refs[feed.Id] = ref
	}
	return refs
}

func (s *Server) greaderSubscriptionList(c *router.Context) {
	refs := s.greaderFeedRefs()
	subscriptions := make([]greaderSubscription, 0, len(refs))
	for _, ref := range refs {
		categories := make([]greaderCategory, 0)
		if ref.folder != nil {
			categories = append(categories, greaderCategory{
				ID:    greaderLabelID(ref.folder.Title),
				Label: ref.folder.Title,
			})
		}
		subscriptions = append(subscriptions, greaderSubscription{
			ID:         greaderFeedID(ref.feed.Id),
			Title:      ref.feed.Title,
			Categories: categories,
			URL:        ref.feed.FeedLink,
			HTMLURL:    ref.feed.Link,
			IconURL:    ref.feed.IconURL,
			SortID:     fmt.Sprintf("%08x", ref.feed.Id),
			FirstItem:  "0",
		})
	}
	c.JSON(http.StatusOK, map[string]interface{}{"subscriptions": subscriptions})
}

func (s *Server) greaderTagList(c *router.Context) {
	tags := []greaderCategory{
		{ID: greaderReadingList, Label: "reading-list"},
		{ID: greaderStarred, Label: "starred"},
	}
	for _, folder := range s.db.ListFolders() {
		tags = append(tags, greaderCategory{ID: greaderLabelID(folder.Title), Label: folder.Title})
	}
	c.JSON(http.StatusOK, map[string]interface{}{"tags": tags})
}

func (s *Server) greaderUnreadCount(c *router.Context) {
	type unreadCount struct {
		ID                    string `json:"id"`
		Count                 int64  `json:"count"`
		NewestItemTimestampUS string `json:"newestItemTimestampUsec"`
	}

	stats := s.db.FeedStats()
	refs := s.greaderFeedRefs()
	counts := make([]unreadCount, 0, len(stats)+len(refs)+1)
	var total int64
	folderCounts := make(map[int64]int64)
	for _, stat := range stats {
		if stat.UnreadCount == 0 {
			continue
		}
		total += stat.UnreadCount
		counts = append(counts, unreadCount{
			ID:                    greaderFeedID(stat.FeedId),
			Count:                 stat.UnreadCount,
			NewestItemTimestampUS: "0",
		})
		if ref, ok := refs[stat.FeedId]; ok && ref.feed.FolderId != nil {
			folderCounts[*ref.feed.FolderId] += stat.UnreadCount
		}
	}
	for _, folder := range s.db.ListFolders() {
		if count := folderCounts[folder.Id]; count > 0 {
			counts = append(counts, unreadCount{
				ID:                    greaderLabelID(folder.Title),
				Count:                 count,
				NewestItemTimestampUS: "0",
			})
		}
	}
	counts = append(counts, unreadCount{
		ID:                    greaderReadingList,
		Count:                 total,
		NewestItemTimestampUS: "0",
	})
	c.JSON(http.StatusOK, map[string]interface{}{
		"max":          1000,
		"unreadcounts": counts,
	})
}

func (s *Server) greaderStreamFilter(stream string, values url.Values) storage.ItemFilter {
	filter := storage.ItemFilter{}
	if stream == "" || stream == greaderReadingList || stream == greaderAll {
		// no-op
	} else if stream == greaderStarred {
		status := storage.STARRED
		filter.Status = &status
	} else if feedID, ok := greaderParseFeedID(stream); ok {
		filter.FeedID = &feedID
	} else if strings.HasPrefix(stream, "user/-/label/") {
		label := strings.TrimPrefix(stream, "user/-/label/")
		for _, folder := range s.db.ListFolders() {
			if folder.Title == label {
				filter.FolderID = &folder.Id
				break
			}
		}
	}

	for _, exclude := range values["xt"] {
		if exclude == greaderRead {
			status := storage.UNREAD
			filter.Status = &status
		}
	}
	for _, include := range values["it"] {
		if include == greaderStarred {
			status := storage.STARRED
			filter.Status = &status
		}
	}
	if after := values.Get("c"); after != "" {
		if id, ok := greaderParseItemID(after); ok {
			filter.After = &id
		}
	}
	if ot := values.Get("ot"); ot != "" {
		if sec, err := strconv.ParseInt(ot, 10, 64); err == nil {
			before := time.Unix(sec, 0)
			filter.Before = &before
		}
	}
	return filter
}

func greaderLimit(values url.Values) int {
	limit := greaderItemLimit
	if n, err := strconv.Atoi(values.Get("n")); err == nil && n > 0 && n < limit {
		limit = n
	}
	return limit
}

func (s *Server) greaderStreamContents(c *router.Context, stream string) {
	values := c.Req.URL.Query()
	filter := s.greaderStreamFilter(stream, values)
	limit := greaderLimit(values)
	items := s.db.ListItems(filter, limit+1, true, true)
	continuation := ""
	if len(items) == limit+1 {
		continuation = greaderItemID(items[limit-1].Id)
		items = items[:limit]
	}
	output := s.greaderItems(items)
	response := map[string]interface{}{
		"id":        stream,
		"updated":   time.Now().Unix(),
		"items":     output,
		"direction": "ltr",
	}
	if continuation != "" {
		response["continuation"] = continuation
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) greaderStreamItemIDs(c *router.Context) {
	values := c.Req.URL.Query()
	stream := values.Get("s")
	filter := s.greaderStreamFilter(stream, values)
	limit := greaderLimit(values)
	items := s.db.ListItems(filter, limit+1, true, false)
	continuation := ""
	if len(items) == limit+1 {
		continuation = greaderItemID(items[limit-1].Id)
		items = items[:limit]
	}
	refs := make([]map[string]string, len(items))
	for i, item := range items {
		refs[i] = map[string]string{"id": greaderItemID(item.Id)}
	}
	response := map[string]interface{}{"itemRefs": refs}
	if continuation != "" {
		response["continuation"] = continuation
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) greaderStreamItemContents(c *router.Context) {
	c.Req.ParseForm()
	ids := make([]int64, 0)
	for _, raw := range c.Req.Form["i"] {
		if id, ok := greaderParseItemID(raw); ok {
			ids = append(ids, id)
		}
	}
	filter := storage.ItemFilter{IDs: &ids}
	items := s.db.ListItems(filter, len(ids), false, true)
	c.JSON(http.StatusOK, map[string]interface{}{"items": s.greaderItems(items)})
}

func (s *Server) greaderItems(items []storage.Item) []greaderItem {
	refs := s.greaderFeedRefs()
	output := make([]greaderItem, len(items))
	for i, item := range items {
		ref := refs[item.FeedId]
		link := item.Link
		if !htmlutil.IsAPossibleLink(link) && ref.feed.Link != "" {
			link = htmlutil.AbsoluteUrl(link, ref.feed.Link)
		}
		title := item.Title
		if title == "" {
			title = htmlutil.TruncateText(htmlutil.ExtractText(item.Content), 140)
		}
		categories := []string{greaderFresh}
		if item.Status == storage.READ || item.Status == storage.STARRED {
			categories = append(categories, greaderRead)
		}
		if item.Status == storage.STARRED {
			categories = append(categories, greaderStarred)
		}
		if ref.folder != nil {
			categories = append(categories, greaderLabelID(ref.folder.Title))
		}
		alternate := []greaderAlternate{}
		if link != "" {
			alternate = append(alternate, greaderAlternate{Href: link, Type: "text/html"})
		}
		output[i] = greaderItem{
			ID:            greaderItemID(item.Id),
			CrawlTimeMsec: fmt.Sprintf("%d", item.Date.UnixMilli()),
			TimestampUsec: fmt.Sprintf("%d", item.Date.UnixMicro()),
			Published:     item.Date.Unix(),
			Updated:       item.Date.Unix(),
			Categories:    categories,
			Title:         title,
			Alternate:     alternate,
			Summary:       greaderContent{Content: item.Content},
			Content:       greaderContent{Content: item.Content},
			Origin: greaderOrigin{
				StreamID: greaderFeedID(item.FeedId),
				Title:    ref.feed.Title,
				HTMLURL:  ref.feed.Link,
			},
			Author: "",
		}
	}
	return output
}

func (s *Server) greaderEditTag(c *router.Context, authToken string) {
	if c.Req.Method != "POST" {
		c.Out.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	c.Req.ParseForm()
	token := c.Req.Form.Get("T")
	if token == "" {
		token = c.Req.Form.Get("t")
	}
	if token != "" && !auth.StringsEqual(token, greaderWriteToken(authToken)) {
		c.Out.WriteHeader(http.StatusForbidden)
		return
	}

	adds := c.Req.Form["a"]
	removes := c.Req.Form["r"]
	streams := c.Req.Form["s"]
	itemIDs := c.Req.Form["i"]

	if len(itemIDs) > 0 {
		for _, raw := range itemIDs {
			id, ok := greaderParseItemID(raw)
			if !ok {
				continue
			}
			item := s.db.GetItem(id)
			if item == nil {
				continue
			}
			status := item.Status
			if containsString(adds, greaderRead) {
				status = storage.READ
			}
			if containsString(adds, greaderStarred) {
				status = storage.STARRED
			}
			if containsString(removes, greaderRead) {
				status = storage.UNREAD
			}
			if containsString(removes, greaderStarred) && status == storage.STARRED {
				status = storage.READ
			}
			s.db.UpdateItemStatus(id, status)
		}
		c.Out.WriteHeader(http.StatusOK)
		return
	}

	if containsString(adds, greaderRead) {
		for _, stream := range streams {
			filter := storage.MarkFilter{}
			switch {
			case stream == "" || stream == greaderReadingList || stream == greaderAll:
			case strings.HasPrefix(stream, "feed/"):
				if feedID, ok := greaderParseFeedID(stream); ok {
					filter.FeedID = &feedID
				}
			case strings.HasPrefix(stream, "user/-/label/"):
				label := strings.TrimPrefix(stream, "user/-/label/")
				for _, folder := range s.db.ListFolders() {
					if folder.Title == label {
						filter.FolderID = &folder.Id
						break
					}
				}
			default:
				continue
			}
			s.db.MarkItemsRead(filter)
		}
	}
	c.Out.WriteHeader(http.StatusOK)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
