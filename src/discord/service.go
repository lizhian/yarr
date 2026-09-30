package discord

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nkanaev/yarr/src/storage"
)

type cacheStore interface {
	GetDiscordCache(string) (storage.DiscordCache, error)
	SaveDiscordCache(storage.DiscordCache) error
	TouchDiscordCache(string, int64) error
	CleanDiscordCache(int64) error
}
type job struct {
	done chan struct{}
	err  error
}
type Service struct {
	store       cacheStore
	client      *client
	mu          sync.Mutex
	jobs        map[string]*job
	initialWait time.Duration
	taskTimeout time.Duration
}
type Result struct {
	Body       []byte
	Status     int
	RetryAfter int
}

func unavailable(message string, retry int) Result {
	return Result{[]byte(message), http.StatusServiceUnavailable, retry}
}
func New(store cacheStore) (*Service, error) {
	c := newClient()
	saved, err := store.GetDiscordCache("discord-rate-limit")
	if err != nil {
		return nil, err
	}
	c.gate.limit(time.Unix(saved.RetryAt, 0))
	c.persistLimit = func(t time.Time) error {
		return store.SaveDiscordCache(storage.DiscordCache{Key: "discord-rate-limit", RetryAt: t.Unix() + 1, AccessedAt: c.now().Unix()})
	}
	s := &Service{store: store, client: c, jobs: map[string]*job{}, initialWait: 25 * time.Second, taskTimeout: 3 * time.Minute}
	if err := store.CleanDiscordCache(c.now().Add(-30 * 24 * time.Hour).Unix()); err != nil {
		return nil, err
	}
	return s, nil
}

// Cleanup runs no upstream requests and never holds authorization credentials.
func (s *Service) Cleanup(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			_ = s.store.CleanDiscordCache(s.client.now().Add(-30 * 24 * time.Hour).Unix())
			s.mu.Unlock()
		}
	}
}
func validToken(token string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}
	for _, b := range []byte(token) {
		if b < 32 || b == 127 {
			return false
		}
	}
	return true
}
func cacheKey(id, token string) string { return fmt.Sprintf("%s:%x", id, sha256.Sum256([]byte(token))) }
func (s *Service) Get(ctx context.Context, id, token string) Result {
	if !numericID(id) || !validToken(token) {
		return Result{Body: []byte("频道 ID 或 token 格式无效。"), Status: http.StatusBadRequest}
	}
	key := cacheKey(id, token)
	s.mu.Lock()
	cached, err := s.store.GetDiscordCache(key)
	if err != nil {
		s.mu.Unlock()
		return unavailable("RSS 缓存暂时不可用。", 30)
	}
	now := s.client.now()
	cached.AccessedAt = now.Unix()
	if err = s.store.TouchDiscordCache(key, now.Unix()); err != nil {
		s.mu.Unlock()
		return unavailable("RSS 缓存暂时不可用。", 30)
	}
	if cached.Failure == "unauthorized" || (len(cached.RSS) > 0 && now.Unix() < cached.ExpiresAt) || now.Unix() < cached.RetryAt || night(now) {
		s.mu.Unlock()
		return s.response(cached, id, now)
	}
	running := s.jobs[key]
	if running == nil {
		// Persist even the first visit, so a failed cold fetch has a durable cooldown.
		if err = s.store.SaveDiscordCache(cached); err != nil {
			s.mu.Unlock()
			return unavailable("RSS 缓存暂时不可用。", 30)
		}
		running = &job{done: make(chan struct{})}
		s.jobs[key] = running
		go s.refresh(key, id, token, running)
	}
	s.mu.Unlock()
	if len(cached.RSS) > 0 {
		return s.response(cached, id, now)
	}
	timer := time.NewTimer(s.initialWait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return unavailable("首次内容正在准备，请稍后重试。", 30)
	case <-timer.C:
		return unavailable("首次内容正在准备，请稍后重试。", 30)
	case <-running.done:
		if running.err != nil {
			return unavailable("RSS 缓存暂时不可用。", 30)
		}
		s.mu.Lock()
		cached, err = s.store.GetDiscordCache(key)
		s.mu.Unlock()
		if err != nil {
			return unavailable("RSS 缓存暂时不可用。", 30)
		}
		return s.response(cached, id, s.client.now())
	}
}
func (s *Service) response(c storage.DiscordCache, id string, now time.Time) Result {
	if c.Failure == "unauthorized" || c.Failure == "forbidden" || c.Failure == "not_found" {
		b, err := noticeRSS(c.RSS, id, c.Failure, c.FailureAt)
		if err != nil {
			return unavailable("RSS 缓存暂时不可用。", 30)
		}
		return Result{Body: b, Status: http.StatusOK}
	}
	if len(c.RSS) > 0 {
		return Result{Body: c.RSS, Status: http.StatusOK}
	}
	if night(now) {
		local := now.In(shanghai)
		morning := time.Date(local.Year(), local.Month(), local.Day(), 6, 0, 0, 0, shanghai)
		if local.Hour() >= 22 {
			morning = morning.AddDate(0, 0, 1)
		}
		return unavailable(failureText("night"), int(morning.Sub(now).Seconds())+1)
	}
	retry := 30
	if c.RetryAt > now.Unix() {
		retry = int(c.RetryAt - now.Unix())
	}
	return unavailable(failureText(c.Failure), retry)
}
func (s *Service) refresh(key, id, token string, j *job) {
	ctx, cancel := context.WithTimeout(context.Background(), s.taskTimeout)
	defer cancel()
	body, fetchErr := s.client.fetch(ctx, id, token)
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() { delete(s.jobs, key); close(j.done) }()
	c, err := s.store.GetDiscordCache(key)
	if err != nil {
		j.err = err
		return
	}
	now := s.client.now()
	if c.AccessedAt == 0 {
		c.AccessedAt = now.Unix()
	}
	if fetchErr == nil {
		c.RSS = body
		c.SucceededAt = now.Unix()
		c.ExpiresAt = now.Add(s.client.jitter(2*time.Hour, 4*time.Hour)).Unix()
		c.Failure = ""
		c.FailureAt = 0
		c.RetryAt = 0
	} else {
		code := errorCode(fetchErr)
		if c.Failure != code {
			c.FailureAt = now.Unix()
		}
		c.Failure = code
		c.RetryAt = now.Add(15 * time.Minute).Unix()
		if limit := s.client.gate.deadline().Unix() + 1; limit > c.RetryAt {
			c.RetryAt = limit
		}
		// Night interruption is not a daytime failure: retry is allowed at 06:00.
		if code == "night" {
			local := now.In(shanghai)
			t := time.Date(local.Year(), local.Month(), local.Day(), 6, 0, 0, 0, shanghai)
			if local.Hour() >= 22 {
				t = t.AddDate(0, 0, 1)
			}
			c.RetryAt = t.Unix()
		}
	}
	j.err = s.store.SaveDiscordCache(c)
}
