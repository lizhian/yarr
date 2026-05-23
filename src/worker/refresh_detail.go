package worker

import (
	"time"
)

type FeedRefreshDetail struct {
	LastRefreshedAt time.Time `json:"last_refreshed_at"`
	Success         bool      `json:"success"`
	FetchedItems    int       `json:"fetched_items"`
	NewItems        int       `json:"new_items"`
	Error           string    `json:"error,omitempty"`
	RSSHubLink      string    `json:"rsshub_link,omitempty"`
}

func (w *Worker) recordFeedRefreshDetail(feedID int64, result *FeedRefreshResult, refreshErr error, newItems int) {
	detail := FeedRefreshDetail{
		LastRefreshedAt: time.Now().UTC(),
		Success:         refreshErr == nil,
	}
	if refreshErr != nil {
		detail.Error = refreshErr.Error()
	} else if result != nil {
		detail.FetchedItems = len(result.Items)
		if newItems > 0 {
			detail.NewItems = newItems
		}
		if result.RSSHubLink != "" {
			detail.RSSHubLink = result.RSSHubLink
		}
	}

	w.refreshDetailsMu.Lock()
	w.refreshDetails[feedID] = detail
	w.refreshDetailsMu.Unlock()
}

func (w *Worker) FeedRefreshDetails() map[int64]FeedRefreshDetail {
	w.refreshDetailsMu.RLock()
	defer w.refreshDetailsMu.RUnlock()

	details := make(map[int64]FeedRefreshDetail, len(w.refreshDetails))
	for feedID, detail := range w.refreshDetails {
		details[feedID] = detail
	}
	return details
}
