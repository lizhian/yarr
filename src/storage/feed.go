package storage

import (
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/nkanaev/yarr/src/feedmeta"
)

const (
	FeedContentModeNormal      = "normal"
	FeedContentModeReadability = "readability"
	FeedContentModeEmbed       = "embed"

	FeedRankingModeOff          = "off"
	FeedRankingModeWithImage    = "with_image"
	FeedRankingModeWithoutImage = "without_image"
)

func ValidFeedContentMode(mode string) bool {
	switch mode {
	case FeedContentModeNormal, FeedContentModeReadability, FeedContentModeEmbed:
		return true
	default:
		return false
	}
}

func ValidFeedRankingMode(mode string) bool {
	switch mode {
	case FeedRankingModeOff, FeedRankingModeWithImage, FeedRankingModeWithoutImage:
		return true
	default:
		return false
	}
}

type Feed struct {
	Id                  int64      `json:"id"`
	FolderId            *int64     `json:"folder_id"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	Link                string     `json:"link"`
	FeedLink            string     `json:"feed_link"`
	ContentSelector     string     `json:"content_selector"`
	ContentMode         string     `json:"content_mode"`
	RankingMode         string     `json:"ranking_mode"`
	LastRankingItem     string     `json:"last_ranking_item"`
	LastRankingMD5      string     `json:"last_ranking_md5"`
	IconURL             string     `json:"icon_url"`
	CustomIcon          bool       `json:"custom_icon"`
	AutoReadScroll      bool       `json:"auto_read_scroll"`
	LatestItemArrivedAt *time.Time `json:"latest_item_arrived_at"`
}

func (s *Storage) CreateFeed(title, description, link, feedLink string, folderId *int64) *Feed {
	return s.CreateFeedWithContentSelector(title, description, link, feedLink, "", folderId)
}

func (s *Storage) CreateFeedWithContentSelector(title, description, link, feedLink, contentSelector string, folderId *int64) *Feed {
	return s.CreateFeedWithContentMode(title, description, link, feedLink, contentSelector, "", folderId)
}

func (s *Storage) CreateFeedWithContentMode(title, description, link, feedLink, contentSelector, contentMode string, folderId *int64) *Feed {
	return s.createFeed(title, description, link, feedLink, contentSelector, contentMode, FeedRankingModeOff, false, folderId)
}

func (s *Storage) CreateFeedWithRankingMode(title, description, link, feedLink, contentSelector, contentMode, rankingMode string, folderId *int64) *Feed {
	return s.createFeed(title, description, link, feedLink, contentSelector, contentMode, rankingMode, true, folderId)
}

func (s *Storage) createFeed(title, description, link, feedLink, contentSelector, contentMode, rankingMode string, updateRankingMode bool, folderId *int64) *Feed {
	title = feedmeta.CleanTitle(title)
	if title == "" {
		title = feedLink
	}
	if !ValidFeedContentMode(contentMode) {
		contentMode = ""
	}
	if !ValidFeedRankingMode(rankingMode) {
		rankingMode = FeedRankingModeOff
	}
	row := s.db.QueryRow(`
		insert into feeds (title, description, link, feed_link, content_selector, content_mode, ranking_mode, folder_id, auto_read_scroll)
		values (?, ?, ?, ?, ?, case when ? != '' then ? else 'normal' end, case when ? != '' then ? else 'off' end, ?, false)
		on conflict (feed_link) do update set
			folder_id = ?,
			content_selector = case
				when excluded.content_selector != '' then excluded.content_selector
				else feeds.content_selector
			end,
			content_mode = case
				when ? != '' then ?
				else feeds.content_mode
			end,
			ranking_mode = case
				when ? then ?
				else feeds.ranking_mode
			end
		returning id, content_selector, content_mode, ranking_mode, last_ranking_item, last_ranking_md5, icon_url, custom_icon, auto_read_scroll, latest_item_arrived_at`,
		title, description, link, feedLink, contentSelector, contentMode, contentMode, rankingMode, rankingMode, folderId,
		folderId,
		contentMode, contentMode,
		updateRankingMode, rankingMode,
	)

	var id int64
	var iconURL string
	var customIcon bool
	var lastRankingItem, lastRankingMD5 string
	var autoReadScroll bool
	var latestItemArrivedAt *time.Time
	err := row.Scan(&id, &contentSelector, &contentMode, &rankingMode, &lastRankingItem, &lastRankingMD5, &iconURL, &customIcon, &autoReadScroll, &latestItemArrivedAt)
	if err != nil {
		log.Print(err)
		return nil
	}
	return &Feed{
		Id:                  id,
		Title:               title,
		Description:         description,
		Link:                link,
		FeedLink:            feedLink,
		ContentSelector:     contentSelector,
		ContentMode:         contentMode,
		RankingMode:         rankingMode,
		LastRankingItem:     lastRankingItem,
		LastRankingMD5:      lastRankingMD5,
		IconURL:             iconURL,
		CustomIcon:          customIcon,
		AutoReadScroll:      autoReadScroll,
		LatestItemArrivedAt: latestItemArrivedAt,
		FolderId:            folderId,
	}
}

func (s *Storage) DeleteFeed(feedId int64) bool {
	result, err := s.db.Exec(`delete from feeds where id = ?`, feedId)
	if err != nil {
		log.Print(err)
		return false
	}
	nrows, err := result.RowsAffected()
	if err != nil {
		if err != sql.ErrNoRows {
			log.Print(err)
		}
		return false
	}
	return nrows == 1
}

func (s *Storage) RenameFeed(feedId int64, newTitle string) bool {
	newTitle = feedmeta.CleanTitle(newTitle)
	_, err := s.db.Exec(`update feeds set title = ? where id = ?`, newTitle, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedFolder(feedId int64, newFolderId *int64) bool {
	_, err := s.db.Exec(`update feeds set folder_id = ? where id = ?`, newFolderId, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedLink(feedId int64, newLink string) bool {
	_, err := s.db.Exec(`update feeds set feed_link = ? where id = ?`, newLink, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedMetadata(feedId int64, title, link, feedLink string) bool {
	title = feedmeta.CleanTitle(title)
	link = strings.TrimSpace(link)
	if feed := s.GetFeed(feedId); feed != nil {
		if !isRefreshMetadataPlaceholder(feed.Title) {
			title = ""
		}
		if !isRefreshMetadataPlaceholder(feed.Link) {
			link = ""
		}
	}
	_, err := s.db.Exec(`
		update feeds set
			title = case when ? != '' then ? else title end,
			link = case when ? != '' then ? else link end,
			feed_link = case when ? != '' then ? else feed_link end
		where id = ?`,
		title, title,
		link, link,
		feedLink, feedLink,
		feedId,
	)
	return err == nil
}

func isRefreshMetadataPlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.HasPrefix(value, "rsshub://")
}

func (f Feed) HasRefreshMetadataPlaceholder() bool {
	return isRefreshMetadataPlaceholder(f.Title) || isRefreshMetadataPlaceholder(f.Link)
}

func (s *Storage) UpdateFeedContentSelector(feedId int64, selector string) bool {
	_, err := s.db.Exec(`update feeds set content_selector = ? where id = ?`, selector, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedContentMode(feedId int64, mode string) bool {
	if !ValidFeedContentMode(mode) {
		return false
	}
	_, err := s.db.Exec(`update feeds set content_mode = ? where id = ?`, mode, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedRankingMode(feedId int64, mode string) bool {
	if !ValidFeedRankingMode(mode) {
		return false
	}
	_, err := s.db.Exec(`update feeds set ranking_mode = ? where id = ?`, mode, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedIconURL(feedId int64, iconURL string) bool {
	_, err := s.db.Exec(`update feeds set icon_url = ? where id = ? and not custom_icon`, iconURL, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedCustomIconURL(feedId int64, iconURL string) bool {
	_, err := s.db.Exec(`update feeds set icon_url = ?, custom_icon = ? where id = ?`, iconURL, iconURL != "", feedId)
	return err == nil
}

func (s *Storage) ResetFeedCustomIcon(feedId int64) bool {
	_, err := s.db.Exec(`update feeds set custom_icon = false where id = ?`, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedAutoReadScroll(feedId int64, enabled bool) bool {
	_, err := s.db.Exec(`update feeds set auto_read_scroll = ? where id = ?`, enabled, feedId)
	return err == nil
}

const feedSelectColumns = `id, folder_id, title, description, link, feed_link, content_selector, content_mode, ranking_mode, last_ranking_item, last_ranking_md5, icon_url, custom_icon, auto_read_scroll, latest_item_arrived_at`

type feedScanner interface {
	Scan(dest ...interface{}) error
}

func scanFeed(scanner feedScanner) (Feed, error) {
	var f Feed
	err := scanner.Scan(
		&f.Id,
		&f.FolderId,
		&f.Title,
		&f.Description,
		&f.Link,
		&f.FeedLink,
		&f.ContentSelector,
		&f.ContentMode,
		&f.RankingMode,
		&f.LastRankingItem,
		&f.LastRankingMD5,
		&f.IconURL,
		&f.CustomIcon,
		&f.AutoReadScroll,
		&f.LatestItemArrivedAt,
	)
	return f, err
}

func (s *Storage) ListFeeds() []Feed {
	result := make([]Feed, 0)
	rows, err := s.db.Query(`
		select ` + feedSelectColumns + `
		from feeds
		order by title collate nocase
	`)
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

func (s *Storage) ListFeedsByLatestItemArrivedAt() []Feed {
	result := make([]Feed, 0)
	rows, err := s.db.Query(`
		select ` + feedSelectColumns + `
		from feeds
		order by latest_item_arrived_at desc, title collate nocase, id
	`)
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

func (s *Storage) ListFeedsMissingIconURLs() []Feed {
	result := make([]Feed, 0)
	rows, err := s.db.Query(`
		select ` + feedSelectColumns + `
		from feeds
		where icon_url = '' and not custom_icon
	`)
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

func (s *Storage) GetFeed(id int64) *Feed {
	f, err := scanFeed(s.db.QueryRow(`
		select `+feedSelectColumns+`
		from feeds where id = ?
	`, id))
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Print(err)
		}
		return nil
	}
	return &f
}

func (s *Storage) ListRankingModeFeeds() []Feed {
	result := make([]Feed, 0)
	rows, err := s.db.Query(`
		select ` + feedSelectColumns + `
		from feeds
		where ranking_mode != 'off'
		order by title collate nocase
	`)
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

func (s *Storage) SetFeedError(feedID int64, lastError error) {
	_, err := s.db.Exec(`
		insert into feed_errors (feed_id, error)
		values (?, ?)
		on conflict (feed_id) do update set error = excluded.error`,
		feedID, lastError.Error(),
	)
	if err != nil {
		log.Print(err)
	}
}

func (s *Storage) GetFeedErrors() map[int64]string {
	errors := make(map[int64]string)

	rows, err := s.db.Query(`select feed_id, error from feed_errors`)
	if err != nil {
		log.Print(err)
		return errors
	}

	for rows.Next() {
		var id int64
		var error string
		if err = rows.Scan(&id, &error); err != nil {
			log.Print(err)
		}
		errors[id] = error
	}
	return errors
}

func (s *Storage) SetFeedSize(feedId int64, size int) {
	_, err := s.db.Exec(`
		insert into feed_sizes (feed_id, size)
		values (?, ?)
		on conflict (feed_id) do update set size = excluded.size`,
		feedId, size,
	)
	if err != nil {
		log.Print(err)
	}
}
