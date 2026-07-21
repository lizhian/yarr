package worker

import (
	"fmt"
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
	pending            atomic.Int32
	refresh            *time.Ticker
	reflock            sync.Mutex
	stopper            chan bool
	rsshubAvailability map[string]rsshubAvailability
	rsshubMu           sync.RWMutex
	rsshubHits         map[int64]rsshubRefreshHit
	rsshubLastSuccess  map[int64]string
	rsshubRoundRobin   int
	refreshDetails     map[int64]FeedRefreshDetail
	refreshDetailsMu   sync.RWMutex
	rsshubRefresh      *time.Ticker
	rsshubStopper      chan bool
	rankingModeExists  map[string]time.Time
	rankingModeMu      sync.Mutex
}

type feedRefreshJobResult struct {
	feed   storage.Feed
	result *FeedRefreshResult
	err    error
}

func NewWorker(db *storage.Storage) *Worker {
	return &Worker{
		db:                 db,
		rsshubAvailability: make(map[string]rsshubAvailability),
		rsshubHits:         make(map[int64]rsshubRefreshHit),
		rsshubLastSuccess:  make(map[int64]string),
		refreshDetails:     make(map[int64]FeedRefreshDetail),
		rankingModeExists:  make(map[string]time.Time),
	}
}

func (w *Worker) FeedsPending() int32 {
	return w.pending.Load()
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

func (w *Worker) RefreshFeedIconURLs() {
	for _, feed := range w.db.ListFeeds() {
		w.RefreshFeedIconURL(feed)
	}
}

func (w *Worker) RefreshFeedIconURL(feed storage.Feed) {
	w.FindFeedFavicon(feed)
}

func (w *Worker) FindFeedFavicon(feed storage.Feed) {
	if feed.CustomIcon {
		return
	}
	feedImageUrl := ""
	feedLink := feed.FeedLink
	if result, err := w.DiscoverFeed(feed.FeedLink); err == nil && result.Feed != nil {
		feedImageUrl = result.Feed.ImageURL
		feedLink = result.FeedLink
	} else if err != nil {
		log.Printf("Failed to discover favicon feed image for %s: %s", feed.FeedLink, err)
	}
	w.findFeedIcon(feed, feedImageUrl, feedLink)
}

func (w *Worker) FindFeedIcon(feed storage.Feed, feedImageUrl string) {
	if feed.CustomIcon {
		return
	}
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
	if feed == nil || feed.CustomIcon {
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
	w.refreshFeeds(w.db.ListFeeds())
}

func (w *Worker) RefreshFeed(feed storage.Feed) {
	w.refreshFeeds([]storage.Feed{feed})
}

func (w *Worker) refreshFeeds(feeds []storage.Feed) {
	w.reflock.Lock()
	defer w.reflock.Unlock()

	if w.pending.Load() > 0 {
		log.Print("Refreshing already in progress")
		return
	}

	if len(feeds) == 0 {
		log.Print("Nothing to refresh")
		return
	}

	log.Print("Refreshing feeds")
	w.pending.Store(int32(len(feeds)))
	go w.refresher(feeds)
}

func (w *Worker) refresher(feeds []storage.Feed) {
	srcqueue := make(chan storage.Feed, len(feeds))
	dstqueue := make(chan feedRefreshJobResult)

	for i := 0; i < NUM_WORKERS; i++ {
		go w.worker(srcqueue, dstqueue)
	}

	for _, feed := range feeds {
		srcqueue <- feed
	}
	for i := 0; i < len(feeds); i++ {
		job := <-dstqueue
		result := job.result
		newItems := 0
		fetchedItems := 0
		if job.err != nil {
			w.db.SetFeedError(job.feed.Id, job.err)
		}
		refreshSucceeded := job.err == nil
		if result != nil && result.Feed != nil {
			fetchedItems = len(result.Items)
			feedLink := result.FeedLink
			if rsshub.IsLink(result.StoredFeedLink) {
				feedLink = result.StoredFeedLink
			}
			w.appendRankingModeItem(job.feed, result, time.Now())
			update := storage.FeedRefreshUpdate{
				FeedID:         result.FeedID,
				UpdateMetadata: true,
				Title:          result.Feed.Title,
				Link:           result.Feed.SiteURL,
				FeedLink:       feedLink,
				Items:          result.Items,
				UpdateFeedSize: len(result.Items) > 0,
				FeedSize:       fetchedItems,
				LastModified:   result.LastModified,
				Etag:           result.Etag,
			}
			if result.RankingItem != nil {
				update.RankingItem = &result.RankingItem.Item
				update.RankingMD5 = result.RankingItem.MD5
			}
			var stored bool
			newItems, stored = w.db.ApplyFeedRefresh(update)
			if !stored {
				refreshSucceeded = false
				job.err = fmt.Errorf("failed to store refresh result")
				w.db.SetFeedError(job.feed.Id, job.err)
			} else {
				w.updateRefreshedFeedIcon(result)
				if result.RankingItem != nil {
					w.cacheRankingModeGUID(result.FeedID, result.RankingItem.Item.GUID, time.Now())
				}
			}
		} else if result != nil {
			_, refreshSucceeded = w.db.ApplyFeedRefresh(storage.FeedRefreshUpdate{
				FeedID:       result.FeedID,
				LastModified: result.LastModified,
				Etag:         result.Etag,
			})
			if !refreshSucceeded {
				job.err = fmt.Errorf("failed to store refresh result")
				w.db.SetFeedError(job.feed.Id, job.err)
			}
		}
		if refreshSucceeded {
			w.recordRSSHubRefreshHit(result)
		}
		w.recordFeedRefreshDetail(job.feed.Id, result, job.err, fetchedItems, newItems)
		w.pending.Add(-1)
	}
	close(srcqueue)
	close(dstqueue)

	log.Printf("Finished refreshing %d feeds", len(feeds))
}

func (w *Worker) worker(srcqueue <-chan storage.Feed, dstqueue chan<- feedRefreshJobResult) {
	for feed := range srcqueue {
		requestLinks, err := w.refreshLinks(feed)
		if err != nil {
			dstqueue <- feedRefreshJobResult{feed: feed, err: err}
			continue
		}
		result, err := refreshFeedFromLinks(feed, requestLinks, w.db)
		dstqueue <- feedRefreshJobResult{feed: feed, result: result, err: err}
	}
}
