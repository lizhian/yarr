package worker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/nkanaev/yarr/src/content/scraper"
	"github.com/nkanaev/yarr/src/parser"
	"github.com/nkanaev/yarr/src/rsshub"
	"github.com/nkanaev/yarr/src/storage"
	"golang.org/x/net/html/charset"
)

type FeedSource struct {
	Title string `json:"title"`
	Url   string `json:"url"`
}

type DiscoverResult struct {
	Feed     *parser.Feed
	FeedLink string
	Sources  []FeedSource
}

type FeedRefreshResult struct {
	FeedID         int64
	StoredFeedLink string
	RSSHubBase     string
	Feed           *parser.Feed
	FeedLink       string
	Items          []storage.Item
}

func DiscoverFeed(candidateUrl string) (*DiscoverResult, error) {
	return DiscoverFeedWithLink(candidateUrl, candidateUrl)
}

func DiscoverFeedWithLink(candidateUrl, feedLink string) (*DiscoverResult, error) {
	result := &DiscoverResult{}
	// Query URL
	res, err := client.get(candidateUrl)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status code %d", res.StatusCode)
	}
	cs := getCharset(res)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// Try to feed into parser
	feed, err := parser.ParseAndFix(bytes.NewReader(body), candidateUrl, cs)
	if err == nil {
		result.Feed = feed
		result.FeedLink = feedLink
		return result, nil
	}

	// Possibly an html link. Search for feed links
	content := string(body)
	if cs != "" {
		if r, err := charset.NewReaderLabel(cs, bytes.NewReader(body)); err == nil {
			if body, err := io.ReadAll(r); err == nil {
				content = string(body)
			}
		}
	}
	sources := make([]FeedSource, 0)
	for url, title := range scraper.FindFeeds(content, candidateUrl) {
		sources = append(sources, FeedSource{Title: title, Url: url})
	}
	switch {
	case len(sources) == 0:
		return nil, errors.New("No feeds found at the given url")
	case len(sources) == 1:
		if sources[0].Url == candidateUrl {
			return nil, errors.New("Recursion!")
		}
		return DiscoverFeedWithLink(sources[0].Url, sources[0].Url)
	}

	result.Sources = sources
	return result, nil
}

var imageTypes = map[string]bool{
	"image/x-icon":  true,
	"image/png":     true,
	"image/jpeg":    true,
	"image/gif":     true,
	"image/webp":    true,
	"image/svg+xml": true,
	"image/avif":    true,
}

func fetchImage(link string) (*[]byte, error) {
	content, _, err := fetchImageWithContentType(link)
	return content, err
}

func FetchImage(link string) (*[]byte, string, error) {
	return fetchImageWithContentType(link)
}

func fetchImageWithContentType(link string) (*[]byte, string, error) {
	res, err := client.get(link)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, "", fmt.Errorf("status code %d", res.StatusCode)
	}

	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	if len(content) == 0 {
		return nil, "", nil
	}

	ctype := http.DetectContentType(content)
	if imageTypes[ctype] {
		return &content, ctype, nil
	}
	if mediaType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type")); err == nil && imageTypes[mediaType] {
		return &content, mediaType, nil
	}
	return nil, "", nil
}

func validImageURL(link string) bool {
	content, _, err := fetchImageWithContentType(link)
	return err == nil && content != nil
}

func findFeedIconURL(feedImageUrl, siteUrl, feedUrl string) (string, error) {
	if feedImageUrl != "" {
		return feedImageUrl, nil
	}

	urls := make([]string, 0)
	favicon := func(link string) string {
		u, err := url.Parse(link)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%s://%s/favicon.ico", u.Scheme, u.Host)
	}

	if siteUrl != "" {
		if res, err := client.get(siteUrl); err == nil {
			defer res.Body.Close()
			if res.StatusCode == 200 {
				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					return "", err
				}
				urls = append(urls, scraper.FindIcons(string(body), siteUrl)...)
			}
			if c := favicon(siteUrl); c != "" {
				urls = append(urls, c)
			}
		}
	}

	if c := favicon(feedUrl); c != "" {
		urls = append(urls, c)
	}

	for _, u := range urls {
		if validImageURL(u) {
			return u, nil
		}
	}
	return "", nil
}

func (w *Worker) resolveLink(link string) (string, error) {
	links, err := w.resolveLinks(link)
	if err != nil {
		return "", err
	}
	return links[0], nil
}

func (w *Worker) DiscoverFeed(link string) (*DiscoverResult, error) {
	requestLinks, err := w.resolveLinks(link)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, requestLink := range requestLinks {
		result, err := DiscoverFeedWithLink(requestLink, link)
		if err == nil {
			return result, nil
		}
		logCandidateFailure(link, requestLink, err)
		lastErr = err
	}
	return nil, lastErr
}

