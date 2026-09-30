// Package discord provides on-demand Discord RSS feeds without storing credentials.
package discord

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
	_ "time/tzdata"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

var shanghai = func() *time.Location {
	l, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return l
}()

func night(t time.Time) bool { h := t.In(shanghai).Hour(); return h >= 22 || h < 6 }
func wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type fetchError struct{ code string }

func (e *fetchError) Error() string { return failureText(e.code) }
func failureText(code string) string {
	switch code {
	case "unauthorized":
		return "Discord 凭据已失效，请替换订阅链接中的 token。"
	case "forbidden":
		return "无访问权限，请检查 Discord 账号的频道访问权限。"
	case "not_found":
		return "频道不存在或不可访问。"
	case "night":
		return "夜间暂停 Discord 请求，请于北京时间 06:00 后重试。"
	case "rate_limit":
		return "Discord 请求受到限流，请稍后重试。"
	case "invalid":
		return "Discord 返回内容不完整或格式异常，本次未更新。"
	default:
		return "Discord 抓取暂时失败，请稍后重试。"
	}
}

// A single scheduler is shared by all services in this process.
type scheduler struct {
	slot         chan struct{}
	mu           sync.Mutex
	next         time.Time
	limitedUntil time.Time
}

func newScheduler() *scheduler { return &scheduler{slot: make(chan struct{}, 1)} }

var processScheduler = newScheduler()

func (g *scheduler) limit(t time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if t.After(g.limitedUntil) {
		g.limitedUntil = t
	}
}
func (g *scheduler) deadline() time.Time { g.mu.Lock(); defer g.mu.Unlock(); return g.limitedUntil }

type client struct {
	http         *http.Client
	gate         *scheduler
	now          func() time.Time
	sleep        func(context.Context, time.Duration) error
	jitter       func(time.Duration, time.Duration) time.Duration
	persistLimit func(time.Time) error
}

func newClient() *client {
	return &client{http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, gate: processScheduler, now: time.Now, sleep: wait,
		jitter: func(a, b time.Duration) time.Duration { return a + time.Duration(rand.Int63n(int64(b-a)+1)) }}
}

func (c *client) get(ctx context.Context, path string, q url.Values, token string, out interface{}) error {
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if err := c.sleep(ctx, time.Duration(attempt*5)*time.Second); err != nil {
				return &fetchError{"temporary"}
			}
		}
		retry, err := c.once(ctx, path, q, token, out)
		if !retry || err == nil {
			return err
		}
	}
	return &fetchError{"temporary"}
}
func (c *client) once(ctx context.Context, path string, q url.Values, token string, out interface{}) (bool, error) {
	select {
	case c.gate.slot <- struct{}{}:
	case <-ctx.Done():
		return false, &fetchError{"temporary"}
	}
	defer func() { <-c.gate.slot }()
	c.gate.mu.Lock()
	next := c.gate.next
	if c.gate.limitedUntil.After(next) {
		next = c.gate.limitedUntil
	}
	c.gate.mu.Unlock()
	if night(c.now()) {
		return false, &fetchError{"night"}
	}
	if err := c.sleep(ctx, next.Sub(c.now())); err != nil {
		return false, &fetchError{"temporary"}
	}
	if night(c.now()) {
		return false, &fetchError{"night"}
	}
	if ctx.Err() != nil {
		return false, &fetchError{"temporary"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10"+path+"?"+q.Encode(), nil)
	if err != nil {
		return false, &fetchError{"invalid"}
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "application/json")
	defer func() {
		c.gate.mu.Lock()
		c.gate.next = c.now().Add(c.jitter(3*time.Second, 7*time.Second))
		c.gate.mu.Unlock()
	}()
	res, err := c.http.Do(req)
	if err != nil {
		return true, &fetchError{"temporary"}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 16*1024*1024+1))
	if err != nil {
		return true, &fetchError{"temporary"}
	}
	if len(body) > 16*1024*1024 {
		return false, &fetchError{"invalid"}
	}
	switch res.StatusCode {
	case 401:
		return false, &fetchError{"unauthorized"}
	case 403:
		return false, &fetchError{"forbidden"}
	case 404:
		return false, &fetchError{"not_found"}
	case 429:
		delay := retryDelay(res.Header.Get("Retry-After"), body, c.now())
		until := c.now().Add(delay)
		c.gate.limit(until)
		if c.persistLimit != nil {
			if err := c.persistLimit(until); err != nil {
				return false, &fetchError{"temporary"}
			}
		}
		// Release the slot and let a subsequent visit resume after the cooldown.
		return false, &fetchError{"rate_limit"}
	}
	if res.StatusCode >= 500 {
		return true, &fetchError{"temporary"}
	}
	if res.StatusCode != 200 {
		return false, &fetchError{"invalid"}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return false, &fetchError{"invalid"}
	}
	return false, nil
}
func retryDelay(header string, body []byte, now time.Time) time.Duration {
	delay := 15 * time.Minute
	apply := func(seconds float64) {
		if seconds > 0 && !math.IsNaN(seconds) && !math.IsInf(seconds, 0) {
			// Saturate duration overflow instead of wrapping into an early retry.
			max := float64(math.MaxInt64) / float64(time.Second)
			var d time.Duration
			if seconds >= max {
				d = time.Duration(math.MaxInt64)
			} else {
				d = time.Duration(math.Ceil(seconds * float64(time.Second)))
			}
			if d > delay {
				delay = d
			}
		}
	}
	var data struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if json.Unmarshal(body, &data) == nil {
		apply(data.RetryAfter)
	}
	if v, err := strconv.ParseFloat(header, 64); err == nil {
		apply(v)
	} else if t, err := http.ParseTime(header); err == nil {
		apply(t.Sub(now).Seconds())
	}
	return delay
}
func errorCode(err error) string {
	var f *fetchError
	if errors.As(err, &f) {
		return f.code
	}
	return "temporary"
}
