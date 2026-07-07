package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/storage"
)

func greaderLogin(t *testing.T, handler http.Handler, username, password string) string {
	t.Helper()
	form := url.Values{}
	form.Set("Email", username)
	form.Set("Passwd", password)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/greader.php/accounts/ClientLogin", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatalf("got login status %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	for _, line := range strings.Split(body, "\n") {
		if token, ok := strings.CutPrefix(line, "Auth="); ok {
			return token
		}
	}
	t.Fatalf("missing auth token in %q", body)
	return ""
}

func greaderRequest(method, path, token string, body *strings.Reader) *http.Request {
	if body == nil {
		body = strings.NewReader("")
	}
	request := httptest.NewRequest(method, path, body)
	if token != "" {
		request.Header.Set("Authorization", "GoogleLogin auth="+token)
	}
	if method == "POST" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return request
}

func seedGReaderData(t *testing.T, db *storage.Storage) (storage.Feed, storage.Feed, storage.Item, storage.Item, storage.Item) {
	t.Helper()
	folder := db.CreateFolder("Tech")
	if folder == nil {
		t.Fatal("expected folder")
	}
	folderID := folder.Id
	feed1 := db.CreateFeed("Feed A", "", "https://example.com/a", "https://example.com/a.xml", &folderID)
	feed2 := db.CreateFeed("Feed B", "", "https://example.com/b", "https://example.com/b.xml", nil)
	items := []storage.Item{
		{GUID: "a1", FeedId: feed1.Id, Title: "Unread A", Link: "https://example.com/a/1", Content: "unread", Date: time.Unix(300, 0).UTC()},
		{GUID: "a2", FeedId: feed1.Id, Title: "Read A", Link: "https://example.com/a/2", Content: "read", Date: time.Unix(200, 0).UTC()},
		{GUID: "b1", FeedId: feed2.Id, Title: "Star B", Link: "https://example.com/b/1", Content: "star", Date: time.Unix(100, 0).UTC()},
	}
	if !db.CreateItems(items) {
		t.Fatal("failed to create items")
	}
	stored := db.ListItems(storage.ItemFilter{}, 10, true, true)
	if len(stored) != 3 {
		t.Fatalf("got %d items", len(stored))
	}
	var unreadA, readA, starB storage.Item
	for _, item := range stored {
		switch item.GUID {
		case "a1":
			unreadA = item
		case "a2":
			readA = item
			db.UpdateItemStatus(item.Id, storage.READ)
		case "b1":
			starB = item
			db.UpdateItemStatus(item.Id, storage.STARRED)
		}
	}
	readA.Status = storage.READ
	starB.Status = storage.STARRED
	return *feed1, *feed2, unreadA, readA, starB
}

