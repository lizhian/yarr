package storage

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

/*
- folder1
  - feed11
	- item111 (unread)
	- item112 (read)
	- item113 (favorite)
  - feed12
	- item121 (unread)
	- item122 (read)
- folder2
  - feed21
    - item211 (read)
	- item212 (favorite)
- feed01
  - item011 (unread)
  - item012 (read)
  - item013 (favorite)
*/

type testItemScope struct {
	feed11, feed12   *Feed
	feed21, feed01   *Feed
	folder1, folder2 *Folder
}

func testItemsSetup(db *Storage) testItemScope {
	folder1 := db.CreateFolder("folder1")
	folder2 := db.CreateFolder("folder2")

	feed11 := db.CreateFeed("feed11", "", "", "http://test.com/feed11.xml", &folder1.Id)
	feed12 := db.CreateFeed("feed12", "", "", "http://test.com/feed12.xml", &folder1.Id)
	feed21 := db.CreateFeed("feed21", "", "", "http://test.com/feed21.xml", &folder2.Id)
	feed01 := db.CreateFeed("feed01", "", "", "http://test.com/feed01.xml", nil)

	now := time.Now()
	db.CreateItems([]Item{
		// feed11
		{GUID: "item111", FeedId: feed11.Id, Title: "title111", Date: now.Add(time.Hour * 24 * 1)},
		{GUID: "item112", FeedId: feed11.Id, Title: "title112", Date: now.Add(time.Hour * 24 * 2)}, // read
		{GUID: "item113", FeedId: feed11.Id, Title: "title113", Date: now.Add(time.Hour * 24 * 3)}, // favorite
		// feed12
		{GUID: "item121", FeedId: feed12.Id, Title: "title121", Date: now.Add(time.Hour * 24 * 4)},
		{GUID: "item122", FeedId: feed12.Id, Title: "title122", Date: now.Add(time.Hour * 24 * 5)}, // read
		// feed21
		{GUID: "item211", FeedId: feed21.Id, Title: "title211", Date: now.Add(time.Hour * 24 * 6)}, // read
		{GUID: "item212", FeedId: feed21.Id, Title: "title212", Date: now.Add(time.Hour * 24 * 7)}, // favorite
		// feed01
		{GUID: "item011", FeedId: feed01.Id, Title: "title011", Date: now.Add(time.Hour * 24 * 8)},
		{GUID: "item012", FeedId: feed01.Id, Title: "title012", Date: now.Add(time.Hour * 24 * 9)},  // read
		{GUID: "item013", FeedId: feed01.Id, Title: "title013", Date: now.Add(time.Hour * 24 * 10)}, // favorite
	})
	db.db.Exec(`update items set status = ? where guid in ("item112", "item113", "item122", "item211", "item212", "item012", "item013")`, READ)
	db.db.Exec(`update items set favorite = true where guid in ("item113", "item212", "item013")`)

	return testItemScope{
		feed11:  feed11,
		feed12:  feed12,
		feed21:  feed21,
		feed01:  feed01,
		folder1: folder1,
		folder2: folder2,
	}
}

func getItem(db *Storage, guid string) *Item {
	i := &Item{}
	err := db.db.QueryRow(`
		select
			i.id, i.guid, i.feed_id, i.title, i.link, i.content,
			i.date, i.status, i.favorite, i.media_links
		from items i
		where i.guid = ?
	`, guid).Scan(
		&i.Id, &i.GUID, &i.FeedId, &i.Title, &i.Link, &i.Content,
		&i.Date, &i.Status, &i.Favorite, &i.MediaLinks,
	)
	if err != nil {
		log.Fatal(err)
	}
	return i
}

func getItemGuids(items []Item) []string {
	guids := make([]string, 0)
	for _, item := range items {
		guids = append(guids, item.GUID)
	}
	return guids
}

func TestItemGUIDExists(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("feed", "", "", "http://test.com/feed.xml", nil)
	if db.ItemGUIDExists(feed.Id, "guid") {
		t.Fatal("unexpected existing item")
	}

	db.CreateItems([]Item{{
		GUID:   "guid",
		FeedId: feed.Id,
		Title:  "title",
		Link:   "http://test.com/item",
		Date:   time.Now(),
	}})
	if !db.ItemGUIDExists(feed.Id, "guid") {
		t.Fatal("expected existing item")
	}
	if db.ItemGUIDExists(feed.Id+1, "guid") {
		t.Fatal("expected feed scoped lookup")
	}
}

