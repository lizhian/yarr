package rsshub

import "testing"

func TestNormalizeBase(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "empty", raw: "", want: "", ok: true},
		{name: "trim trailing slash", raw: "https://rsshub.rssforever.com/", want: "https://rsshub.rssforever.com", ok: true},
		{name: "keep base path", raw: "https://example.com/rsshub/", want: "https://example.com/rsshub", ok: true},
		{name: "reject unsupported scheme", raw: "file:///tmp/rsshub", ok: false},
		{name: "reject missing host", raw: "https:///rsshub", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeBase(test.raw)
			if test.ok && err != nil {
				t.Fatal(err)
			}
			if !test.ok && err == nil {
				t.Fatal("expected error")
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	got, err := Resolve("rsshub://bilibili/weekly", "https://rsshub.rssforever.com/")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://rsshub.rssforever.com/bilibili/weekly"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveWithQuery(t *testing.T) {
	got, err := Resolve("rsshub://x/users?limit=10", "https://example.com/rsshub")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://example.com/rsshub/x/users?limit=10"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveRequiresConfiguredBase(t *testing.T) {
	_, err := Resolve("rsshub://bilibili/weekly", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRequiresRouteFormat(t *testing.T) {
	_, err := Resolve("rsshub:/bilibili/weekly", "https://rsshub.rssforever.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveLeavesRegularURL(t *testing.T) {
	got, err := Resolve("https://example.com/feed.xml", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/feed.xml" {
		t.Fatal(got)
	}
}

func TestNormalizeSubscriptionInput(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "regular URL", raw: "https://example.com/feed.xml", want: "https://example.com/feed.xml", ok: false},
		{name: "rsshub link", raw: "rsshub://bilibili/weekly", want: "rsshub://bilibili/weekly", ok: false},
		{name: "Bilibili UID", raw: " 703186600 ", want: "703186600", ok: false},
		{name: "Bilibili space", raw: "https://space.bilibili.com/703186600", want: "rsshub://bilibili/user/video/703186600", ok: true},
		{name: "Bilibili dynamic", raw: "https://space.bilibili.com/703186600/dynamic", want: "rsshub://bilibili/user/video/703186600", ok: true},
		{name: "Bilibili upload video", raw: "https://space.bilibili.com/703186600/upload/video", want: "rsshub://bilibili/user/video/703186600", ok: true},
		{name: "Bilibili ignores query", raw: "https://space.bilibili.com/703186600/dynamic?spm_id_from=333", want: "rsshub://bilibili/user/video/703186600", ok: true},
		{name: "Telegram ID", raw: "me888888888888", want: "me888888888888", ok: false},
		{name: "Telegram ID with at", raw: "@me888888888888", want: "@me888888888888", ok: false},
		{name: "Telegram channel URL", raw: "https://t.me/me888888888888", want: "rsshub://telegram/channel/me888888888888", ok: true},
		{name: "Telegram preview URL", raw: "https://t.me/s/me888888888888", want: "rsshub://telegram/channel/me888888888888", ok: true},
		{name: "Telegram message URL", raw: "https://t.me/me888888888888/123", want: "https://t.me/me888888888888/123", ok: false},
		{name: "Telegram invite URL", raw: "https://t.me/+invite", want: "https://t.me/+invite", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := NormalizeSubscriptionInput(test.raw)
			if ok != test.ok {
				t.Fatalf("got ok %v, want %v", ok, test.ok)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeBilibiliInput(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "UID", raw: " 703186600 ", want: "rsshub://bilibili/user/video/703186600", ok: true},
		{name: "space URL", raw: "https://space.bilibili.com/703186600", want: "rsshub://bilibili/user/video/703186600", ok: true},
		{name: "Telegram URL", raw: "https://t.me/me888888888888", want: "https://t.me/me888888888888", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := NormalizeBilibiliInput(test.raw)
			if ok != test.ok {
				t.Fatalf("got ok %v, want %v", ok, test.ok)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeTelegramInput(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "ID", raw: "me888888888888", want: "rsshub://telegram/channel/me888888888888", ok: true},
		{name: "ID with at", raw: "@me888888888888", want: "rsshub://telegram/channel/me888888888888", ok: true},
		{name: "channel URL", raw: "https://t.me/me888888888888", want: "rsshub://telegram/channel/me888888888888", ok: true},
		{name: "preview URL", raw: "https://t.me/s/me888888888888", want: "rsshub://telegram/channel/me888888888888", ok: true},
		{name: "Bilibili URL", raw: "https://space.bilibili.com/703186600", want: "https://space.bilibili.com/703186600", ok: false},
		{name: "message URL", raw: "https://t.me/me888888888888/123", want: "https://t.me/me888888888888/123", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := NormalizeTelegramInput(test.raw)
			if ok != test.ok {
				t.Fatalf("got ok %v, want %v", ok, test.ok)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeBaseList(t *testing.T) {
	got, err := NormalizeBaseList(`
		https://a.com/
		# https://b.com/rsshub/
		https://a.com

		https://c.com/?ignored=true
	`)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://a.com\n#https://b.com/rsshub\nhttps://c.com"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNormalizeBaseListKeepsFirstDuplicateState(t *testing.T) {
	got, err := NormalizeBaseList(`
		#https://a.com/
		https://a.com
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "#https://a.com" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeBaseListRejectsInvalidDisabledURL(t *testing.T) {
	_, err := NormalizeBaseList("# note")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEnabledBases(t *testing.T) {
	got, err := EnabledBases("https://a.com\n#https://b.com\nhttps://c.com")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://a.com", "https://c.com"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestResolveWithBaseListLimitsEnabledBases(t *testing.T) {
	got, err := ResolveWithBaseList("rsshub://bilibili/weekly", "https://a.com\n#https://b.com\nhttps://c.com", 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://a.com/bilibili/weekly"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