func TestGReaderClientLoginUsesAuthConfig(t *testing.T) {
	db := testServerDB(t)
	if !db.SetAuthConfig(true, "username", "password") {
		t.Fatal("did not enable auth")
	}
	handler := NewServer(db, "127.0.0.1:8000").handler()

	form := url.Values{"Email": {"username"}, "Passwd": {"wrong"}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/greader.php/accounts/ClientLogin", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != http.StatusForbidden {
		t.Fatal("got", recorder.Result().StatusCode)
	}

	token := greaderLogin(t, handler, "username", "password")
	recorder = httptest.NewRecorder()
	request = greaderRequest("GET", "/api/greader.php/reader/api/0/user-info", token, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
}

func TestGReaderClientLoginOpenWhenAuthDisabled(t *testing.T) {
	db := testServerDB(t)
	handler := NewServer(db, "127.0.0.1:8000").handler()
	token := greaderLogin(t, handler, "anyone", "anything")
	if token == "" {
		t.Fatal("expected token")
	}
}

func TestGReaderRejectsMissingOrInvalidToken(t *testing.T) {
	db := testServerDB(t)
	if !db.SetAuthConfig(true, "username", "password") {
		t.Fatal("did not enable auth")
	}
	handler := NewServer(db, "127.0.0.1:8000").handler()

	for _, token := range []string{"", "bad"} {
		recorder := httptest.NewRecorder()
		request := greaderRequest("GET", "/api/greader.php/reader/api/0/subscription/list", token, nil)
		handler.ServeHTTP(recorder, request)
		if recorder.Result().StatusCode != http.StatusUnauthorized {
			t.Fatalf("token %q got %d", token, recorder.Result().StatusCode)
		}
	}
}

func TestGReaderSubscriptionTagAndUnreadCount(t *testing.T) {
	db := testServerDB(t)
	feed1, _, _, _, _ := seedGReaderData(t, db)
	handler := NewServer(db, "127.0.0.1:8000").handler()
	token := greaderLogin(t, handler, "user", "pass")

	recorder := httptest.NewRecorder()
	request := greaderRequest("GET", "/api/greader.php/reader/api/0/subscription/list", token, nil)
	handler.ServeHTTP(recorder, request)
	var subscriptions struct {
		Subscriptions []greaderSubscription `json:"subscriptions"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&subscriptions); err != nil {
		t.Fatal(err)
	}
	if len(subscriptions.Subscriptions) != 2 {
		t.Fatalf("got %d subscriptions", len(subscriptions.Subscriptions))
	}
	var foundFeed bool
	for _, subscription := range subscriptions.Subscriptions {
		if subscription.ID == greaderFeedID(feed1.Id) && len(subscription.Categories) == 1 && subscription.Categories[0].Label == "Tech" {
			foundFeed = true
		}
	}
	if !foundFeed {
		t.Fatalf("missing categorized feed: %#v", subscriptions.Subscriptions)
	}

	recorder = httptest.NewRecorder()
	request = greaderRequest("GET", "/api/greader.php/reader/api/0/tag/list", token, nil)
	handler.ServeHTTP(recorder, request)
	var tags struct {
		Tags []greaderCategory `json:"tags"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&tags); err != nil {
		t.Fatal(err)
	}
	if !hasCategory(tags.Tags, greaderLabelID("Tech")) {
		t.Fatalf("missing Tech tag: %#v", tags.Tags)
	}

	recorder = httptest.NewRecorder()
	request = greaderRequest("GET", "/api/greader.php/reader/api/0/unread-count", token, nil)
	handler.ServeHTTP(recorder, request)
	var counts struct {
		UnreadCounts []struct {
			ID    string `json:"id"`
			Count int64  `json:"count"`
		} `json:"unreadcounts"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&counts); err != nil {
		t.Fatal(err)
	}
	if countFor(counts.UnreadCounts, greaderReadingList) != 1 {
		t.Fatalf("got counts %#v", counts.UnreadCounts)
	}
	if countFor(counts.UnreadCounts, greaderLabelID("Tech")) != 1 {
		t.Fatalf("got counts %#v", counts.UnreadCounts)
	}
}

func TestGReaderStreamContentsFiltersAndContinuation(t *testing.T) {
	db := testServerDB(t)
	feed1, _, unreadA, _, starB := seedGReaderData(t, db)
	handler := NewServer(db, "127.0.0.1:8000").handler()
	token := greaderLogin(t, handler, "user", "pass")

	recorder := httptest.NewRecorder()
	request := greaderRequest("GET", "/api/greader.php/reader/api/0/stream/contents/"+greaderReadingList+"?n=1", token, nil)
	handler.ServeHTTP(recorder, request)
	var firstPage struct {
		Items        []greaderItem `json:"items"`
		Continuation string        `json:"continuation"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&firstPage); err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Items) != 1 || firstPage.Items[0].ID != greaderItemID(unreadA.Id) {
		t.Fatalf("got first page %#v", firstPage)
	}
	if firstPage.Continuation == "" {
		t.Fatal("expected continuation")
	}

	recorder = httptest.NewRecorder()
	request = greaderRequest("GET", "/api/greader.php/reader/api/0/stream/contents?s="+url.QueryEscape(greaderFeedID(feed1.Id))+"&n=1", token, nil)
	handler.ServeHTTP(recorder, request)
	var queryStream struct {
		Items []greaderItem `json:"items"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&queryStream); err != nil {
		t.Fatal(err)
	}
	if len(queryStream.Items) != 1 || queryStream.Items[0].ID != greaderItemID(unreadA.Id) {
		t.Fatalf("got query stream %#v", queryStream.Items)
	}

	recorder = httptest.NewRecorder()
	url := "/api/greader.php/reader/api/0/stream/contents/" + greaderFeedID(feed1.Id) + "?xt=" + url.QueryEscape(greaderRead)
	request = greaderRequest("GET", url, token, nil)
	handler.ServeHTTP(recorder, request)
	var unreadFeed struct {
		Items []greaderItem `json:"items"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&unreadFeed); err != nil {
		t.Fatal(err)
	}
	if len(unreadFeed.Items) != 1 || unreadFeed.Items[0].ID != greaderItemID(unreadA.Id) {
		t.Fatalf("got unread feed %#v", unreadFeed.Items)
	}

	recorder = httptest.NewRecorder()
	request = greaderRequest("GET", "/api/greader.php/reader/api/0/stream/contents/"+greaderStarred, token, nil)
	handler.ServeHTTP(recorder, request)
	var starred struct {
		Items []greaderItem `json:"items"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&starred); err != nil {
		t.Fatal(err)
	}
	if len(starred.Items) != 1 || starred.Items[0].ID != greaderItemID(starB.Id) {
		t.Fatalf("got starred %#v", starred.Items)
	}
}

func TestGReaderItemIDsAndContentsRoundTrip(t *testing.T) {
	db := testServerDB(t)
	_, _, unreadA, _, _ := seedGReaderData(t, db)
	handler := NewServer(db, "127.0.0.1:8000").handler()
	token := greaderLogin(t, handler, "user", "pass")

	recorder := httptest.NewRecorder()
	request := greaderRequest("GET", "/api/greader.php/reader/api/0/stream/items/ids?s="+url.QueryEscape(greaderReadingList)+"&n=1", token, nil)
	handler.ServeHTTP(recorder, request)
	var ids struct {
		ItemRefs []map[string]string `json:"itemRefs"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&ids); err != nil {
		t.Fatal(err)
	}
	if len(ids.ItemRefs) != 1 || ids.ItemRefs[0]["id"] != greaderItemID(unreadA.Id) {
		t.Fatalf("got ids %#v", ids.ItemRefs)
	}

	form := url.Values{"i": {ids.ItemRefs[0]["id"]}}
	recorder = httptest.NewRecorder()
	request = greaderRequest("POST", "/api/greader.php/reader/api/0/stream/items/contents", token, strings.NewReader(form.Encode()))
	handler.ServeHTTP(recorder, request)
	var contents struct {
		Items []greaderItem `json:"items"`
	}
	if err := json.NewDecoder(recorder.Result().Body).Decode(&contents); err != nil {
		t.Fatal(err)
	}
	if len(contents.Items) != 1 || contents.Items[0].ID != greaderItemID(unreadA.Id) {
		t.Fatalf("got contents %#v", contents.Items)
	}
}

