package worker

import (
	"bytes"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nkanaev/yarr/src/storage"
)

const NUM_WORKERS = 4

type Worker struct {
	db                 *storage.Storage
	pending            *int32
	refresh            *time.Ticker
	reflock            sync.Mutex
	stopper            chan bool
	feedImageUrls      map[int64]string
	feedImageUrlsMu    sync.RWMutex
	OnFeedIconUpdated  func(int64)
	rsshubAvailability map[string]rsshubAvailability
	rsshubMu           sync.RWMutex
	rsshubRefresh      *time.Ticker
	rsshubStopper      chan bool
}

func NewWorker(db *storage.Storage) *Worker {
	pending := int32(0)
	return &Worker{
		db:                 db,
		pending:            &pending,
		feedImageUrls:      make(map[int64]string),
		rsshubAvailability: make(map[string]rsshubAvailability),
	}
}

func (w *Worker) FeedsPending() int32 {
	return *w.pending
}

func (w *Worker) StartFeedCleaner() {
	go w.db.DeleteOldItems()
	ticker := time.NewTicker(time.Hour * 24)
	go func() {
		for {
			<-ticker.C
			w.db.DeleteOldItems()
		}
	}()
}

func (w *Worker) FindFavicons() {
	go func() {
		for _, feed := range w.db.ListFeedsMissingIcons() {
			w.FindFeedFavicon(feed)
		}
	}()
}

func (w *Worker) FindFeedFavicon(feed storage.Feed) {
	feedLink, err := w.resolveLink(feed.FeedLink)
	if err != nil {
		log.Printf("Failed to resolve favicon feed link for %s: %s", feed.FeedLink, err)
		return
	}
	feedImageUrl := ""
	if result, err := DiscoverFeedWithLink(feedLink, feed.FeedLink); err == nil && result.Feed != nil {
		feedImageUrl = result.Feed.ImageURL
	}
	w.findFeedIcon(feed, feedImageUrl, feedLink)
}

func (w *Worker) FindFeedIcon(feed storage.Feed, feedImageUrl string) {
	feedLink, err := w.resolveLink(feed.FeedLink)
	if err != nil {
		log.Printf("Failed to resolve icon feed link for %s: %s", feed.FeedLink, err)
		return
	}
	w.findFeedIcon(feed, feedImageUrl, feedLink)
}

func (w *Worker) findFeedIcon(feed storage.Feed, feedImageUrl, feedLink string) {
	if feedImageUrl != "" {
		if w.updateFeedIconFromImageUrl(feed.Id, feedImageUrl) {
			return
		}
		w.setFeedImageUrl(feed.Id, "")
	}

	icon, err := findFeedIcon("", feed.Link, feedLink)
	if err != nil {
		log.Printf("Failed to find favicon for %s (%s): %s", feed.FeedLink, feed.Link, err)
	}
	if icon != nil {
		w.updateFeedIcon(feed.Id, icon)
	}
}

func (w *Worker) updateFeedIconFromImageUrl(feedID int64, feedImageUrl string) bool {
	return w.updateFeedIconFromImageUrlIfChanged(feedID, feedImageUrl, nil)
}

func (w *Worker) updateFeedIconFromImageUrlIfChanged(feedID int64, feedImageUrl string, currentIcon *[]byte) bool {
	icon, err := fetchImage(feedImageUrl)
	if err != nil {
		return false
	}
	if icon == nil {
		return false
	}
	if currentIcon != nil && bytes.Equal(*currentIcon, *icon) {
		w.setFeedImageUrl(feedID, feedImageUrl)
		return true
	}
	if !w.updateFeedIcon(feedID, icon) {
		return false
	}
	w.setFeedImageUrl(feedID, feedImageUrl)
	return true
}

func (w *Worker) updateFeedIcon(feedID int64, icon *[]byte) bool {
	if !w.db.UpdateFeedIcon(feedID, icon) {
		return false
	}
	if w.OnFeedIconUpdated != nil {
		w.OnFeedIconUpdated(feedID)
	}
	return true
}

