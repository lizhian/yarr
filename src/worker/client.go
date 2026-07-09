package worker

import (
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	httpClient       *http.Client
	userAgent        string
	requestInterval  time.Duration
	requestMu        sync.Mutex
	lastRequestStart time.Time
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
	c.waitForRequestInterval()
	return c.httpClient.Do(req)
}

func shouldRetryGet(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func (c *Client) waitForRequestInterval() {
	if c.requestInterval <= 0 {
		return
	}
	c.requestMu.Lock()
	defer c.requestMu.Unlock()

	wait := c.requestInterval - time.Since(c.lastRequestStart)
	if !c.lastRequestStart.IsZero() && wait > 0 {
		time.Sleep(wait)
	}
	c.lastRequestStart = time.Now()
}

var client *Client

func init() {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:   true,
		DisableKeepAlives:   true,
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
	}
}
