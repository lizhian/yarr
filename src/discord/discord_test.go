package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/content/sanitizer"
	"github.com/nkanaev/yarr/src/parser"
	"github.com/nkanaev/yarr/src/storage"
)

type fakeClock struct {
	mu    sync.Mutex
	t     time.Time
	waits []time.Duration
}

func (f *fakeClock) now() time.Time  { f.mu.Lock(); defer f.mu.Unlock(); return f.t }
func (f *fakeClock) set(t time.Time) { f.mu.Lock(); defer f.mu.Unlock(); f.t = t }
func (f *fakeClock) sleep(ctx context.Context, d time.Duration) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if d > 0 {
		f.waits = append(f.waits, d)
		f.t = f.t.Add(d)
	}
	return nil
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func setup(t *testing.T, h http.HandlerFunc) (*Service, *fakeClock, *storage.Storage) {
	t.Helper()
	db, err := storage.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	clock := &fakeClock{t: time.Date(2026, 9, 30, 12, 0, 0, 0, shanghai)}
	s.client.now = clock.now
	s.client.sleep = clock.sleep
	s.client.jitter = func(a, b time.Duration) time.Duration { return a }
	s.client.gate = newScheduler()
	s.client.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "discord.com" || r.Header.Get("Authorization") == "" || !strings.Contains(r.UserAgent(), "Mozilla/") {
			t.Errorf("unexpected request host/headers")
		}
		w := httptest.NewRecorder()
		h(w, r)
		return w.Result(), nil
	})
	return s, clock, db
}
func normal(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/messages") {
		io.WriteString(w, `[{"id":"20","content":"Hello <script>\nsecond line","timestamp":"2026-09-30T03:00:00Z","author":{"username":"alice"},"attachments":[{"filename":"file","url":"https://example.com/a?x=1&y=2"}],"embeds":[{"title":"card","url":"javascript:alert(1)","description":"<b>text</b>"}]},{"id":"10","content":"older","timestamp":"2026-09-29T03:00:00Z"}]`)
	} else {
		io.WriteString(w, `{"id":"123","guild_id":"456","name":"test","type":0}`)
	}
}
func finish(s *Service) {
	s.mu.Lock()
	jobs := make([]*job, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	s.mu.Unlock()
	for _, j := range jobs {
		<-j.done
	}
}
func readRSS(t *testing.T, b []byte) rss {
	t.Helper()
	var r rss
	if err := xml.Unmarshal(b, &r); err != nil {
		t.Fatal(err, string(b))
	}
	return r
}

func TestNormalRSSAndCacheIsolation(t *testing.T) {
	var calls atomic.Int32
	s, clock, db := setup(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); normal(w, r) })
	got := s.Get(context.Background(), "123", "private-token")
	if got.Status != 200 {
		t.Fatal(got)
	}
	parsed, err := parser.Parse(bytes.NewReader(got.Body))
	if err != nil || len(parsed.Items) != 2 {
		t.Fatalf("RSS parse: %v %+v", err, parsed)
	}
	rss := readRSS(t, got.Body)
	if rss.Channel.Items[0].GUID.Value != "20" || rss.Channel.Items[0].Author != "alice" {
		t.Fatal(rss)
	}
	body := rss.Channel.Items[0].Description
	if strings.Contains(body, "<script>") || strings.Contains(body, "javascript:") || !strings.Contains(body, "<br>") || !strings.Contains(body, "https://example.com") {
		t.Fatal(body)
	}
	if strings.Contains(string(got.Body), "private-token") {
		t.Fatal("token in RSS")
	}
	again := s.Get(context.Background(), "123", "private-token")
	if calls.Load() != 2 || !bytes.Equal(got.Body, again.Body) {
		t.Fatal("cache miss")
	}
	c, err := db.GetDiscordCache(cacheKey("123", "private-token"))
	if err != nil {
		t.Fatal(err)
	}
	if c.ExpiresAt-c.SucceededAt != 1800 || c.AccessedAt != clock.now().Unix() {
		t.Fatal(c)
	}
	encoded, _ := json.Marshal(c)
	if strings.Contains(string(encoded), "private-token") {
		t.Fatal("token stored")
	}
	other := s.Get(context.Background(), "123", "second-token")
	if other.Status != 200 || calls.Load() != 4 {
		t.Fatal("credential caches not isolated")
	}
}
func TestForumInlineContentAndMissingStarter(t *testing.T) {
	var calls atomic.Int32
	s, _, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.HasSuffix(r.URL.Path, "/123") {
			io.WriteString(w, `{"id":"123","guild_id":"456","name":"forum","type":15}`)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/threads/search") {
			t.Errorf("per-thread request: %s", r.URL.Path)
		}
		if r.URL.Query().Get("archived") == "false" {
			io.WriteString(w, `{"threads":[{"id":"300","name":"new"},{"id":"100","name":"old"}],"first_messages":[{"id":"300","channel_id":"300","content":"inline","author":{"global_name":"作者"}}],"has_more":false}`)
		} else {
			io.WriteString(w, `{"threads":[{"id":"200","name":"archived"},{"id":"100","name":"old"}],"has_more":false}`)
		}
	})
	got := s.Get(context.Background(), "123", "secret")
	if got.Status != 200 {
		t.Fatal(got)
	}
	items := readRSS(t, got.Body).Channel.Items
	if calls.Load() != 3 || len(items) != 3 || items[0].GUID.Value != "300" || items[1].GUID.Value != "200" || items[0].Description != "<p>inline</p>\n" || items[1].Description != "" {
		t.Fatal(items, calls.Load())
	}
}
func TestNightBoundariesAndRestart(t *testing.T) {
	for _, tc := range []struct {
		h, m int
		want bool
	}{{21, 59, false}, {22, 0, true}, {5, 59, true}, {6, 0, false}} {
		if night(time.Date(2026, 9, 30, tc.h, tc.m, 0, 0, shanghai)) != tc.want {
			t.Fatal(tc)
		}
	}
	var calls atomic.Int32
	s, clock, db := setup(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); normal(w, r) })
	old := s.Get(context.Background(), "123", "secret")
	clock.set(time.Date(2026, 9, 30, 23, 0, 0, 0, shanghai))
	cached := s.Get(context.Background(), "123", "secret")
	if !bytes.Equal(old.Body, cached.Body) || calls.Load() != 2 {
		t.Fatal("night fetched")
	}
	if got := s.Get(context.Background(), "123", "new-token"); got.Status != 503 || !strings.Contains(string(got.Body), "06:00") {
		t.Fatal(got)
	}
	restarted, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	restarted.client = s.client
	if got := restarted.Get(context.Background(), "123", "secret"); !bytes.Equal(old.Body, got.Body) || calls.Load() != 2 {
		t.Fatal("restart lost cache")
	}
}
func TestUnauthorizedNoticePersistsWithoutRepeating(t *testing.T) {
	var unauthorized atomic.Bool
	var calls atomic.Int32
	s, clock, db := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if unauthorized.Load() {
			w.WriteHeader(401)
			return
		}
		normal(w, r)
	})
	old := s.Get(context.Background(), "123", "secret")
	unauthorized.Store(true)
	clock.set(clock.now().Add(3 * time.Hour))
	stale := s.Get(context.Background(), "123", "secret")
	if !bytes.Equal(stale.Body, old.Body) {
		t.Fatal("did not return stale")
	}
	finish(s)
	failed := s.Get(context.Background(), "123", "secret")
	items := readRSS(t, failed.Body).Channel.Items
	if failed.Status != 200 || len(items) != 3 || !strings.Contains(items[0].Title, "凭据已失效") {
		t.Fatal(items)
	}
	repeat := s.Get(context.Background(), "123", "secret")
	if !bytes.Equal(failed.Body, repeat.Body) || calls.Load() != 3 {
		t.Fatal("repeated failure")
	}
	restarted, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	restarted.client = s.client
	if got := restarted.Get(context.Background(), "123", "secret"); !bytes.Equal(got.Body, failed.Body) || calls.Load() != 3 {
		t.Fatal("401 state lost")
	}
}
func TestColdUnauthorizedAndPermissionErrors(t *testing.T) {
	for _, status := range []int{401, 403, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			s, _, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(status) })
			result := s.Get(context.Background(), "123", "secret")
			if result.Status != 200 {
				t.Fatal(result)
			}
			title := readRSS(t, result.Body).Channel.Items[0].Title
			if status != 401 && strings.Contains(title, "失效") {
				t.Fatal(title)
			}
			s.Get(context.Background(), "123", "secret")
			if calls.Load() != 1 {
				t.Fatal("retried in cooldown")
			}
		})
	}
}
func TestConcurrentColdRequestsAndInitialTimeout(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	s, _, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		normal(w, r)
	})
	s.initialWait = 5 * time.Millisecond
	first := make(chan Result, 1)
	go func() { first <- s.Get(context.Background(), "123", "secret") }()
	<-entered
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := s.Get(context.Background(), "123", "secret")
			if got.Status != 503 {
				t.Errorf("got %d", got.Status)
			}
		}()
	}
	wg.Wait()
	if got := <-first; got.Status != 503 || got.RetryAfter != 30 {
		t.Fatal(got)
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate fetches")
	}
	close(release)
	finish(s)
	if got := s.Get(context.Background(), "123", "secret"); got.Status != 200 || calls.Load() != 2 {
		t.Fatal(got, calls.Load())
	}
}
func TestRetriesCooldownAndRateLimitPersistence(t *testing.T) {
	var calls atomic.Int32
	s, clock, db := setup(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) })
	if got := s.Get(context.Background(), "123", "secret"); got.Status != 503 || calls.Load() != 3 {
		t.Fatal(got, calls.Load())
	}
	s.Get(context.Background(), "123", "secret")
	if calls.Load() != 3 {
		t.Fatal("cooldown bypass")
	}
	if len(clock.waits) != 2 || clock.waits[0] != 5*time.Second || clock.waits[1] != 10*time.Second {
		t.Fatal(clock.waits)
	}
	// A different credential encounters a long Discord rate limit.
	s.client.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(429)
		io.WriteString(w, `{"retry_after":7200.5}`)
		return w.Result(), nil
	})
	got := s.Get(context.Background(), "123", "second")
	if got.Status != 503 || got.RetryAfter < 7200 {
		t.Fatal(got)
	}
	stored, err := db.GetDiscordCache("discord-rate-limit")
	if err != nil || stored.RetryAt < clock.now().Add(2*time.Hour).Unix() {
		t.Fatal(stored, err)
	}
	saved, err := db.GetDiscordCache(cacheKey("123", "second"))
	if err != nil || saved.RetryAt < stored.RetryAt {
		t.Fatal(saved, err)
	}
	// Check production initialization actually restores the process-wide gate.
	previous := processScheduler
	processScheduler = newScheduler()
	defer func() { processScheduler = previous }()
	restarted, err := New(db)
	if err != nil || restarted.client.gate.deadline().Unix() != stored.RetryAt {
		t.Fatal("rate limit not restored", err)
	}
}
func TestNightInterruptionDoesNotPublishPartial(t *testing.T) {
	var clock *fakeClock
	var calls atomic.Int32
	s, c, db := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		normal(w, r)
		clock.set(time.Date(2026, 9, 30, 22, 0, 0, 0, shanghai))
	})
	clock = c
	clock.set(time.Date(2026, 9, 30, 21, 59, 59, 0, shanghai))
	result := s.Get(context.Background(), "123", "secret")
	cache, _ := db.GetDiscordCache(cacheKey("123", "secret"))
	if result.Status != 503 || calls.Load() != 1 || len(cache.RSS) != 0 || cache.Failure != "night" {
		t.Fatal(result, cache, calls.Load())
	}
}
func TestTaskDeadlineAndCancellation(t *testing.T) {
	s, _, _ := setup(t, normal)
	s.taskTimeout = 5 * time.Millisecond
	s.client.gate.slot <- struct{}{} // Another request occupies the global queue.
	got := s.Get(context.Background(), "123", "secret")
	<-s.client.gate.slot
	if got.Status != 503 || !strings.Contains(string(got.Body), "暂时失败") {
		t.Fatal(got)
	}
}
func TestValidationAndEmptyFeed(t *testing.T) {
	var calls atomic.Int32
	s, _, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.HasSuffix(r.URL.Path, "/messages") {
			io.WriteString(w, `[]`)
		} else {
			normal(w, r)
		}
	})
	for _, pair := range [][2]string{{"abc", "secret"}, {"18446744073709551616", "secret"}, {"123", ""}, {"123", "secret\r\nX: y"}} {
		if got := s.Get(context.Background(), pair[0], pair[1]); got.Status != 400 {
			t.Fatal(got)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input reached upstream")
	}
	got := s.Get(context.Background(), "123", "secret")
	if got.Status != 200 || len(readRSS(t, got.Body).Channel.Items) != 0 {
		t.Fatal(got)
	}
}