func ConvertItems(items []parser.Item, feed storage.Feed) []storage.Item {
	result := make([]storage.Item, len(items))
	for i, item := range items {
		item := item
		mediaLinks := make(storage.MediaLinks, 0)
		for _, link := range item.MediaLinks {
			mediaLinks = append(mediaLinks, storage.MediaLink(link))
		}
		result[i] = storage.Item{
			GUID:       item.GUID,
			FeedId:     feed.Id,
			Title:      item.Title,
			Link:       item.URL,
			Content:    item.Content,
			Date:       item.Date,
			Status:     storage.UNREAD,
			MediaLinks: mediaLinks,
		}
	}
	return result
}

func listItems(f storage.Feed, db *storage.Storage) ([]storage.Item, error) {
	requestLinks, err := rsshub.ResolveWithBaseList(f.FeedLink, db.GetSettingsValueString("rsshub_base_url"), RSSHUB_MAX_ATTEMPTS)
	if err != nil {
		return nil, err
	}
	return listItemsFromLinks(f, requestLinks, db)
}

func listItemsFromLinks(f storage.Feed, requestLinks []string, db *storage.Storage) ([]storage.Item, error) {
	result, err := refreshFeedFromLinks(f, requestLinks, db)
	if err != nil || result == nil {
		return nil, err
	}
	return result.Items, nil
}

func refreshFeedFromLinks(f storage.Feed, requestLinks []string, db *storage.Storage) (*FeedRefreshResult, error) {
	lmod := ""
	etag := ""
	if state := db.GetHTTPState(f.Id); state != nil {
		lmod = state.LastModified
		etag = state.Etag
	}
	if f.HasRefreshMetadataPlaceholder() {
		lmod = ""
		etag = ""
	}

	var lastErr error
	for _, requestLink := range requestLinks {
		result, err := refreshFeedFromLink(f, requestLink, lmod, etag, db)
		if err == nil {
			return result, nil
		}
		logCandidateFailure(f.FeedLink, requestLink, err)
		lastErr = err
	}
	return nil, lastErr
}

func listItemsFromLink(f storage.Feed, requestLink, lmod, etag string, db *storage.Storage) ([]storage.Item, error) {
	result, err := refreshFeedFromLink(f, requestLink, lmod, etag, db)
	if err != nil || result == nil {
		return nil, err
	}
	return result.Items, nil
}

func refreshFeedFromLink(f storage.Feed, requestLink, lmod, etag string, db *storage.Storage) (*FeedRefreshResult, error) {
	res, err := client.getConditional(requestLink, lmod, etag)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	rsshubBase := rsshubBaseForRequestLink(f.FeedLink, requestLink, db)
	switch {
	case res.StatusCode < 200 || res.StatusCode > 399:
		if res.StatusCode == 404 {
			return nil, fmt.Errorf("feed not found")
		}
		return nil, fmt.Errorf("status code %d", res.StatusCode)
	case res.StatusCode == http.StatusNotModified:
		return &FeedRefreshResult{
			FeedID:         f.Id,
			StoredFeedLink: f.FeedLink,
			RSSHubBase:     rsshubBase,
			FeedLink:       requestLink,
		}, nil
	}

	feed, err := parser.ParseAndFix(res.Body, requestLink, getCharset(res))
	if err != nil {
		return nil, err
	}

	lmod = res.Header.Get("Last-Modified")
	etag = res.Header.Get("Etag")
	if lmod != "" || etag != "" {
		db.SetHTTPState(f.Id, lmod, etag)
	}
	feedLink := requestLink
	if res.Request != nil && res.Request.URL != nil {
		feedLink = res.Request.URL.String()
	}
	return &FeedRefreshResult{
		FeedID:         f.Id,
		StoredFeedLink: f.FeedLink,
		RSSHubBase:     rsshubBase,
		Feed:           feed,
		FeedLink:       feedLink,
		Items:          ConvertItems(feed.Items, f),
	}, nil
}

func rsshubBaseForRequestLink(link, requestLink string, db *storage.Storage) string {
	if !rsshub.IsLink(link) {
		return ""
	}
	bases, err := rsshub.EnabledBases(db.GetSettingsValueString("rsshub_base_url"))
	if err != nil {
		return ""
	}
	for _, base := range bases {
		resolved, err := rsshub.Resolve(link, base)
		if err == nil && resolved == requestLink {
			return base
		}
	}
	return ""
}

func logCandidateFailure(link, requestLink string, err error) {
	if link != requestLink {
		log.Printf("Failed RSSHub candidate %s for %s: %s", requestLink, link, err)
	}
}

func getCharset(res *http.Response) string {
	contentType := res.Header.Get("Content-Type")
	if _, params, err := mime.ParseMediaType(contentType); err == nil {
		if cs, ok := params["charset"]; ok {
			if e, _ := charset.Lookup(cs); e != nil {
				return cs
			}
		}
	}
	return ""
}

func GetBody(url string) (string, error) {
	res, err := client.get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var r io.Reader

	ctype := res.Header.Get("Content-Type")
	if strings.Contains(ctype, "charset") {
		r, err = charset.NewReader(res.Body, ctype)
		if err != nil {
			return "", err
		}
	} else {
		r = res.Body
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
