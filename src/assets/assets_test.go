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

func TestFeedRowsOnlyShowUnreadCounts(t *testing.T) {
	var output bytes.Buffer
	Render("index.html", &output, map[string]interface{}{
		"settings":      map[string]interface{}{},
		"authenticated": false,
	})

	html := output.String()
	start := strings.Index(html, `id="col-feed-list"`)
	end := strings.Index(html, `id="col-item-list"`)
	if start == -1 || end == -1 || start >= end {
		t.Fatal("feed list markup not found")
	}
	feedList := html[start:end]
	for _, want := range []string{
		`<span class="counter feed-unread-count text-right">{{ filteredFolderStats[folder.id] || '' }}</span>`,
		`<span class="counter feed-unread-count text-right">{{ filteredFeedStats[feed.id] || '' }}</span>`,
	} {
		if !strings.Contains(feedList, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, unwanted := range []string{"showFeedSettings", "showFolderSettings", "relative-time", "feed-settings-action"} {
		if strings.Contains(feedList, unwanted) {
			t.Errorf("feed and folder rows should only show unread counts: found %q", unwanted)
		}
	}
}

func TestLogoRotatesWhileFeedsRefresh(t *testing.T) {
	var output bytes.Buffer
	Render("index.html", &output, map[string]interface{}{
		"settings":      map[string]interface{}{},
		"authenticated": false,
	})

	html := output.String()
	for _, want := range []string{
		`:class="{'app-logo-refreshing': logoRefreshAnimating}"`,
		`@animationend="logoRefreshAnimating = false"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing logo animation behavior %q", want)
		}
	}

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
	for _, want := range []string{
		"@keyframes rotate",
		"animation: rotate .9s linear 1;",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("missing logo refresh animation rule %q", want)
		}
	}
	if strings.Contains(css, ".app-logo-refreshing {\n  animation: rotate .9s infinite") {
		t.Error("logo refresh animation should rotate only once per status response")
	}

	javascriptFile, err := FS.Open("javascripts/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer javascriptFile.Close()
	javascriptContent, err := io.ReadAll(javascriptFile)
	if err != nil {
		t.Fatal(err)
	}
	javascript := string(javascriptContent)
	for _, want := range []string{
		"'logoRefreshAnimating': false",
		"triggerLogoRefreshAnimation: function()",
		"this.logoRefreshAnimating = false",
		"vm.logoRefreshAnimating = true",
		"if (data.running > 0) vm.triggerLogoRefreshAnimation()",
	} {
		if !strings.Contains(javascript, want) {
			t.Errorf("missing status-triggered logo animation behavior %q", want)
		}
	}
}

func TestItemListToolbarShowsSelectionControls(t *testing.T) {
	var output bytes.Buffer
	Render("index.html", &output, map[string]interface{}{
		"settings":      map[string]interface{}{},
		"authenticated": false,
	})

	html := output.String()
	start := strings.Index(html, `id="col-item-list"`)
	end := strings.Index(html, `id="item-list-scroll"`)
	if start == -1 || end == -1 || start >= end {
		t.Fatal("item list toolbar markup not found")
	}
	toolbar := html[start:end]
	for _, want := range []string{
		`@click="showCurrentSettings()"`,
		`aria-label="未读优先"`,
		`v-if="currentLastRefreshedAt || currentLatestItemArrivedAt"`,
		`v-if="currentLastRefreshedAt"`,
		`:val="currentLastRefreshedAt"`,
		`:aria-label="'最后刷新：' + formatDate(currentLastRefreshedAt)"`,
		`v-if="currentLatestItemArrivedAt"`,
		`:val="currentLatestItemArrivedAt"`,
		`:aria-label="'最新文章入库：' + formatDate(currentLatestItemArrivedAt)"`,
		`:class="{'item-list-time-icon-success': currentLastRefreshSucceeded}"`,
		`feather-arrow-down`,
		`feather-arrow-up`,
	} {
		if !strings.Contains(toolbar, want) {
			t.Errorf("missing item list selection control %q", want)
		}
	}
	if strings.Count(toolbar, `class="icon item-list-time-icon"`) != 2 {
		t.Error("refresh and item-arrival times should each have an aria-hidden icon")
	}

	orderedControls := []string{
		`@click="showCurrentSettings()"`,
		`@click="toggleArticleListLayout()"`,
		`aria-label="未读优先"`,
		`@click="toggleAutoReadScroll()"`,
		`v-if="currentLastRefreshedAt || currentLatestItemArrivedAt"`,
		`@click="setItemOrder('sort_newest_first', !itemSortNewestFirst)"`,
		`@click="markItemsRead()"`,
	}
	previousIndex := -1
	for _, control := range orderedControls {
		index := strings.Index(toolbar, control)
		if index <= previousIndex {
			t.Fatalf("item list toolbar control is out of order: %q", control)
		}
		previousIndex = index
	}
	if strings.Contains(toolbar, `@click="showFeedList()"`) || strings.Contains(toolbar, `item-list-back`) {
		t.Error("item list toolbar should not include a mobile back-to-feeds button")
	}
	if strings.Contains(toolbar, `<span class="toolbar-label">未读优先</span>`) {
		t.Error("unread-first should use an icon instead of text")
	}

	javascriptFile, err := FS.Open("javascripts/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer javascriptFile.Close()
	content, err := io.ReadAll(javascriptFile)
	if err != nil {
		t.Fatal(err)
	}
	javascript := string(content)
	for _, want := range []string{
		"currentLatestItemArrivedAt: function()",
		"if (feed.folder_id != current.folder.id || !feed.latest_item_arrived_at) return",
		"currentLastRefreshDetail: function()",
		"return this.feedRefreshDetails[current.feed.id] || null",
		"var refreshDetails = this.feedRefreshDetails",
		"if (feed.folder_id != current.folder.id) return",
		"latestDetail = detail",
		"currentLastRefreshedAt: function()",
		"currentLastRefreshSucceeded: function()",
		"showCurrentSettings: function()",
		"this.showFeedSettings(current.feed)",
		"this.showFolderSettings(current.folder)",
	} {
		if !strings.Contains(javascript, want) {
			t.Errorf("missing current selection behavior %q", want)
		}
	}
}

func TestFeedSettingsSectionOrder(t *testing.T) {
	var output bytes.Buffer
	Render("index.html", &output, map[string]interface{}{
		"settings":      map[string]interface{}{},
		"authenticated": false,
	})

	html := output.String()
	start := strings.Index(html, `v-else-if="settings=='feed' && settingsFeed"`)
	end := strings.Index(html, `v-else-if="settings=='folder' && settingsFolder"`)
	if start == -1 || end == -1 || start >= end {
		t.Fatal("feed settings markup not found")
	}
	feedSettings := html[start:end]
	orderedContent := []string{
		`@click.prevent="refreshFeedIcon(settingsFeed)"`,
		`<header>基础设置</header>`,
		`<header>移动到</header>`,
		`<header>刷新详情</header>`,
	}
	previousIndex := -1
	for _, content := range orderedContent {
		index := strings.Index(feedSettings, content)
		if index <= previousIndex {
			t.Fatalf("feed settings content is out of order: %q", content)
		}
		previousIndex = index
	}
}

func TestFeedListToolbarDisplayScope(t *testing.T) {
	var output bytes.Buffer
	Render("index.html", &output, map[string]interface{}{
		"settings":      map[string]interface{}{},
		"authenticated": false,
	})

	html := output.String()
	for _, title := range []string{"全部", "未读", "收藏"} {
		titleIndex := strings.Index(html, `title="`+title+`"`)
		if titleIndex == -1 {
			t.Fatalf("missing %s feed filter", title)
		}
		buttonStart := strings.LastIndex(html[:titleIndex], "<button")
		buttonEndOffset := strings.Index(html[titleIndex:], "</button>")
		if buttonStart == -1 || buttonEndOffset == -1 {
			t.Fatalf("could not find %s feed filter button", title)
		}
		button := html[buttonStart : titleIndex+buttonEndOffset]
		if !strings.Contains(button, "toolbar-label") || strings.Contains(button, "toolbar-icon") {
			t.Errorf("%s feed filter should display text only", title)
		}
	}

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
	for _, want := range []string{
		".toolbar-display-icon #col-item .toolbar-label",
		".toolbar-display-text #col-item .toolbar-icon",
		".toolbar-display-text #col-item .toolbar-item-icon-only .toolbar-icon",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("toolbar display setting should target article details: missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"\n.toolbar-display-icon .toolbar-label,",
		"\n.toolbar-display-text .toolbar-item-icon-only .toolbar-icon",
	} {
		if strings.Contains(css, unwanted) {
			t.Errorf("toolbar display setting should not target the whole app: found %q", unwanted)
		}
	}
}

func TestPaneDividersUseConsistentStacking(t *testing.T) {
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
	for _, want := range []string{
		"#col-feed-list {\n  z-index: 3;\n}",
		"#col-item-list {\n  z-index: 2;\n}",
		"#col-item {\n  position: relative;\n  z-index: 1;\n}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("missing pane stacking rule %q", want)
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

func TestDefaultColumnWidthsUseRequestedRatio(t *testing.T) {
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
	if !strings.Contains(javascript, "this.feedListWidth = Math.round(appWidth / 5)") {
		t.Fatal("default feed column should use one fifth of the app width")
	}
	if !strings.Contains(javascript, "this.itemListWidth = Math.round(appWidth * 3 / 10)") {
		t.Fatal("default item column should use three tenths of the app width")
	}
}

func TestFeedSortModes(t *testing.T) {
	javascriptFile, err := FS.Open("javascripts/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer javascriptFile.Close()
	javascriptContent, err := io.ReadAll(javascriptFile)
	if err != nil {
		t.Fatal(err)
	}
	javascript := string(javascriptContent)
	for _, want := range []string{
		"{name: 'name', title: '名称'}",
		"{name: 'time', title: '时间'}",
		"{name: 'count', title: '数量'}",
		"if (timeA != timeB) return timeB - timeA",
		"if (countA != countB) return countB - countA",
		"return compareFeedNames(a, b)",
	} {
		if !strings.Contains(javascript, want) {
			t.Fatalf("missing feed sort behavior %q", want)
		}
	}

	templateFile, err := FS.Open("index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer templateFile.Close()
	templateContent, err := io.ReadAll(templateFile)
	if err != nil {
		t.Fatal(err)
	}
	template := string(templateContent)
	if !strings.Contains(template, "<header>订阅源排序</header>") {
		t.Fatal("feed sort control is missing")
	}
	if strings.Contains(template, "feedSort == 'time' && feed.latest_item_arrived_at") {
		t.Fatal("feed sorting should not control the item-list latest arrival time")
	}
}
