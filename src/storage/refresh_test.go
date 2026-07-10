package storage

import (
	"errors"
	"testing"
	"time"
)

func TestApplyFeedRefreshCommitsItemsSearchStateAndErrorTogether(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("feed", "", "", "https://example.com/feed.xml", nil)
	db.SetFeedError(feed.Id, errors.New("old error"))

	created, ok := db.ApplyFeedRefresh(FeedRefreshUpdate{
		FeedID:         feed.Id,
		UpdateMetadata: true,
		Title:          "fresh title",
		Link:           "https://example.com",
		FeedLink:       feed.FeedLink,
		Items: []Item{{
			GUID:   "item-1",
			FeedId: feed.Id,
			Title:  "searchable title",
			Date:   time.Now(),
		}},
		UpdateFeedSize: true,
		FeedSize:       1,
		LastModified:   "last-modified",
		Etag:           "etag",
	})
	if !ok || created != 1 {
		t.Fatalf("created %d, ok %v", created, ok)
	}
	if errors := db.GetFeedErrors(); errors[feed.Id] != "" {
		t.Fatalf("feed error was not cleared: %#v", errors)
	}
	state := db.GetHTTPState(feed.Id)
	if state == nil || state.LastModified != "last-modified" || state.Etag != "etag" || state.LastRefreshed.IsZero() {
		t.Fatalf("invalid HTTP state: %#v", state)
	}
	search := "searchable"
	items := db.ListItems(ItemFilter{Search: &search}, 10, true, false)
	if len(items) != 1 || items[0].GUID != "item-1" {
		t.Fatalf("item was not indexed: %#v", items)
	}

	created, ok = db.ApplyFeedRefresh(FeedRefreshUpdate{
		FeedID: feed.Id,
		Items: []Item{{
			GUID:   "item-1",
			FeedId: feed.Id,
			Title:  "searchable title",
			Date:   time.Now(),
		}},
	})
	if !ok || created != 0 {
		t.Fatalf("duplicate created %d, ok %v", created, ok)
	}
}

func TestApplyFeedRefreshRollsBackItemsWhenHTTPStateFails(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("feed", "", "", "https://example.com/feed.xml", nil)
	db.SetHTTPState(feed.Id, "old-modified", "old-etag")

	created, ok := db.ApplyFeedRefresh(FeedRefreshUpdate{
		FeedID: feed.Id + 1000,
		Items: []Item{{
			GUID:   "item-1",
			FeedId: feed.Id,
			Title:  "title",
			Date:   time.Now(),
		}},
		LastModified: "new-modified",
		Etag:         "new-etag",
	})
	if ok || created != 0 {
		t.Fatalf("created %d, ok %v", created, ok)
	}
	if item := db.GetItemByGUID(feed.Id, "item-1"); item != nil {
		t.Fatalf("item was not rolled back: %#v", item)
	}
	state := db.GetHTTPState(feed.Id)
	if state == nil || state.LastModified != "old-modified" || state.Etag != "old-etag" {
		t.Fatalf("HTTP state changed after rollback: %#v", state)
	}
}