func TestForumPaginationTop100AndStalledPage(t *testing.T) {
	for _, stalled := range []bool{false, true} {
		t.Run(fmt.Sprint(stalled), func(t *testing.T) {
			var calls atomic.Int32
			s, _, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if strings.HasSuffix(r.URL.Path, "/123") {
					io.WriteString(w, `{"id":"123","guild_id":"456","name":"forum","type":15}`)
					return
				}
				offset := 0
				fmt.Sscan(r.URL.Query().Get("offset"), &offset)
				if stalled {
					offset = 0
				}
				start := 500
				if r.URL.Query().Get("archived") == "true" {
					start = 499
				}
				batch := []thread{}
				for i := offset; i < offset+25 && i < 100; i++ {
					batch = append(batch, thread{ID: fmt.Sprint(start - i*2), Name: "post"})
				}
				json.NewEncoder(w).Encode(map[string]interface{}{"threads": batch, "has_more": offset+25 < 100})
			})
			got := s.Get(context.Background(), "123", "secret")
			if stalled {
				if got.Status != 503 || calls.Load() != 3 {
					t.Fatal(got, calls.Load())
				}
				return
			}
			items := readRSS(t, got.Body).Channel.Items
			if len(items) != 100 || calls.Load() != 7 {
				t.Fatal(len(items), calls.Load())
			}
			for i, item := range items {
				if item.GUID.Value != fmt.Sprint(500-i) {
					t.Fatal(i, item)
				}
			}
		})
	}
}

