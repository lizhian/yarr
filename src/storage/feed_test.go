package storage

import (
	"reflect"
	"testing"
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
	icon := []byte("icon")

	db.RenameFeed(feed1.Id, "newtitle")
	db.UpdateFeedFolder(feed1.Id, &folder.Id)
	db.UpdateFeedContentSelector(feed1.Id, ".content")
	db.UpdateFeedIcon(feed1.Id, &icon)

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
	if !feed2.HasIcon || string(*feed2.Icon) != "icon" {
		t.Error("invalid icon")
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
