package storage

import (
	"reflect"
	"testing"
	"time"
)

func TestCreateFeed(t *testing.T) {
	db := testDB()
	feed1 := db.CreateFeed("title", "", "http://example.com", "http://example.com/feed.xml", nil)
	if feed1 == nil || feed1.Id == 0 {
		t.Fatal("expected feed")
	}
	feed2 := db.GetFeed(feed1.Id)
	if feed2 == nil || !reflect.DeepEqual(feed1, feed2) {
		t.Fatal("invalid feed")
	}
}

func TestCreateFeedCleansTitleSuffix(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("Alice - Telegram Channel", "", "http://example.com", "http://example.com/feed.xml", nil)
	if feed.Title != "Alice" {
		t.Fatalf("got %q", feed.Title)
	}

	db.RenameFeed(feed.Id, "Alice 的 bilibili 动态")
	feed = db.GetFeed(feed.Id)
	if feed.Title != "Alice" {
		t.Fatalf("got %q", feed.Title)
	}
}

func TestCreateFeedSameLink(t *testing.T) {
	db := testDB()
	feed1 := db.CreateFeed("title", "", "", "http://example1.com/feed.xml", nil)
	if feed1 == nil || feed1.Id == 0 {
		t.Fatal("expected feed")
	}

	for i := 0; i < 10; i++ {
		db.CreateFeed("title", "", "", "http://example2.com/feed.xml", nil)
	}

	feed2 := db.CreateFeed("title", "", "http://example.com", "http://example1.com/feed.xml", nil)
	if feed1.Id != feed2.Id {
		t.Fatalf("expected the same feed.\nwant: %#v\nhave: %#v", feed1, feed2)
	}
}

func TestDeleteFeedDeletesItems(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("feed", "", "", "http://example.com/feed.xml", nil)
	other := db.CreateFeed("other", "", "", "http://example.com/other.xml", nil)
	db.CreateItems([]Item{
		{GUID: "feed-item", FeedId: feed.Id, Title: "feed item", Date: time.Now()},
		{GUID: "other-item", FeedId: other.Id, Title: "other item", Date: time.Now()},
	})

	if !db.DeleteFeed(feed.Id) {
		t.Fatal("failed to delete feed")
	}

	if count := db.CountItems(ItemFilter{FeedID: &feed.Id}); count != 0 {
		t.Fatalf("expected feed items to be deleted, got %d", count)
	}
	if count := db.CountItems(ItemFilter{FeedID: &other.Id}); count != 1 {
		t.Fatalf("expected other feed item to remain, got %d", count)
	}
}

func TestReadFeed(t *testing.T) {
	db := testDB()
	if db.GetFeed(100500) != nil {
		t.Fatal("cannot get nonexistent feed")
	}

	feed1 := db.CreateFeed("feed 1", "", "http://example1.com", "http://example1.com/feed.xml", nil)
	feed2 := db.CreateFeed("feed 2", "", "http://example2.com", "http://example2.com/feed.xml", nil)
	feeds := db.ListFeeds()
	if !reflect.DeepEqual(feeds, []Feed{*feed1, *feed2}) {
		t.Fatalf("invalid feed list: %#v", feeds)
	}
}

func TestUpdateFeed(t *testing.T) {
	db := testDB()
	feed1 := db.CreateFeedWithContentSelector("feed 1", "", "http://example1.com", "http://example1.com/feed.xml", ".article", nil)
	folder := db.CreateFolder("test")

	db.RenameFeed(feed1.Id, "newtitle")
	db.UpdateFeedFolder(feed1.Id, &folder.Id)
	db.UpdateFeedContentSelector(feed1.Id, ".content")
	db.UpdateFeedContentMode(feed1.Id, FeedContentModeEmbed)
	db.UpdateFeedIconURL(feed1.Id, "https://example.com/icon.png")

	feed2 := db.GetFeed(feed1.Id)
	if feed2.Title != "newtitle" {
		t.Error("invalid title")
	}
	if feed2.FolderId == nil || *feed2.FolderId != folder.Id {
		t.Error("invalid folder")
	}
	if feed2.ContentSelector != ".content" {
		t.Error("invalid content selector")
	}
	if feed2.ContentMode != FeedContentModeEmbed {
		t.Error("invalid content mode")
	}
	if feed2.IconURL != "https://example.com/icon.png" {
		t.Error("invalid icon url")
	}
	db.UpdateFeedRankingMode(feed1.Id, FeedRankingModeWithImage)
	feed2 = db.GetFeed(feed1.Id)
	if feed2.RankingMode != FeedRankingModeWithImage {
		t.Error("invalid ranking mode")
	}
	if db.UpdateFeedContentMode(feed1.Id, "invalid") {
		t.Error("invalid content mode accepted")
	}
	db.UpdateFeedIconURL(feed1.Id, "")
	feed2 = db.GetFeed(feed1.Id)
	if feed2.IconURL != "" {
		t.Error("invalid cleared icon url")
	}
	if !feed2.AutoReadScroll {
		t.Error("auto_read_scroll should default to true")
	}
	if !db.UpdateFeedAutoReadScroll(feed1.Id, false) {
		t.Fatal("failed to disable auto_read_scroll")
	}
	feed2 = db.GetFeed(feed1.Id)
	if feed2.AutoReadScroll {
		t.Error("auto_read_scroll should be disabled")
	}
}

