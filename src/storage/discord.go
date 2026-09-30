package storage

import (
	"database/sql"
)

// DiscordCache contains no recoverable authorization credential.
type DiscordCache struct {
	Key         string
	RSS         []byte
	SucceededAt int64
	ExpiresAt   int64
	AccessedAt  int64
	Failure     string
	FailureAt   int64
	RetryAt     int64
}

func m26_discord_cache(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS discord_cache (
 cache_key TEXT PRIMARY KEY, rss BLOB NOT NULL, succeeded_at INTEGER NOT NULL,
 expires_at INTEGER NOT NULL, accessed_at INTEGER NOT NULL, failure TEXT NOT NULL,
 failure_at INTEGER NOT NULL, retry_at INTEGER NOT NULL);
 CREATE INDEX IF NOT EXISTS discord_cache_access ON discord_cache(accessed_at);`)
	return err
}

func (s *Storage) GetDiscordCache(key string) (DiscordCache, error) {
	c := DiscordCache{Key: key}
	err := s.db.QueryRow(`SELECT rss,succeeded_at,expires_at,accessed_at,failure,failure_at,retry_at FROM discord_cache WHERE cache_key=?`, key).Scan(&c.RSS, &c.SucceededAt, &c.ExpiresAt, &c.AccessedAt, &c.Failure, &c.FailureAt, &c.RetryAt)
	if err == sql.ErrNoRows {
		err = nil
	}
	return c, err
}

func (s *Storage) SaveDiscordCache(c DiscordCache) error {
	if c.RSS == nil {
		c.RSS = []byte{}
	}
	_, err := s.db.Exec(`INSERT INTO discord_cache VALUES(?,?,?,?,?,?,?,?)
 ON CONFLICT(cache_key) DO UPDATE SET rss=excluded.rss,succeeded_at=excluded.succeeded_at,
 expires_at=excluded.expires_at,accessed_at=MAX(discord_cache.accessed_at,excluded.accessed_at),
 failure=excluded.failure,failure_at=excluded.failure_at,retry_at=excluded.retry_at`,
		c.Key, c.RSS, c.SucceededAt, c.ExpiresAt, c.AccessedAt, c.Failure, c.FailureAt, c.RetryAt)
	return err
}

func (s *Storage) TouchDiscordCache(key string, now int64) error {
	_, err := s.db.Exec(`UPDATE discord_cache SET accessed_at=? WHERE cache_key=?`, now, key)
	return err
}

func (s *Storage) CleanDiscordCache(before int64) error {
	_, err := s.db.Exec(`DELETE FROM discord_cache WHERE accessed_at < ? AND cache_key != 'discord-rate-limit'`, before)
	return err
}