func TestFailedRefreshPreservesLastCompleteRSS(t *testing.T) {
	var fail atomic.Bool
	s, clock, db := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() && strings.HasSuffix(r.URL.Path, "/messages") {
			io.WriteString(w, `{"unexpected":"shape"}`)
			return
		}
		normal(w, r)
	})
	old := s.Get(context.Background(), "123", "secret")
	clock.set(clock.now().Add(3 * time.Hour))
	fail.Store(true)
	got := s.Get(context.Background(), "123", "secret")
	if !bytes.Equal(got.Body, old.Body) {
		t.Fatal("did not return stale")
	}
	finish(s)
	got = s.Get(context.Background(), "123", "secret")
	c, _ := db.GetDiscordCache(cacheKey("123", "secret"))
	if !bytes.Equal(got.Body, old.Body) || c.Failure != "invalid" || !bytes.Equal(c.RSS, old.Body) {
		t.Fatal("partial result replaced cache", c)
	}
}

func TestGlobalSchedulerSerializesAndSpacesDifferentCredentials(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	var active atomic.Int32
	var clock *fakeClock
	var timesMu sync.Mutex
	var times []time.Time
	s, c, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if active.Add(1) != 1 {
			t.Error("concurrent Discord requests")
		}
		defer active.Add(-1)
		timesMu.Lock()
		times = append(times, clock.now())
		timesMu.Unlock()
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		normal(w, r)
	})
	clock = c
	results := make(chan Result, 2)
	go func() { results <- s.Get(context.Background(), "123", "one") }()
	<-entered
	go func() { results <- s.Get(context.Background(), "123", "two") }()
	close(release)
	for i := 0; i < 2; i++ {
		if got := <-results; got.Status != 200 {
			t.Fatal(got)
		}
	}
	if calls.Load() != 4 {
		t.Fatal(calls.Load())
	}
	for i := 1; i < len(times); i++ {
		if times[i].Sub(times[i-1]) < 3*time.Second {
			t.Fatal(times)
		}
	}
}