func TestCreateFeedContentMode(t *testing.T) {
	db := testDB()

	feed := db.CreateFeed("feed", "", "http://example.com", "http://example.com/feed.xml", nil)
	if feed.ContentMode != FeedContentModeNormal {
		t.Fatalf("got %q", feed.ContentMode)
	}

	feed = db.CreateFeedWithContentMode("feed", "", "http://example.com", "http://example.com/feed.xml", "", FeedContentModeReadability, nil)
	if feed.ContentMode != FeedContentModeReadability {
		t.Fatalf("got %q", feed.ContentMode)
	}

	feed = db.CreateFeed("feed", "", "http://example.com", "http://example.com/feed.xml", nil)
	if feed.ContentMode != FeedContentModeReadability {
		t.Fatalf("expected existing content mode to be preserved, got %q", feed.ContentMode)
	}
}

func TestCreateFeedRankingMode(t *testing.T) {
	db := testDB()

	feed := db.CreateFeed("feed", "", "http://example.com", "http://example.com/feed.xml", nil)
	if feed.RankingMode != FeedRankingModeOff {
		t.Fatalf("expected ranking mode to default off, got %q", feed.RankingMode)
	}
	if feed.LastRankingItem != "" || feed.LastRankingMD5 != "" {
		t.Fatalf("expected empty last ranking state, got %q %q", feed.LastRankingItem, feed.LastRankingMD5)
	}

	feed = db.CreateFeedWithRankingMode("feed", "", "http://example.com", "http://example.com/feed.xml", "", FeedContentModeNormal, FeedRankingModeWithoutImage, nil)
	if feed.RankingMode != FeedRankingModeWithoutImage {
		t.Fatalf("expected ranking mode to be without_image, got %q", feed.RankingMode)
	}

	feed = db.CreateFeed("feed", "", "http://example.com", "http://example.com/feed.xml", nil)
	if feed.RankingMode != FeedRankingModeWithoutImage {
		t.Fatal("expected existing ranking mode to be preserved")
	}
	if feed.LastRankingItem != "" || feed.LastRankingMD5 != "" {
		t.Fatalf("expected empty last ranking state, got %q %q", feed.LastRankingItem, feed.LastRankingMD5)
	}
}

func TestCreateRankingModeItemUpdatesFeedState(t *testing.T) {
	db := testDB()
	feed := db.CreateFeedWithRankingMode("feed", "", "http://example.com", "http://example.com/feed.xml", "", FeedContentModeNormal, FeedRankingModeWithImage, nil)
	item := Item{
		GUID:    "ranking:2026070610",
		FeedId:  feed.Id,
		Title:   "ranking",
		Link:    "http://example.com",
		Date:    time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC),
		Content: "content",
	}

	if !db.CreateRankingModeItem(item, "md5") {
		t.Fatal("failed to create ranking item")
	}

	feed = db.GetFeed(feed.Id)
	if feed.LastRankingItem != item.GUID {
		t.Fatalf("last ranking item got %q", feed.LastRankingItem)
	}
	if feed.LastRankingMD5 != "md5" {
		t.Fatalf("last ranking md5 got %q", feed.LastRankingMD5)
	}
	stored := db.GetItemByGUID(feed.Id, item.GUID)
	if stored == nil || stored.Content != "content" {
		t.Fatalf("invalid ranking item: %#v", stored)
	}
}