func (w *Worker) feedImageUrl(feedID int64) (string, bool) {
	w.feedImageUrlsMu.RLock()
	defer w.feedImageUrlsMu.RUnlock()
	url, ok := w.feedImageUrls[feedID]
	return url, ok
}

func (w *Worker) setFeedImageUrl(feedID int64, feedImageUrl string) {
	w.feedImageUrlsMu.Lock()
	defer w.feedImageUrlsMu.Unlock()
	w.feedImageUrls[feedID] = feedImageUrl
}

func (w *Worker) updateRefreshedFeedIcon(result *FeedRefreshResult) {
	if result == nil || result.Feed == nil || result.Feed.ImageURL == "" {
		return
	}

	if knownUrl, ok := w.feedImageUrl(result.FeedID); ok {
		if knownUrl != result.Feed.ImageURL {
			w.updateFeedIconFromImageUrl(result.FeedID, result.Feed.ImageURL)
		}
		return
	}

	feed := w.db.GetFeed(result.FeedID)
	if feed == nil {
		return
	}
	w.updateFeedIconFromImageUrlIfChanged(result.FeedID, result.Feed.ImageURL, feed.Icon)
}

func (w *Worker) SetRefreshRate(minute int64) {
	if w.stopper != nil {
		w.refresh.Stop()
		w.refresh = nil
		w.stopper <- true
		w.stopper = nil
	}
	w.setRSSHubRefreshRate(minute)

	if minute == 0 {
		return
	}

	w.stopper = make(chan bool)
	w.refresh = time.NewTicker(time.Minute * time.Duration(minute))

	go func(fire <-chan time.Time, stop <-chan bool, m int64) {
		log.Printf("auto-refresh %dm: starting", m)
		for {
			select {
			case <-fire:
				log.Printf("auto-refresh %dm: firing", m)
				w.RefreshFeeds()
			case <-stop:
				log.Printf("auto-refresh %dm: stopping", m)
				return
			}
		}
	}(w.refresh.C, w.stopper, minute)

}

func (w *Worker) RefreshFeeds() {
	w.reflock.Lock()
	defer w.reflock.Unlock()

	if *w.pending > 0 {
		log.Print("Refreshing already in progress")
		return
	}

	feeds := w.db.ListFeeds()
	if len(feeds) == 0 {
		log.Print("Nothing to refresh")
		return
	}

	log.Print("Refreshing feeds")
	atomic.StoreInt32(w.pending, int32(len(feeds)))
	go w.refresher(feeds)
}

func (w *Worker) refresher(feeds []storage.Feed) {
	w.db.ResetFeedErrors()

	srcqueue := make(chan storage.Feed, len(feeds))
	dstqueue := make(chan *FeedRefreshResult)

	for i := 0; i < NUM_WORKERS; i++ {
		go w.worker(srcqueue, dstqueue)
	}

	for _, feed := range feeds {
		srcqueue <- feed
	}
	for i := 0; i < len(feeds); i++ {
		result := <-dstqueue
		if result != nil && result.Feed != nil {
			w.db.UpdateFeedMetadata(result.FeedID, result.Feed.Title, result.Feed.SiteURL, result.FeedLink)
			w.updateRefreshedFeedIcon(result)
		}
		if result != nil && len(result.Items) > 0 {
			w.db.CreateItems(result.Items)
			w.db.SetFeedSize(result.Items[0].FeedId, len(result.Items))
		}
		atomic.AddInt32(w.pending, -1)
		w.db.SyncSearch()
	}
	close(srcqueue)
	close(dstqueue)

	log.Printf("Finished refreshing %d feeds", len(feeds))
}

func (w *Worker) worker(srcqueue <-chan storage.Feed, dstqueue chan<- *FeedRefreshResult) {
	for feed := range srcqueue {
		requestLinks, err := w.resolveLinks(feed.FeedLink)
		if err != nil {
			w.db.SetFeedError(feed.Id, err)
			dstqueue <- nil
			continue
		}
		result, err := refreshFeedFromLinks(feed, requestLinks, w.db)
		if err != nil {
			w.db.SetFeedError(feed.Id, err)
		}
		dstqueue <- result
	}
}
