package worker

import (
	"errors"
	"io"
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
