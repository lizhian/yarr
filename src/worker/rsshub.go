package worker

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/nkanaev/yarr/src/rsshub"
)

const RSSHUB_MAX_ATTEMPTS = 5

type rsshubAvailability int

const (
	rsshubUnknown rsshubAvailability = iota
	rsshubAvailable
	rsshubUnavailable
)

type rsshubRefreshHit struct {
	BaseURL string
	Link    string
}

func (w *Worker) setRSSHubRefreshRate(minute int64) {
	if w.rsshubStopper != nil {
		w.rsshubRefresh.Stop()
		w.rsshubRefresh = nil
		w.rsshubStopper <- true
		w.rsshubStopper = nil
	}

	if minute == 0 {
		return
	}

	w.rsshubStopper = make(chan bool)
	w.rsshubRefresh = time.NewTicker(time.Minute * time.Duration(minute))

	go func(fire <-chan time.Time, stop <-chan bool, m int64) {
		log.Printf("rsshub availability %dm: starting", m)
		w.RefreshRSSHubAvailability()
		for {
			select {
			case <-fire:
				log.Printf("rsshub availability %dm: firing", m)
				w.RefreshRSSHubAvailability()
			case <-stop:
				log.Printf("rsshub availability %dm: stopping", m)
				return
			}
		}
	}(w.rsshubRefresh.C, w.rsshubStopper, minute)
}

func (w *Worker) ResetRSSHubAvailability() {
	w.rsshubMu.Lock()
	w.rsshubAvailability = make(map[string]rsshubAvailability)
	w.rsshubMu.Unlock()
}

func (w *Worker) ResetRSSHubRefreshHits() {
	w.rsshubMu.Lock()
	w.rsshubHits = make(map[int64]rsshubRefreshHit)
	w.rsshubMu.Unlock()
}

func (w *Worker) CheckRSSHubAvailability() {
	w.ResetRSSHubAvailability()
	w.ResetRSSHubRefreshHits()
	refreshRate := w.db.GetSettingsValueInt64("refresh_rate")
	if refreshRate > 0 {
		go w.RefreshRSSHubAvailability()
	}
}

func (w *Worker) RefreshRSSHubAvailability() {
	bases, err := rsshub.EnabledBases(w.db.GetSettingsValueString("rsshub_base_url"))
	if err != nil {
		log.Printf("Failed to parse RSSHub base list: %s", err)
		return
	}
	if len(bases) == 0 {
		return
	}

	type result struct {
		base   string
		status rsshubAvailability
	}
	srcqueue := make(chan string, len(bases))
	dstqueue := make(chan result)
	workers := NUM_WORKERS
	if len(bases) < workers {
		workers = len(bases)
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for base := range srcqueue {
				dstqueue <- result{base: base, status: checkRSSHubBase(base)}
			}
		}()
	}

	go func() {
		for _, base := range bases {
			srcqueue <- base
		}
		close(srcqueue)
		wg.Wait()
		close(dstqueue)
	}()

	statuses := make(map[string]rsshubAvailability)
	for result := range dstqueue {
		statuses[result.base] = result.status
	}

	w.rsshubMu.Lock()
	w.rsshubAvailability = statuses
	w.rsshubMu.Unlock()
}

func checkRSSHubBase(base string) rsshubAvailability {
	req, err := http.NewRequest("GET", base, nil)
	if err != nil {
		log.Printf("RSSHub base %s is unavailable: %s", base, err)
		return rsshubUnavailable
	}
	req.Header.Set("User-Agent", client.userAgent)
	checkClient := *client.httpClient
	checkClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	res, err := checkClient.Do(req)
	if err != nil {
		log.Printf("RSSHub base %s is unavailable: %s", base, err)
		return rsshubUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusBadRequest {
		return rsshubAvailable
	}
	log.Printf("RSSHub base %s is unavailable: status code %d", base, res.StatusCode)
	return rsshubUnavailable
}

func (w *Worker) rsshubBasesForRequest() ([]string, error) {
	enabled, err := rsshub.EnabledBases(w.db.GetSettingsValueString("rsshub_base_url"))
	if err != nil {
		return nil, err
	}
	if len(enabled) == 0 {
		return nil, fmt.Errorf("RSSHub base URL is not configured")
	}

	w.rsshubMu.RLock()
	available := make([]string, 0, len(enabled))
	for _, base := range enabled {
		if w.rsshubAvailability[base] == rsshubAvailable {
			available = append(available, base)
		}
	}
	w.rsshubMu.RUnlock()

	bases := enabled
	if len(available) > 0 {
		bases = available
	}
	if len(bases) > RSSHUB_MAX_ATTEMPTS {
		bases = bases[:RSSHUB_MAX_ATTEMPTS]
	}
	return bases, nil
}

func (w *Worker) resolveLinks(link string) ([]string, error) {
	if !rsshub.IsLink(link) {
		return []string{link}, nil
	}
	bases, err := w.rsshubBasesForRequest()
	if err != nil {
		return nil, err
	}
	return rsshub.ResolveWithBases(link, bases)
}

func (w *Worker) recordRSSHubRefreshHit(result *FeedRefreshResult) {
	if result == nil || !rsshub.IsLink(result.StoredFeedLink) || result.RSSHubBase == "" || result.RSSHubLink == "" {
		return
	}
	w.rsshubMu.Lock()
	w.rsshubHits[result.FeedID] = rsshubRefreshHit{
		BaseURL: result.RSSHubBase,
		Link:    result.RSSHubLink,
	}
	w.rsshubMu.Unlock()
}

type RSSHubRefreshFeedDetail struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}

type RSSHubRefreshDetail struct {
	BaseURL string                    `json:"base_url"`
	Feeds   int                       `json:"feeds"`
	Details []RSSHubRefreshFeedDetail `json:"details"`
}

func (w *Worker) RSSHubRefreshDetails() []RSSHubRefreshDetail {
	bases, err := rsshub.EnabledBases(w.db.GetSettingsValueString("rsshub_base_url"))
	if err != nil {
		return nil
	}
	counts := make(map[string]int, len(bases))
	for _, base := range bases {
		counts[base] = 0
	}
	feedDetails := make(map[string][]RSSHubRefreshFeedDetail, len(bases))

	w.rsshubMu.RLock()
	hits := make(map[int64]rsshubRefreshHit, len(w.rsshubHits))
	for feedID, hit := range w.rsshubHits {
		hits[feedID] = hit
	}
	w.rsshubMu.RUnlock()

	for feedID, hit := range hits {
		feed := w.db.GetFeed(feedID)
		if feed == nil || !rsshub.IsLink(feed.FeedLink) {
			continue
		}
		if _, ok := counts[hit.BaseURL]; ok {
			counts[hit.BaseURL]++
			feedDetails[hit.BaseURL] = append(feedDetails[hit.BaseURL], RSSHubRefreshFeedDetail{
				Title: feed.Title,
				Link:  hit.Link,
			})
		}
	}

	details := make([]RSSHubRefreshDetail, 0, len(bases))
	for _, base := range bases {
		details = append(details, RSSHubRefreshDetail{
			BaseURL: base,
			Feeds:   counts[base],
			Details: feedDetails[base],
		})
	}
	return details
}
