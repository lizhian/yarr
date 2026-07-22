package worker

import (
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	userAgent  string
}

const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
const tooManyRequestsRetryDelay = time.Second

func (c *Client) get(url string) (*http.Response, error) {
	return c.getConditional(url, "", "")
}

func (c *Client) getConditional(url, lastModified, etag string) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		res, err := c.doGetConditional(url, lastModified, etag)
		if attempt == 1 {
			return res, err
		}
		if shouldRetryGet(err) {
			continue
		}
		if err == nil && res.StatusCode == http.StatusTooManyRequests {
			res.Body.Close()
			time.Sleep(tooManyRequestsRetryDelay)
			continue
		}
		return res, err
	}
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
	return c.httpClient.Do(req)
}

func shouldRetryGet(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

var client *Client

func init() {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: 2 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
	}
	httpClient := &http.Client{
		Timeout:   time.Second * 10,
		Transport: transport,
	}
	client = &Client{
		httpClient: httpClient,
		userAgent:  browserUserAgent,
	}
}
