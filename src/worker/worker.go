package worker

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nkanaev/yarr/src/rsshub"
	"github.com/nkanaev/yarr/src/storage"
)

const NUM_WORKERS = 4

type Worker struct {
	db                 *storage.Storage
	pending            *int32
	refresh            *time.Ticker
	reflock            sync.Mutex
	stopper            chan bool
	rsshubAvailability map[string]rsshubAvailability
	rsshubMu           sync.RWMutex
	rsshubHits         map[int64]rsshubRefreshHit
	rsshubRefresh      *time.Ticker
	rsshubStopper      chan bool
}

func NewWorker(db *storage.Storage) *Worker {
	pending := int32(0)
	return &Worker{
		db:                 db,
		pending:            &pending,
		rsshubAvailability: make(map[string]rsshubAvailability),
		rsshubHits:         make(map[int64]rsshubRefreshHit),
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
		for _, feed := range w.db.ListFeedsMissingIconURLs() {
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
	iconURL, err := findFeedIconURL(feedImageUrl, feed.Link, feedLink)
	if err != nil {
		log.Printf("Failed to find favicon for %s (%s): %s", feed.FeedLink, feed.Link, err)
	}
	if iconURL != "" {
		w.updateFeedIconURL(feed.Id, iconURL)
	}
}

func (w *Worker) updateFeedIconURL(feedID int64, iconURL string) bool {
	return w.db.UpdateFeedIconURL(feedID, iconURL)
}

func (w *Worker) updateRefreshedFeedIcon(result *FeedRefreshResult) {
	if result == nil || result.Feed == nil || result.Feed.ImageURL == "" {
		return
	}

	feed := w.db.GetFeed(result.FeedID)
	if feed == nil || feed.IconURL != "" {
		return
	}
	w.findFeedIcon(*feed, result.Feed.ImageURL, result.FeedLink)
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
			feedLink := result.FeedLink
			if rsshub.IsLink(result.StoredFeedLink) {
				feedLink = result.StoredFeedLink
			}
			w.db.UpdateFeedMetadata(result.FeedID, result.Feed.Title, result.Feed.SiteURL, feedLink)
			w.updateRefreshedFeedIcon(result)
		}
		if result != nil && len(result.Items) > 0 {
			w.db.CreateItems(result.Items)
			w.db.SetFeedSize(result.Items[0].FeedId, len(result.Items))
		}
		w.recordRSSHubRefreshHit(result)
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
