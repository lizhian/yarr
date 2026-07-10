package worker

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
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

func TestClientLimitsRequestsPerOrigin(t *testing.T) {
	requestInterval := 50 * time.Millisecond
	testClient := &Client{
		requestInterval: requestInterval,
		limiters:        make(map[string]*requestLimiter),
	}
	urlA, _ := url.Parse("https://a.example/feed.xml")
	urlB, _ := url.Parse("https://b.example/feed.xml")
	testClient.waitForRequestInterval(urlA)
	testClient.waitForRequestInterval(urlB)

	started := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		testClient.waitForRequestInterval(urlA)
	}()
	go func() {
		defer wg.Done()
		testClient.waitForRequestInterval(urlB)
	}()
	wg.Wait()
	elapsed := time.Since(started)
	if elapsed < requestInterval/2 {
		t.Fatalf("requests were not limited: %s", elapsed)
	}
	if elapsed >= requestInterval*2 {
		t.Fatalf("different origins were serialized: %s", elapsed)
	}
}

func TestClientReleasesRequestSlotWhenBodyCloses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	testClient := &Client{
		httpClient:   server.Client(),
		userAgent:    browserUserAgent,
		requestSlots: make(chan struct{}, 1),
	}

	res, err := testClient.get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(testClient.requestSlots) != 1 {
		t.Fatalf("request slot was released before body close")
	}
	if err := res.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if len(testClient.requestSlots) != 0 {
		t.Fatalf("request slot was not released")
	}
}

func TestClientReleasesRequestSlotAtEndOfBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "body")
	}))
	defer server.Close()
	testClient := &Client{
		httpClient:   server.Client(),
		userAgent:    browserUserAgent,
		requestSlots: make(chan struct{}, 1),
	}

	res, err := testClient.get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(res.Body); err != nil {
		t.Fatal(err)
	}
	if len(testClient.requestSlots) != 0 {
		t.Fatal("request slot was not released at end of body")
	}
	defer res.Body.Close()

	secondDone := make(chan error, 1)
	go func() {
		second, err := testClient.get(server.URL)
		if err == nil {
			err = second.Body.Close()
		}
		secondDone <- err
	}()
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("second request blocked after first body reached EOF")
	}
}

func TestClientLimitsActualRequestStartsAfterSlotWait(t *testing.T) {
	requestInterval := 50 * time.Millisecond
	requestStarts := make(chan time.Time, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarts <- time.Now()
	}))
	defer server.Close()
	testClient := &Client{
		httpClient:      server.Client(),
		userAgent:       browserUserAgent,
		requestInterval: requestInterval,
		limiters:        make(map[string]*requestLimiter),
		requestSlots:    make(chan struct{}, 1),
	}
	testClient.requestSlots <- struct{}{}

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			res, err := testClient.get(server.URL)
			if err != nil {
				t.Error(err)
				return
			}
			res.Body.Close()
		}()
	}
	time.Sleep(requestInterval * 2)
	<-testClient.requestSlots
	wg.Wait()
	first := <-requestStarts
	second := <-requestStarts
	if second.Sub(first) < requestInterval/2 {
		t.Fatalf("actual request starts were too close: %s", second.Sub(first))
	}
}