func TestGReaderEditTagAndModifyUpdatesStatus(t *testing.T) {
	db := testServerDB(t)
	feed1, _, unreadA, readA, starB := seedGReaderData(t, db)
	handler := NewServer(db, "127.0.0.1:8000").handler()
	token := greaderLogin(t, handler, "user", "pass")

	tokenRecorder := httptest.NewRecorder()
	request := greaderRequest("GET", "/api/greader.php/reader/api/0/token", token, nil)
	handler.ServeHTTP(tokenRecorder, request)
	writeToken := strings.TrimSpace(tokenRecorder.Body.String())

	form := url.Values{"i": {greaderItemID(unreadA.Id)}, "a": {greaderRead}, "T": {writeToken}}
	recorder := httptest.NewRecorder()
	request = greaderRequest("POST", "/api/greader.php/reader/api/0/edit-tag", token, strings.NewReader(form.Encode()))
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	if item := db.GetItem(unreadA.Id); item.Status != storage.READ {
		t.Fatalf("got %v", item.Status)
	}

	form = url.Values{"i": {greaderItemID(readA.Id)}, "a": {greaderStarred}, "T": {writeToken}}
	recorder = httptest.NewRecorder()
	request = greaderRequest("POST", "/api/greader.php/reader/api/0/stream/items/modify", token, strings.NewReader(form.Encode()))
	handler.ServeHTTP(recorder, request)
	if item := db.GetItem(readA.Id); item.Status != storage.STARRED {
		t.Fatalf("got %v", item.Status)
	}

	form = url.Values{"i": {greaderItemID(starB.Id)}, "r": {greaderStarred}, "T": {writeToken}}
	recorder = httptest.NewRecorder()
	request = greaderRequest("POST", "/api/greader.php/reader/api/0/edit-tag", token, strings.NewReader(form.Encode()))
	handler.ServeHTTP(recorder, request)
	if item := db.GetItem(starB.Id); item.Status != storage.READ {
		t.Fatalf("got %v", item.Status)
	}

	form = url.Values{"s": {greaderFeedID(feed1.Id)}, "a": {greaderRead}, "T": {writeToken}}
	recorder = httptest.NewRecorder()
	request = greaderRequest("POST", "/api/greader.php/reader/api/0/edit-tag", token, strings.NewReader(form.Encode()))
	handler.ServeHTTP(recorder, request)
	if count := db.CountItems(storage.ItemFilter{Status: statusPtr(storage.UNREAD)}); count != 0 {
		t.Fatalf("got %d unread items", count)
	}
}

func hasCategory(categories []greaderCategory, id string) bool {
	for _, category := range categories {
		if category.ID == id {
			return true
		}
	}
	return false
}

func countFor(counts []struct {
	ID    string `json:"id"`
	Count int64  `json:"count"`
}, id string) int64 {
	for _, count := range counts {
		if count.ID == id {
			return count.Count
		}
	}
	return 0
}

func statusPtr(status storage.ItemStatus) *storage.ItemStatus {
	return &status
}

func TestGReaderRouteProtectedByOwnAuthWhenWebAuthEnabled(t *testing.T) {
	db := testServerDB(t)
	if !db.SetAuthConfig(true, "username", "password") {
		t.Fatal("did not enable auth")
	}
	handler := NewServer(db, "127.0.0.1:8000").handler()

	recorder := httptest.NewRecorder()
	form := url.Values{"Email": {"username"}, "Passwd": {"password"}}
	request := httptest.NewRequest("POST", "/api/greader.php/accounts/ClientLogin", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(recorder, request)
	if recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("got", recorder.Result().StatusCode)
	}
	if !strings.Contains(recorder.Body.String(), "Auth=") {
		t.Fatalf("got body %q", recorder.Body.String())
	}
}
