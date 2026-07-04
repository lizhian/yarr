package worker

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
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

	testClient := *client
	testClient.httpClient = &http.Client{
		Transport: transport,
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