func TestQueueWaitChecksNightAgain(t *testing.T) {
	var calls atomic.Int32
	s, clock, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); normal(w, r) })
	clock.set(time.Date(2026, 9, 30, 21, 59, 59, 0, shanghai))
	s.client.gate.next = clock.now().Add(3 * time.Second)
	got := s.Get(context.Background(), "123", "secret")
	if got.Status != 503 || calls.Load() != 0 {
		t.Fatal(got, calls.Load())
	}
}

func TestRedirectNeverForwardsCredential(t *testing.T) {
	var calls atomic.Int32
	s, _, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Location", "https://example.com/collect")
		w.WriteHeader(302)
	})
	got := s.Get(context.Background(), "123", "secret")
	if got.Status != 503 || calls.Load() != 1 || strings.Contains(string(got.Body), "secret") {
		t.Fatal(got, calls.Load())
	}
}

func TestRandomRangesAndRetryHeaders(t *testing.T) {
	c := newClient()
	for i := 0; i < 100; i++ {
		for _, bounds := range [][2]time.Duration{{30 * time.Minute, 2 * time.Hour}, {3 * time.Second, 7 * time.Second}} {
			got := c.jitter(bounds[0], bounds[1])
			if got < bounds[0] || got > bounds[1] {
				t.Fatal(got)
			}
		}
	}
	now := time.Date(2026, 9, 30, 4, 0, 0, 0, time.UTC)
	if got := retryDelay(now.Add(2*time.Hour).Format(http.TimeFormat), []byte(`{}`), now); got != 2*time.Hour {
		t.Fatal(got)
	}
	if got := retryDelay("", []byte(`{"retry_after": -1}`), now); got != 15*time.Minute {
		t.Fatal(got)
	}
}

