package assets

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestStaticAssetVersion(t *testing.T) {
	previousVersion := assetVersion
	SetVersion("3.58-deadbeef")
	t.Cleanup(func() { SetVersion(previousVersion) })

	tests := []struct {
		template string
		data     map[string]interface{}
		assets   []string
	}{
		{
			template: "index.html",
			data: map[string]interface{}{
				"settings":      map[string]interface{}{},
				"authenticated": false,
			},
			assets: []string{
				"stylesheets/bootstrap.min.css",
				"stylesheets/app.css",
				"javascripts/vue.min.js",
				"javascripts/api.js",
				"javascripts/app.js",
				"javascripts/key.js",
			},
		},
		{
			template: "login.html",
			data:     map[string]interface{}{},
			assets: []string{
				"stylesheets/bootstrap.min.css",
				"stylesheets/app.css",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.template, func(t *testing.T) {
			var output bytes.Buffer
			Render(test.template, &output, test.data)
			for _, asset := range test.assets {
				want := asset + "?v=3.58-deadbeef"
				if !strings.Contains(output.String(), want) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Contains(output.String(), "v=ranking-mode") {
				t.Error("found stale hard-coded asset version")
			}
		})
	}
}

func TestFeedTimeDoesNotDependOnUnreadCount(t *testing.T) {
	var output bytes.Buffer
	Render("index.html", &output, map[string]interface{}{
		"settings":      map[string]interface{}{},
		"authenticated": false,
	})

	html := output.String()
	for _, want := range []string{
		`:class="{'feed-settings-has-time': feed.latest_item_arrived_at}"`,
		`v-if="feed.latest_item_arrived_at"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(html, `filteredFeedStats[feed.id] &amp;&amp; feed.latest_item_arrived_at`) {
		t.Error("feed time visibility should not depend on unread count")
	}
}

func TestSelectedFeedKeepsActivityTimeVisible(t *testing.T) {
	file, err := FS.Open("stylesheets/app.css")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	css := string(content)
	for _, unwanted := range []string{
		".feed-selectgroup input:checked + .selectgroup-label .feed-settings-icon",
		".feed-selectgroup input:checked + .selectgroup-label .feed-settings-time",
		".feed-selectgroup:focus-within .feed-settings-icon",
		".feed-selectgroup:focus-within .feed-settings-time",
	} {
		if strings.Contains(css, unwanted) {
			t.Errorf("selected or focused feed should not switch activity time: found %q", unwanted)
		}
	}
	for _, want := range []string{
		".feed-selectgroup:hover .feed-settings-icon",
		".feed-selectgroup:hover .feed-settings-time",
		".feed-settings-action:focus .feed-settings-icon",
		".feed-settings-action:focus .feed-settings-time",
		".feed-selectgroup input:checked + .selectgroup-label .feed-settings-action:hover {\n  background-color: transparent;\n}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestChangingFeedKeepsSelectedItem(t *testing.T) {
	file, err := FS.Open("javascripts/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	javascript := string(content)
	start := strings.Index(javascript, "'feedSelected': function(newVal, oldVal)")
	end := strings.Index(javascript, "'itemSelected': function(newVal, oldVal)")
	if start == -1 || end == -1 || start >= end {
		t.Fatal("feedSelected watcher not found")
	}

	watcher := javascript[start:end]
	if !strings.Contains(watcher, "this.refreshItems(false)") {
		t.Error("changing feed should refresh the item list")
	}
	if strings.Contains(watcher, "this.itemSelected = null") {
		t.Error("changing feed should keep the selected item details visible")
	}
}
