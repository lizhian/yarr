package server

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/storage"
)

func TestDiscordRSSPublicRouteAndBasePath(t *testing.T) {
	for _, base := range []string{"", "/reader"} {
		t.Run(base, func(t *testing.T) {
			db := testServerDB(t)
			db.SetAuthConfig(true, "user", "password")
			server := NewServer(db, "")
			server.BasePath = base
			body := []byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Discord</title><link>https://discord.com</link><description>cached</description></channel></rss>`)
			err := db.SaveDiscordCache(storage.DiscordCache{Key: fmt.Sprintf("123:%x", sha256.Sum256([]byte("secret"))), RSS: body, ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), AccessedAt: time.Now().Unix()})
			if err != nil {
				t.Fatal(err)
			}
			handler := server.handler()
			for _, tc := range []struct {
				method, path string
				status       int
			}{{"GET", "/rss/discord/channel/123/secret", 200}, {"POST", "/rss/discord/channel/123/secret", 405}, {"GET", "/rss/discord/channel/invalid/secret", 400}, {"GET", "/api/feeds", 401}, {"GET", "/rss/discord/channel/123/secret/extra", 404}} {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, httptest.NewRequest(tc.method, base+tc.path, nil))
				if w.Code != tc.status {
					t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
				}
				if tc.status == 200 {
					if w.Body.String() != string(body) || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Type"), "application/rss+xml") {
						t.Fatal(w)
					}
				}
			}
			// A URL-encoded header control character is rejected before upstream access.
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, base+"/rss/discord/channel/123/secret%0D%0AX-test", nil))
			if w.Code != 400 {
				t.Fatal(w.Code)
			}
		})
	}
}
