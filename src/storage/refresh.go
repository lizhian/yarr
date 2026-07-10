package storage

import (
	"database/sql"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/nkanaev/yarr/src/feedmeta"
)

type FeedRefreshUpdate struct {
	FeedID         int64
	UpdateMetadata bool
	Title          string
	Link           string
	FeedLink       string
	Items          []Item
	UpdateFeedSize bool
	FeedSize       int
	RankingItem    *Item
	RankingMD5     string
	LastModified   string
	Etag           string
}

func (s *Storage) ApplyFeedRefresh(update FeedRefreshUpdate) (int, bool) {
	tx, err := s.db.Begin()
	if err != nil {
		log.Print(err)
		return 0, false
	}
	rollback := func() (int, bool) {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Print(rollbackErr)
		}
		return 0, false
	}

	if update.UpdateMetadata {
		if err = updateFeedMetadataTx(tx, update); err != nil {
			log.Print(err)
			return rollback()
		}
	}

	created := 0
	items := ItemList(update.Items)
	sort.Sort(items)
	arrivedAt := time.Now().UTC()
	for _, item := range items {
		var inserted bool
		inserted, err = createItemTx(tx, item, arrivedAt)
		if err != nil {
			log.Print(err)
			return rollback()
		}
		if inserted {
			if err = syncSearchItemTx(tx, item); err != nil {
				log.Print(err)
				return rollback()
			}
			created++
		}
	}

	if update.UpdateFeedSize {
		_, err = tx.Exec(`
			insert into feed_sizes (feed_id, size)
			values (?, ?)
			on conflict (feed_id) do update set size = excluded.size`,
			update.FeedID, update.FeedSize,
		)
		if err != nil {
			log.Print(err)
			return rollback()
		}
	}

	if update.RankingItem != nil {
		inserted, insertErr := createItemTx(tx, *update.RankingItem, arrivedAt)
		if insertErr != nil {
			log.Print(insertErr)
			return rollback()
		}
		if inserted {
			if err = syncSearchItemTx(tx, *update.RankingItem); err != nil {
				log.Print(err)
				return rollback()
			}
			created++
		}
		_, err = tx.Exec(`
			update feeds
			set last_ranking_item = ?, last_ranking_md5 = ?
			where id = ?`,
			update.RankingItem.GUID, update.RankingMD5, update.FeedID,
		)
		if err != nil {
			log.Print(err)
			return rollback()
		}
	}

	_, err = tx.Exec(`
		insert into http_states (feed_id, last_modified, etag, last_refreshed)
		values (?, ?, ?, datetime())
		on conflict (feed_id) do update set
			last_modified = excluded.last_modified,
			etag = excluded.etag,
			last_refreshed = datetime()`,
		update.FeedID, update.LastModified, update.Etag,
	)
	if err != nil {
		log.Print(err)
		return rollback()
	}

	if _, err = tx.Exec(`delete from feed_errors where feed_id = ?`, update.FeedID); err != nil {
		log.Print(err)
		return rollback()
	}
	if err = tx.Commit(); err != nil {
		log.Print(err)
		return 0, false
	}
	return created, true
}

func updateFeedMetadataTx(tx *sql.Tx, update FeedRefreshUpdate) error {
	title := feedmeta.CleanTitle(update.Title)
	link := strings.TrimSpace(update.Link)
	var currentTitle, currentLink string
	if err := tx.QueryRow(`select title, link from feeds where id = ?`, update.FeedID).Scan(&currentTitle, &currentLink); err != nil {
		return err
	}
	if !isRefreshMetadataPlaceholder(currentTitle) {
		title = ""
	}
	if !isRefreshMetadataPlaceholder(currentLink) {
		link = ""
	}
	_, err := tx.Exec(`
		update feeds set
			title = case when ? != '' then ? else title end,
			link = case when ? != '' then ? else link end,
			feed_link = case when ? != '' then ? else feed_link end
		where id = ?`,
		title, title,
		link, link,
		update.FeedLink, update.FeedLink,
		update.FeedID,
	)
	return err
}