func TestListItems(t *testing.T) {
	db := testDB()
	scope := testItemsSetup(db)

	// filter by folder_id

	have := getItemGuids(db.ListItems(ItemFilter{FolderID: &scope.folder1.Id}, 10, false, false))
	want := []string{"item111", "item112", "item113", "item121", "item122"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	have = getItemGuids(db.ListItems(ItemFilter{FolderID: &scope.folder2.Id}, 10, false, false))
	want = []string{"item211", "item212"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	// filter by feed_id

	have = getItemGuids(db.ListItems(ItemFilter{FeedID: &scope.feed11.Id}, 10, false, false))
	want = []string{"item111", "item112", "item113"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	have = getItemGuids(db.ListItems(ItemFilter{FeedID: &scope.feed01.Id}, 10, false, false))
	want = []string{"item011", "item012", "item013"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	// filter by favorite and status

	favorite := true
	have = getItemGuids(db.ListItems(ItemFilter{Favorite: &favorite}, 10, false, false))
	want = []string{"item113", "item212", "item013"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	var unread ItemStatus = UNREAD
	have = getItemGuids(db.ListItems(ItemFilter{Status: &unread}, 10, false, false))
	want = []string{"item111", "item121", "item011"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	// limit

	have = getItemGuids(db.ListItems(ItemFilter{}, 2, false, false))
	want = []string{"item111", "item112"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	// filter by search
	db.SyncSearch()
	search1 := "title111"
	have = getItemGuids(db.ListItems(ItemFilter{Search: &search1}, 4, true, false))
	want = []string{"item111"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	// sort by date
	have = getItemGuids(db.ListItems(ItemFilter{}, 4, true, false))
	want = []string{"item013", "item012", "item011", "item212"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}
}

func TestCreateItemsBackfillsMediaLinks(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("feed", "", "", "http://test.com/feed.xml", nil)
	now := time.Now()

	db.CreateItems([]Item{{
		GUID:   "item",
		FeedId: feed.Id,
		Title:  "old title",
		Date:   now,
	}})
	db.UpdateItemStatus(getItem(db, "item").Id, READ)

	db.CreateItems([]Item{{
		GUID:   "item",
		FeedId: feed.Id,
		Title:  "new title",
		Date:   now,
		MediaLinks: MediaLinks{{
			URL:  "https://example.com/image.webp",
			Type: "image",
		}},
	}})

	item := getItem(db, "item")
	wantMediaLinks := MediaLinks{{URL: "https://example.com/image.webp", Type: "image"}}
	if !reflect.DeepEqual(wantMediaLinks, item.MediaLinks) {
		t.Fatalf("media links were not backfilled\nwant: %#v\nhave: %#v", wantMediaLinks, item.MediaLinks)
	}
	if item.Status != READ {
		t.Fatalf("item status should be preserved\nwant: %#v\nhave: %#v", READ, item.Status)
	}
	if item.Title != "old title" {
		t.Fatalf("existing item content should not be overwritten\nwant: %#v\nhave: %#v", "old title", item.Title)
	}
}

func TestListItemsPaginated(t *testing.T) {
	db := testDB()
	testItemsSetup(db)

	item012 := getItem(db, "item012")
	item121 := getItem(db, "item121")

	// all, newest first
	have := getItemGuids(db.ListItems(ItemFilter{After: &item012.Id}, 3, true, false))
	want := []string{"item011", "item212", "item211"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	// unread, newest first
	unread := UNREAD
	have = getItemGuids(db.ListItems(ItemFilter{After: &item012.Id, Status: &unread}, 3, true, false))
	want = []string{"item011", "item121", "item111"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	// favorite, oldest first
	favorite := true
	have = getItemGuids(db.ListItems(ItemFilter{After: &item121.Id, Favorite: &favorite}, 3, false, false))
	want = []string{"item212", "item013"}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}
}

func TestListItemsOrdered(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("feed", "", "", "http://test.com/feed.xml", nil)
	base := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	db.CreateItems([]Item{
		{GUID: "unread-old", FeedId: feed.Id, Date: base.Add(time.Hour)},
		{GUID: "read-old", FeedId: feed.Id, Date: base.Add(2 * time.Hour)},
		{GUID: "unread-new", FeedId: feed.Id, Date: base.Add(3 * time.Hour)},
		{GUID: "read-new", FeedId: feed.Id, Date: base.Add(4 * time.Hour)},
	})
	db.db.Exec(`update items set status = ? where guid like 'read-%'`, READ)
	db.db.Exec(`update items set favorite = true where guid in ('unread-new', 'read-old')`)
	unread := UNREAD
	read := READ

	tests := []struct {
		name        string
		filter      ItemFilter
		unreadFirst bool
		newestFirst bool
		want        []string
	}{
		{"unread newest read newest", ItemFilter{}, true, true, []string{"unread-new", "unread-old", "read-new", "read-old"}},
		{"unread oldest read newest", ItemFilter{}, true, false, []string{"unread-old", "unread-new", "read-new", "read-old"}},
		{"mixed oldest", ItemFilter{}, false, false, []string{"unread-old", "read-old", "unread-new", "read-new"}},
		{"favorites", ItemFilter{Favorite: boolPtr(true)}, true, false, []string{"unread-new", "read-old"}},
		{"unread filter oldest", ItemFilter{Status: &unread}, true, false, []string{"unread-old", "unread-new"}},
		{"read filter remains newest", ItemFilter{Status: &read}, true, false, []string{"read-new", "read-old"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			have := getItemGuids(db.ListItemsOrdered(test.filter, 10, test.unreadFirst, test.newestFirst, false))
			if !reflect.DeepEqual(have, test.want) {
				t.Fatalf("want %#v, have %#v", test.want, have)
			}
		})
	}
}

func TestListItemsOrderedUsesIDAsDateTieBreaker(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("feed", "", "", "http://test.com/feed.xml", nil)
	date := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	db.CreateItems([]Item{
		{GUID: "a", FeedId: feed.Id, Date: date},
		{GUID: "b", FeedId: feed.Id, Date: date},
	})

	have := getItemGuids(db.ListItemsOrdered(ItemFilter{}, 10, true, true, false))
	if want := []string{"b", "a"}; !reflect.DeepEqual(have, want) {
		t.Fatalf("newest order want %#v, have %#v", want, have)
	}
	have = getItemGuids(db.ListItemsOrdered(ItemFilter{}, 10, false, false, false))
	if want := []string{"a", "b"}; !reflect.DeepEqual(have, want) {
		t.Fatalf("oldest order want %#v, have %#v", want, have)
	}
}

func TestItemReadAndFavoriteAreIndependent(t *testing.T) {
	db := testDB()
	feed := db.CreateFeed("feed", "", "", "http://test.com/feed.xml", nil)
	db.CreateItems([]Item{
		{GUID: "unread", FeedId: feed.Id},
		{GUID: "unread-favorite", FeedId: feed.Id},
		{GUID: "read", FeedId: feed.Id},
		{GUID: "read-favorite", FeedId: feed.Id},
	})
	db.db.Exec(`update items set favorite = true where guid in ('unread-favorite', 'read-favorite')`)
	db.db.Exec(`update items set status = ? where guid in ('read', 'read-favorite')`, READ)

	if !db.UpdateItemFavorite(getItem(db, "unread").Id, true) || getItem(db, "unread").Status != UNREAD {
		t.Fatal("favoriting changed read status")
	}
	if !db.UpdateItemStatus(getItem(db, "unread-favorite").Id, READ) || !getItem(db, "unread-favorite").Favorite {
		t.Fatal("marking read changed favorite status")
	}
	if db.UpdateItemStatus(getItem(db, "read").Id, ItemStatus(2)) {
		t.Fatal("accepted unsupported item status")
	}

	db.MarkItemsRead(MarkFilter{})
	favorites := db.ListItems(ItemFilter{Favorite: boolPtr(true)}, 10, false, false)
	if len(favorites) != 3 {
		t.Fatalf("want 3 favorites after mark-all-read, have %d", len(favorites))
	}
	for _, item := range favorites {
		if item.Status != READ {
			t.Fatalf("favorite %q was not marked read", item.GUID)
		}
	}
	stats := db.FeedStats()
	if len(stats) != 1 || stats[0].UnreadCount != 0 || stats[0].FavoriteCount != 3 {
		t.Fatalf("unexpected stats %#v", stats)
	}
}

func TestItemListQueryPlansUseScopeIndexes(t *testing.T) {
	db := testDB()
	folder := db.CreateFolder("folder")
	feed := db.CreateFeed("feed", "", "", "http://test.com/feed.xml", &folder.Id)
	unread := UNREAD
	favorite := true

	tests := []struct {
		name    string
		filter  ItemFilter
		indexes []string
	}{
		{"all", ItemFilter{Status: &unread}, []string{"idx_item_status_date_id"}},
		{"folder", ItemFilter{FolderID: &folder.Id, Status: &unread}, []string{"idx_item_status_date_id", "idx_feed_folder_id"}},
		{"feed", ItemFilter{FeedID: &feed.Id, Status: &unread}, []string{"idx_item_feed_status_date_id"}},
		{"favorite", ItemFilter{Favorite: &favorite, Status: &unread}, []string{"idx_item_favorite_status_date_id"}},
		{"feed favorite", ItemFilter{FeedID: &feed.Id, Favorite: &favorite, Status: &unread}, []string{"idx_item_feed_favorite_status_date_id"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			predicate, args := listQueryPredicate(test.filter, true)
			query := fmt.Sprintf(`explain query plan select i.id from items i where %s order by i.date desc, i.id desc limit 31`, predicate)
			rows, err := db.db.Query(query, args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var details []string
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				details = append(details, detail)
			}
			plan := strings.Join(details, "\n")
			for _, index := range test.indexes {
				if !strings.Contains(plan, index) {
					t.Fatalf("query plan did not use %s:\n%s", index, plan)
				}
			}
		})
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func TestMarkItemsRead(t *testing.T) {
	var read ItemStatus = READ

	db1 := testDB()
	testItemsSetup(db1)
	db1.MarkItemsRead(MarkFilter{})
	have := getItemGuids(db1.ListItems(ItemFilter{Status: &read}, 10, false, false))
	want := []string{
		"item111", "item112", "item113", "item121", "item122",
		"item211", "item212", "item011", "item012", "item013",
	}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	db2 := testDB()
	scope2 := testItemsSetup(db2)
	db2.MarkItemsRead(MarkFilter{FolderID: &scope2.folder1.Id})
	have = getItemGuids(db2.ListItems(ItemFilter{Status: &read}, 10, false, false))
	want = []string{
		"item111", "item112", "item113", "item121", "item122",
		"item211", "item212", "item012", "item013",
	}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	db3 := testDB()
	scope3 := testItemsSetup(db3)
	db3.MarkItemsRead(MarkFilter{FeedID: &scope3.feed11.Id})
	have = getItemGuids(db3.ListItems(ItemFilter{Status: &read}, 10, false, false))
	want = []string{
		"item111", "item112", "item113", "item122",
		"item211", "item212", "item012", "item013",
	}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}

	db4 := testDB()
	testItemsSetup(db4)
	db4.SyncSearch()
	search := "title111"
	db4.MarkItemsRead(MarkFilter{Search: &search})
	have = getItemGuids(db4.ListItems(ItemFilter{Status: &read}, 10, false, false))
	want = []string{
		"item111", "item112", "item113", "item122",
		"item211", "item212", "item012", "item013",
	}
	if !reflect.DeepEqual(have, want) {
		t.Logf("want: %#v", want)
		t.Logf("have: %#v", have)
		t.Fail()
	}
}

func TestDeleteOldItems(t *testing.T) {
	extraItems := 10

	now := time.Now().UTC()
	db := testDB()
	feed := db.CreateFeed("feed", "", "", "http://test.com/feed11.xml", nil)

	items := make([]Item, 0)
	for i := 0; i < readItemsKeepSize+extraItems; i++ {
		istr := strconv.Itoa(i)
		items = append(items, Item{
			GUID:   istr,
			FeedId: feed.Id,
			Title:  istr,
			Date:   now.Add(time.Hour * time.Duration(i)),
		})
	}
	items = append(items,
		Item{GUID: "unread", FeedId: feed.Id, Title: "unread", Date: now.Add(time.Hour * time.Duration(readItemsKeepSize+extraItems+1))},
		Item{GUID: "favorite", FeedId: feed.Id, Title: "favorite", Date: now.Add(time.Hour * time.Duration(readItemsKeepSize+extraItems+2))},
	)
	db.CreateItems(items)
	_, err := db.db.Exec(`update items set status = ?, favorite = true where guid = 'favorite'`, READ)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.db.Exec(`update items set status = ? where guid != 'unread' and guid != 'favorite'`, READ)
	if err != nil {
		t.Fatal(err)
	}

	db.DeleteOldItems()
	feedItems := db.ListItems(ItemFilter{FeedID: &feed.Id}, 1000, false, false)
	if len(feedItems) != readItemsKeepSize+2 {
		t.Fatalf(
			"invalid number of old items kept\nwant: %d\nhave: %d",
			readItemsKeepSize+2,
			len(feedItems),
		)
	}
	if getItem(db, "unread").Status != UNREAD {
		t.Fatal("unread item was deleted")
	}
	if !getItem(db, "favorite").Favorite {
		t.Fatal("favorite item was deleted")
	}
	var oldReadCount int
	err = db.db.QueryRow(`select count(*) from items where guid = "0"`).Scan(&oldReadCount)
	if err != nil {
		t.Fatal(err)
	}
	if oldReadCount != 0 {
		t.Fatal("oldest read item was kept")
	}
}
