package storage

import (
	"database/sql"
	"log"
	"strings"

	"github.com/nkanaev/yarr/src/feedmeta"
)

const (
	FeedContentModeNormal      = "normal"
	FeedContentModeReadability = "readability"
	FeedContentModeEmbed       = "embed"
)

func ValidFeedContentMode(mode string) bool {
	switch mode {
	case FeedContentModeNormal, FeedContentModeReadability, FeedContentModeEmbed:
		return true
	default:
		return false
	}
}

type Feed struct {
	Id              int64  `json:"id"`
	FolderId        *int64 `json:"folder_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Link            string `json:"link"`
	FeedLink        string `json:"feed_link"`
	ContentSelector string `json:"content_selector"`
	ContentMode     string `json:"content_mode"`
	IconURL         string `json:"icon_url"`
}

func (s *Storage) CreateFeed(title, description, link, feedLink string, folderId *int64) *Feed {
	return s.CreateFeedWithContentSelector(title, description, link, feedLink, "", folderId)
}

func (s *Storage) CreateFeedWithContentSelector(title, description, link, feedLink, contentSelector string, folderId *int64) *Feed {
	return s.CreateFeedWithContentMode(title, description, link, feedLink, contentSelector, "", folderId)
}

func (s *Storage) CreateFeedWithContentMode(title, description, link, feedLink, contentSelector, contentMode string, folderId *int64) *Feed {
	title = feedmeta.CleanTitle(title)
	if title == "" {
		title = feedLink
	}
	if !ValidFeedContentMode(contentMode) {
		contentMode = ""
	}
	row := s.db.QueryRow(`
		insert into feeds (title, description, link, feed_link, content_selector, content_mode, folder_id)
		values (?, ?, ?, ?, ?, case when ? != '' then ? else 'normal' end, ?)
		on conflict (feed_link) do update set
			folder_id = ?,
			content_selector = case
				when excluded.content_selector != '' then excluded.content_selector
				else feeds.content_selector
			end,
			content_mode = case
				when ? != '' then ?
				else feeds.content_mode
			end
		returning id, content_selector, content_mode, icon_url`,
		title, description, link, feedLink, contentSelector, contentMode, contentMode, folderId,
		folderId,
		contentMode, contentMode,
	)

	var id int64
	var iconURL string
	err := row.Scan(&id, &contentSelector, &contentMode, &iconURL)
	if err != nil {
		log.Print(err)
		return nil
	}
	return &Feed{
		Id:              id,
		Title:           title,
		Description:     description,
		Link:            link,
		FeedLink:        feedLink,
		ContentSelector: contentSelector,
		ContentMode:     contentMode,
		IconURL:         iconURL,
		FolderId:        folderId,
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

func (s *Storage) UpdateFeedIconURL(feedId int64, iconURL string) bool {
	_, err := s.db.Exec(`update feeds set icon_url = ? where id = ?`, iconURL, feedId)
	return err == nil
}

func (s *Storage) ListFeeds() []Feed {
	result := make([]Feed, 0)
	rows, err := s.db.Query(`
		select id, folder_id, title, description, link, feed_link, content_selector, content_mode, icon_url
		from feeds
		order by title collate nocase
	`)
	if err != nil {
		log.Print(err)
		return result
	}
	for rows.Next() {
		var f Feed
		err = rows.Scan(
			&f.Id,
			&f.FolderId,
			&f.Title,
			&f.Description,
			&f.Link,
			&f.FeedLink,
			&f.ContentSelector,
			&f.ContentMode,
			&f.IconURL,
		)
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
		select id, folder_id, title, description, link, feed_link, content_selector, content_mode, icon_url
		from feeds
		where icon_url = ''
	`)
	if err != nil {
		log.Print(err)
		return result
	}
	for rows.Next() {
		var f Feed
		err = rows.Scan(
			&f.Id,
			&f.FolderId,
			&f.Title,
			&f.Description,
			&f.Link,
			&f.FeedLink,
			&f.ContentSelector,
			&f.ContentMode,
			&f.IconURL,
		)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

func (s *Storage) GetFeed(id int64) *Feed {
	var f Feed
	err := s.db.QueryRow(`
		select
			id, folder_id, title, link, feed_link, content_selector, content_mode, icon_url
		from feeds where id = ?
	`, id).Scan(
		&f.Id, &f.FolderId, &f.Title, &f.Link, &f.FeedLink, &f.ContentSelector,
		&f.ContentMode,
		&f.IconURL,
	)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Print(err)
		}
		return nil
	}
	return &f
}

func (s *Storage) ResetFeedErrors() {
	if _, err := s.db.Exec(`delete from feed_errors`); err != nil {
		log.Print(err)
	}
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
