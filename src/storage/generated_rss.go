package storage

import (
	"database/sql"
	"log"
	"time"
)

type GeneratedRSSSource struct {
	Id          int64
	Key         string
	Title       string
	Link        string
	Description string
}

type GeneratedRSSItem struct {
	Id          int64
	SourceId    int64
	GUID        string
	Title       string
	Link        string
	Content     string
	PublishedAt time.Time
}

func (s *Storage) UpsertGeneratedRSSSource(key, title, link, description string) *GeneratedRSSSource {
	now := time.Now().UTC()
	row := s.db.QueryRow(`
		insert into generated_rss_sources (
			key, title, link, description, created_at, updated_at
		)
		values (?, ?, ?, ?, ?, ?)
		on conflict (key) do update set
			title = excluded.title,
			link = excluded.link,
			description = excluded.description,
			updated_at = excluded.updated_at
		returning id, key, title, link, description`,
		key, title, link, description, now, now,
	)

	var source GeneratedRSSSource
	if err := row.Scan(&source.Id, &source.Key, &source.Title, &source.Link, &source.Description); err != nil {
		log.Print(err)
		return nil
	}
	return &source
}

func (s *Storage) GetGeneratedRSSSource(key string) *GeneratedRSSSource {
	var source GeneratedRSSSource
	err := s.db.QueryRow(`
		select id, key, title, link, description
		from generated_rss_sources
		where key = ?`, key,
	).Scan(&source.Id, &source.Key, &source.Title, &source.Link, &source.Description)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Print(err)
		}
		return nil
	}
	return &source
}

func (s *Storage) UpsertGeneratedRSSItem(sourceKey, guid, title, link, content string, publishedAt time.Time) bool {
	source := s.GetGeneratedRSSSource(sourceKey)
	if source == nil {
		return false
	}

	now := time.Now().UTC()
	_, err := s.db.Exec(`
		insert into generated_rss_items (
			source_id, guid, title, link, content, published_at, created_at, updated_at
		)
		values (?, ?, ?, ?, ?, ?, ?, ?)
		on conflict (source_id, guid) do update set
			title = excluded.title,
			link = excluded.link,
			content = excluded.content,
			published_at = excluded.published_at,
			updated_at = excluded.updated_at`,
		source.Id, guid, title, link, content, publishedAt.UTC(), now, now,
	)
	if err != nil {
		log.Print(err)
		return false
	}
	return true
}

func (s *Storage) ListGeneratedRSSItems(sourceKey string, limit int) []GeneratedRSSItem {
	result := make([]GeneratedRSSItem, 0)
	if limit <= 0 {
		return result
	}

	rows, err := s.db.Query(`
		select i.id, i.source_id, i.guid, i.title, i.link, i.content, i.published_at
		from generated_rss_items i
		join generated_rss_sources s on s.id = i.source_id
		where s.key = ?
		order by i.published_at desc, i.id desc
		limit ?`,
		sourceKey, limit,
	)
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var item GeneratedRSSItem
		err := rows.Scan(
			&item.Id,
			&item.SourceId,
			&item.GUID,
			&item.Title,
			&item.Link,
			&item.Content,
			&item.PublishedAt,
		)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, item)
	}
	return result
}