func TestListRankingModeFeeds(t *testing.T) {
	db := testDB()
	feed1 := db.CreateFeedWithRankingMode("feed 1", "", "http://example1.com", "http://example1.com/feed.xml", "", "", FeedRankingModeWithImage, nil)
	db.CreateFeed("feed 2", "", "http://example2.com", "http://example2.com/feed.xml", nil)

	feeds := db.ListRankingModeFeeds()
	if !reflect.DeepEqual(feeds, []Feed{*feed1}) {
		t.Fatalf("invalid feed list: %#v", feeds)
	}
}

func TestListFeedsMissingIconURLs(t *testing.T) {
	db := testDB()
	feed1 := db.CreateFeed("feed 1", "", "http://example1.com", "http://example1.com/feed.xml", nil)
	feed2 := db.CreateFeed("feed 2", "", "http://example2.com", "http://example2.com/feed.xml", nil)
	db.UpdateFeedIconURL(feed2.Id, "https://example.com/icon.png")

	feeds := db.ListFeedsMissingIconURLs()
	if !reflect.DeepEqual(feeds, []Feed{*feed1}) {
		t.Fatalf("invalid feed list: %#v", feeds)
	}
}

func TestUpdateFeedMetadataPreservesSavedTitleAndLink(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("Saved Title", "", "https://example.com/saved", "https://example.com/feed.xml", nil)

	if !db.UpdateFeedMetadata(feed.Id, "Fresh Title", "https://example.com/fresh", "https://example.com/new-feed.xml") {
		t.Fatal("failed to update metadata")
	}

	feed = db.GetFeed(feed.Id)
	if feed.Title != "Saved Title" {
		t.Fatalf("title got %q", feed.Title)
	}
	if feed.Link != "https://example.com/saved" {
		t.Fatalf("link got %q", feed.Link)
	}
	if feed.FeedLink != "https://example.com/new-feed.xml" {
		t.Fatalf("feed_link got %q", feed.FeedLink)
	}
}

func TestUpdateFeedMetadataFillsPlaceholderTitleAndLink(t *testing.T) {
	tests := []struct {
		name      string
		oldTitle  string
		oldLink   string
		wantTitle string
		wantLink  string
	}{
		{
			name:      "empty",
			oldTitle:  "",
			oldLink:   "",
			wantTitle: "Fresh Title",
			wantLink:  "https://example.com/fresh",
		},
		{
			name:      "whitespace",
			oldTitle:  "  ",
			oldLink:   "  ",
			wantTitle: "Fresh Title",
			wantLink:  "https://example.com/fresh",
		},
		{
			name:      "rsshub placeholders",
			oldTitle:  "rsshub://telegram/channel/test",
			oldLink:   "rsshub://telegram/channel/test",
			wantTitle: "Fresh Title",
			wantLink:  "https://example.com/fresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testDB()
			feed := db.CreateFeed("Initial Title", "", tt.oldLink, "https://example.com/feed.xml", nil)
			db.RenameFeed(feed.Id, tt.oldTitle)

			if !db.UpdateFeedMetadata(feed.Id, "Fresh Title - Telegram Channel", "https://example.com/fresh", "") {
				t.Fatal("failed to update metadata")
			}

			feed = db.GetFeed(feed.Id)
			if feed.Title != tt.wantTitle {
				t.Fatalf("title got %q", feed.Title)
			}
			if feed.Link != tt.wantLink {
				t.Fatalf("link got %q", feed.Link)
			}
		})
	}
}

func TestUpdateFeedMetadataKeepsPlaceholderWhenFreshMetadataEmpty(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("rsshub://telegram/channel/test", "", "rsshub://telegram/channel/test", "https://example.com/feed.xml", nil)

	if !db.UpdateFeedMetadata(feed.Id, "", "", "") {
		t.Fatal("failed to update metadata")
	}

	feed = db.GetFeed(feed.Id)
	if feed.Title != "rsshub://telegram/channel/test" {
		t.Fatalf("title got %q", feed.Title)
	}
	if feed.Link != "rsshub://telegram/channel/test" {
		t.Fatalf("link got %q", feed.Link)
	}
}

func TestDeleteFeed(t *testing.T) {
	db := testDB()
	feed1 := db.CreateFeed("title", "", "http://example.com", "http://example.com/feed.xml", nil)

	if db.DeleteFeed(100500) {
		t.Error("cannot delete what does not exist")
	}

	if !db.DeleteFeed(feed1.Id) {
		t.Fatal("did not delete existing feed")
	}
	if db.GetFeed(feed1.Id) != nil {
		t.Fatal("feed still exists")
	}
}