func TestImageAttachmentsRenderInline(t *testing.T) {
	for _, tc := range []struct {
		name, filename, mime, link string
		image                      bool
	}{
		{"mime", "upload", "image/png", "https://cdn.discordapp.com/a?x=1&y=2", true},
		{"extension", "photo.JPG", "", "https://cdn.discordapp.com/photo.JPG?ex=123", true},
		{"file", "archive.zip", "application/zip", "https://cdn.discordapp.com/archive.zip", false},
		{"explicit non-image", "fake.png", "application/octet-stream", "https://cdn.discordapp.com/fake.png", false},
		{"unsafe URL", "photo.png", "image/png", "javascript:alert(1)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]interface{}{"id": "20", "timestamp": "2026-09-30T03:00:00Z", "attachments": []map[string]string{{"filename": tc.filename, "content_type": tc.mime, "url": tc.link}}})
			var m message
			if err := json.Unmarshal(payload, &m); err != nil {
				t.Fatal(err)
			}
			item, err := messageItem(m, "https://discord.com/channels/1/2")
			if err != nil {
				t.Fatal(err)
			}
			body, err := encodeRSS(rss{Version: "2.0", Channel: rssChannel{Items: []rssItem{item}}})
			if err != nil {
				t.Fatal(err)
			}
			rendered := sanitizer.Sanitize("https://discord.com", readRSS(t, body).Channel.Items[0].Description)
			if strings.Contains(rendered, "<img ") != tc.image {
				t.Fatalf("inline image=%v, got %s", tc.image, rendered)
			}
			if tc.image && (!strings.Contains(rendered, `<a href=`) || !strings.Contains(rendered, `alt="`)) {
				t.Fatal("image must retain original link and alt text", rendered)
			}
		})
	}
}

func TestFirstImageEnclosureFeedsListImage(t *testing.T) {
	for _, forum := range []bool{false, true} {
		t.Run(fmt.Sprint(forum), func(t *testing.T) {
			starter := message{ID: "300", ChannelID: "300", Timestamp: "2026-09-30T03:00:00Z", Attachments: []attachment{
				{Filename: "archive.zip", URL: "https://cdn.discordapp.com/a.zip", ContentType: "application/zip", Size: 42},
				{Filename: "first.jpg", URL: "https://cdn.discordapp.com/first.jpg?x=1&y=2", Size: 1234},
				{Filename: "second.png", URL: "https://cdn.discordapp.com/second.png", ContentType: "image/png", Size: 5678},
			}}
			payload := map[string]interface{}{"id": starter.ID, "channel_id": starter.ChannelID, "timestamp": starter.Timestamp, "attachments": starter.Attachments}
			s, _, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/123") {
					typ := 0
					if forum {
						typ = 15
					}
					json.NewEncoder(w).Encode(map[string]interface{}{"id": "123", "guild_id": "456", "type": typ})
					return
				}
				if forum {
					json.NewEncoder(w).Encode(map[string]interface{}{"threads": []thread{{ID: "300", Name: "post"}}, "first_messages": []interface{}{payload}, "has_more": false})
				} else {
					json.NewEncoder(w).Encode([]interface{}{payload})
				}
			})
			got := s.Get(context.Background(), "123", "secret")
			if got.Status != 200 {
				t.Fatal(got)
			}
			encoded := readRSS(t, got.Body).Channel.Items[0]
			if encoded.Enclosure == nil || encoded.Enclosure.URL != starter.Attachments[1].URL || encoded.Enclosure.Type != "image/jpeg" || encoded.Enclosure.Length != 1234 {
				t.Fatalf("bad enclosure: %+v", encoded.Enclosure)
			}
			if strings.Count(string(got.Body), "<enclosure ") != 1 {
				t.Fatal("must emit only the first image")
			}
			parsed, err := parser.Parse(bytes.NewReader(got.Body))
			if err != nil {
				t.Fatal(err)
			}
			if len(parsed.Items[0].MediaLinks) != 1 || parsed.Items[0].MediaLinks[0].Type != "image" || parsed.Items[0].MediaLinks[0].URL != starter.Attachments[1].URL {
				t.Fatal(parsed.Items[0].MediaLinks)
			}
			if !strings.Contains(parsed.Items[0].Content, "<img ") {
				t.Fatal("inline image lost")
			}
		})
	}
	if firstImageEnclosure(message{Attachments: []attachment{{Filename: "file.zip", URL: "https://example.com/file.zip"}}}) != nil {
		t.Fatal("non-image enclosure")
	}
}

