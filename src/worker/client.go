package worker

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Client struct {
	httpClient      *http.Client
	userAgent       string
	requestInterval time.Duration
	limitersMu      sync.Mutex
	limiters        map[string]*requestLimiter
	requestSlots    chan struct{}
}

type requestLimiter struct {
	mu               sync.Mutex
	lastRequestStart time.Time
}

type releaseBody struct {
	io.ReadCloser
	release func()
	once    sync.Once
}

func (b *releaseBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if errors.Is(err, io.EOF) {
		b.releaseOnce()
	}
	return n, err
}

func (b *releaseBody) Close() error {
	err := b.ReadCloser.Close()
	b.releaseOnce()
	return err
}

func (b *releaseBody) releaseOnce() {
	b.once.Do(b.release)
}

const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func (c *Client) get(url string) (*http.Response, error) {
	return c.getConditional(url, "", "")
}

func (c *Client) getConditional(url, lastModified, etag string) (*http.Response, error) {
	res, err := c.doGetConditional(url, lastModified, etag)
	if shouldRetryGet(err) {
		return c.doGetConditional(url, lastModified, etag)
	}
	return res, err
}

func (c *Client) doGetConditional(url, lastModified, etag string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if c.requestSlots != nil {
		c.requestSlots <- struct{}{}
	}
	c.waitForRequestInterval(req.URL)
	res, err := c.httpClient.Do(req)
	if c.requestSlots == nil {
		return res, err
	}
	release := func() { <-c.requestSlots }
	if err != nil {
		release()
		return res, err
	}
	res.Body = &releaseBody{ReadCloser: res.Body, release: release}
	return res, nil
}

func shouldRetryGet(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func (c *Client) waitForRequestInterval(requestURL *url.URL) {
	if c.requestInterval <= 0 {
		return
	}
	origin := requestURL.Scheme + "://" + requestURL.Host
	c.limitersMu.Lock()
	if c.limiters == nil {
		c.limiters = make(map[string]*requestLimiter)
	}
	limiter := c.limiters[origin]
	if limiter == nil {
		limiter = &requestLimiter{}
		c.limiters[origin] = limiter
	}
	c.limitersMu.Unlock()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	wait := c.requestInterval - time.Since(limiter.lastRequestStart)
	if !limiter.lastRequestStart.IsZero() && wait > 0 {
		time.Sleep(wait)
	}
	limiter.lastRequestStart = time.Now()
}

var client *Client

func init() {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: time.Second * 10,
	}
	httpClient := &http.Client{
		Timeout:   time.Second * 10,
		Transport: transport,
	}
	client = &Client{
		httpClient:      httpClient,
		userAgent:       browserUserAgent,
		requestInterval: time.Second,
		limiters:        make(map[string]*requestLimiter),
		requestSlots:    make(chan struct{}, NUM_WORKERS),
	}
}
