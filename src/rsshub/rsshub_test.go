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