func TestMarkdownRenderedInRSSBodies(t *testing.T) {
	for _, forum := range []bool{false, true} {
		t.Run(fmt.Sprint(forum), func(t *testing.T) {
			content := "# Heading\n\n**bold** and *italic* and ~~gone~~\nnext line\n\n- one\n- two\n\n> quote\n\n[link](https://example.com/read) and `code`\n\n```go\nfmt.Println(\"<safe>\")\n```\n\nhttps://example.com/plain"
			msg := message{ID: "300", ChannelID: "300", Timestamp: "2026-09-30T03:00:00Z", Content: content, Attachments: []attachment{{Filename: "cover.png", URL: "https://cdn.discordapp.com/cover.png", ContentType: "image/png"}}}
			s, _, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/123") {
					typ := 0
					if forum {
						typ = 15
					}
					json.NewEncoder(w).Encode(map[string]interface{}{"id": "123", "guild_id": "456", "type": typ})
					return
				}
				if forum {
					json.NewEncoder(w).Encode(map[string]interface{}{"threads": []thread{{ID: "300", Name: "post"}}, "first_messages": []message{msg}, "has_more": false})
				} else {
					json.NewEncoder(w).Encode([]message{msg})
				}
			})
			got := s.Get(context.Background(), "123", "secret")
			if got.Status != 200 {
				t.Fatal(got)
			}
			parsed, err := parser.Parse(bytes.NewReader(got.Body))
			if err != nil {
				t.Fatal(err)
			}
			rendered := parsed.Items[0].Content
			for _, want := range []string{"<h1>Heading</h1>", "<strong>bold</strong>", "<em>italic</em>", "<del>gone</del>", "<br>", "<ul>", "<li>one</li>", "<blockquote>", `href="https://example.com/read"`, "<code>code</code>", "<pre><code", "&lt;safe&gt;", `href="https://example.com/plain"`, `<img src="https://cdn.discordapp.com/cover.png"`} {
				if !strings.Contains(rendered, want) {
					t.Errorf("missing %q in %s", want, rendered)
				}
			}
			if len(parsed.Items[0].MediaLinks) != 1 {
				t.Fatal("cover lost")
			}
		})
	}
}

func TestMarkdownSafetyAndEmbedDescription(t *testing.T) {
	m := message{Content: "before\n\n<script>alert(1)</script>\n\n[bad](javascript:alert%281%29)\n\n![bad](data:text/html,payload)", Embeds: []embed{{Title: "card", URL: "https://example.com", Description: "**card description**"}}}
	m.Embeds[0].Fields = append(m.Embeds[0].Fields, struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{Name: "field", Value: "*value*"})
	got := bodyHTML(m)
	for _, bad := range []string{"<script", "javascript:", "data:text/html", "alert(1)"} {
		if strings.Contains(got, bad) {
			t.Fatal("unsafe output", got)
		}
	}
	if !strings.Contains(got, "<strong>card description</strong>") || !strings.Contains(got, "<em>value</em>") {
		t.Fatal(got)
	}
}
