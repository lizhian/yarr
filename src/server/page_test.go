package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/nkanaev/yarr/src/storage"
)

func TestPageCrawlUsesFeedContentSelector(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<html><body><nav><p>navigation text with enough words to be ignored</p></nav><article class="content__default"><h1>Title</h1><p>Unique fallback article content has enough words, commas, and detail to be selected.</p><p>Hello <a href="/next">world</a>.</p><script>alert(1)</script></article></body></html>`)
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("http_proxy", proxy.URL)
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	targetURL := "http://example.com/article"

	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeedWithContentSelector("feed", "", "http://example.com", "http://example.com/feed.xml", ".content__default", nil)
	log.SetOutput(os.Stderr)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/page?url="+url.QueryEscape(targetURL)+"&feed_id="+strconv.FormatInt(feed.Id, 10), nil)
	NewServer(db, "127.0.0.1:8000").handler().ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	var result map[string]string
	if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	content := result["content"]
	if !strings.Contains(content, `<h1>Title</h1>`) || !strings.Contains(content, `href="http://example.com/next"`) {
		t.Fatalf("selector content not returned: %s", content)
	}
	if strings.Contains(content, "script") || strings.Contains(content, "navigation") || strings.Contains(content, "content__default") {
		t.Fatalf("unexpected content: %s", content)
	}

	fallbackFeed := db.CreateFeedWithContentSelector("fallback", "", "http://example.com", "http://example.com/fallback.xml", ".missing", nil)
	fallbackRecorder := httptest.NewRecorder()
	fallbackRequest := httptest.NewRequest("GET", "/page?url="+url.QueryEscape(targetURL)+"&feed_id="+strconv.FormatInt(fallbackFeed.Id, 10), nil)
	NewServer(db, "127.0.0.1:8000").handler().ServeHTTP(fallbackRecorder, fallbackRequest)

	if fallbackRecorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", fallbackRecorder.Result().StatusCode)
	}
	if err := json.NewDecoder(fallbackRecorder.Result().Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result["content"], "Unique fallback article content") {
		t.Fatalf("readability fallback not returned: %s", result["content"])
	}
}
