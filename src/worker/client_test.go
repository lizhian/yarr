package worker

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestShouldRetryGet(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{err: io.EOF, want: true},
		{err: io.ErrUnexpectedEOF, want: true},
		{err: &url.Error{Op: "Get", URL: "https://example.com/feed.xml", Err: io.EOF}, want: true},
		{err: &url.Error{Op: "Get", URL: "https://example.com/feed.xml", Err: errors.New("connection refused")}, want: false},
		{err: nil, want: false},
	}

	for _, tt := range tests {
		if have := shouldRetryGet(tt.err); have != tt.want {
			t.Fatalf("shouldRetryGet(%v): want %v, have %v", tt.err, tt.want, have)
		}
	}
}

func TestClientGetUsesBrowserUserAgent(t *testing.T) {
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
	}))
	defer server.Close()

	res, err := client.get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if userAgent != browserUserAgent {
		t.Fatalf("want %q, have %q", browserUserAgent, userAgent)
	}
}

func TestClientGetAttemptsHTTP2(t *testing.T) {
	var proto string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proto = r.Proto
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	transport := client.httpClient.Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	testClient := &Client{
		httpClient: &http.Client{
			Transport: transport,
		},
		userAgent: browserUserAgent,
	}
	res, err := testClient.get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if proto != "HTTP/2.0" {
		t.Fatalf("want HTTP/2.0, have %q", proto)
	}
}

func TestClientRetriesTooManyRequestsOnce(t *testing.T) {
	requestStarts := make([]time.Time, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarts = append(requestStarts, time.Now())
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	testClient := &Client{
		httpClient: server.Client(),
		userAgent:  browserUserAgent,
	}

	res, err := testClient.get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("want status %d, have %d", http.StatusTooManyRequests, res.StatusCode)
	}
	if len(requestStarts) != 2 {
		t.Fatalf("want 2 requests, have %d", len(requestStarts))
	}
	if delay := requestStarts[1].Sub(requestStarts[0]); delay < tooManyRequestsRetryDelay {
		t.Fatalf("retry delay was too short: %s", delay)
	}
}
