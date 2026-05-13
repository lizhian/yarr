package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/nkanaev/yarr/src/storage"
)

func TestStatic(t *testing.T) {
	handler := NewServer(nil, "127.0.0.1:8000").handler()
	url := "/static/javascripts/app.js"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", url, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != 200 {
		t.FailNow()
	}
}

func TestStaticWithBase(t *testing.T) {
	server := NewServer(nil, "127.0.0.1:8000")
	server.BasePath = "/sub"

	handler := server.handler()
	url := "/sub/static/javascripts/app.js"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", url, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != 200 {
		t.FailNow()
	}
}

func TestStaticBanTemplates(t *testing.T) {
	handler := NewServer(nil, "127.0.0.1:8000").handler()
	url := "/static/login.html"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", url, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != 404 {
		t.FailNow()
	}
}

func TestIndexGzipped(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	log.SetOutput(os.Stderr)
	handler := NewServer(db, "127.0.0.1:8000").handler()
	url := "/"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", url, nil)
	request.Header.Set("accept-encoding", "gzip")
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	if response.StatusCode != 200 {
		t.FailNow()
	}
	if response.Header.Get("content-encoding") != "gzip" {
		t.Errorf("invalid content-encoding header: %#v", response.Header.Get("content-encoding"))
	}
	if response.Header.Get("content-type") != "text/html" {
		t.Errorf("invalid content-type header: %#v", response.Header.Get("content-type"))
	}
}

func TestFeedIcons(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	icon := []byte("test")
	feed := db.CreateFeed("", "", "", "", nil)
	db.UpdateFeedIcon(feed.Id, &icon)
	log.SetOutput(os.Stderr)

	recorder := httptest.NewRecorder()
	url := fmt.Sprintf("/api/feeds/%d/icon", feed.Id)
	request := httptest.NewRequest("GET", url, nil)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()

	if response.StatusCode != http.StatusOK {
		t.Fatal()
	}
	body, _ := io.ReadAll(response.Body)
	if !reflect.DeepEqual(body, icon) {
		t.Fatal()
	}
	if response.Header.Get("Etag") == "" {
		t.Fatal()
	}

	recorder2 := httptest.NewRecorder()
	request2 := httptest.NewRequest("GET", url, nil)
	request2.Header.Set("If-None-Match", response.Header.Get("Etag"))
	handler.ServeHTTP(recorder2, request2)
	response2 := recorder2.Result()

	if response2.StatusCode != http.StatusNotModified {
		t.Fatal("got", response2.StatusCode)
	}
}

func TestMissingItemReturnsNotFound(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	log.SetOutput(os.Stderr)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/items/1178", nil)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusNotFound {
		t.Fatal("got", recorder.Result().StatusCode)
	}
}

func TestCreateRSSHubFeedRequiresBaseURL(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"url":"rsshub://bilibili/weekly"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/feeds", body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	var result map[string]string
	if err := json.NewDecoder(recorder.Result().Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "error" {
		t.Fatalf("got %q", result["status"])
	}
}

func TestUpdateFeedContentSelector(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"content_selector":".content__default"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.ContentSelector != ".content__default" {
		t.Fatalf("got %q", feed.ContentSelector)
	}
}

func TestUpdateFeedContentSelectorRejectsUnsupportedSelector(t *testing.T) {
	log.SetOutput(io.Discard)
	db, _ := storage.New(":memory:")
	feed := db.CreateFeed("feed", "", "https://example.com", "https://example.com/feed.xml", nil)
	log.SetOutput(os.Stderr)

	body := bytes.NewBufferString(`{"content_selector":"main .content"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", fmt.Sprintf("/api/feeds/%d", feed.Id), body)

	handler := NewServer(db, "127.0.0.1:8000").handler()
	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusBadRequest {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	feed = db.GetFeed(feed.Id)
	if feed.ContentSelector != "" {
		t.Fatalf("got %q", feed.ContentSelector)
	}
}
