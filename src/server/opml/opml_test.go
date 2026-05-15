package opml

import (
	"reflect"
	"testing"
)

func TestOPML(t *testing.T) {
	have := (Folder{
		Title: "",
		Feeds: []Feed{
			{
				Title:   "title1",
				FeedUrl: "https://baz.com/feed.xml",
				SiteUrl: "https://baz.com/",
			},
		},
		Folders: []Folder{
			{
				Title: "sub",
				Feeds: []Feed{
					{
						Title:           "subtitle1",
						FeedUrl:         "https://foo.com/feed.xml",
						SiteUrl:         "https://foo.com/",
						ContentSelector: `main .content > a[href="https://example.com/?a=1&b=2"]`,
						IconURL:         "https://foo.com/icon.png?a=1&b=2",
					},
					{
						Title:   "&>",
						FeedUrl: "https://bar.com/feed.xml",
						SiteUrl: "https://bar.com/",
					},
				},
				Folders: []Folder{},
			},
		},
	}).OPML()
	want := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.1">
<head><title>subscriptions</title></head>
<body>
  <outline text="sub">
    <outline type="rss" text="subtitle1" xmlUrl="https://foo.com/feed.xml" htmlUrl="https://foo.com/" icon_url="https://foo.com/icon.png?a=1&amp;b=2" content_selector="main .content &gt; a[href=&#34;https://example.com/?a=1&amp;b=2&#34;]"/>
    <outline type="rss" text="&amp;&gt;" xmlUrl="https://bar.com/feed.xml" htmlUrl="https://bar.com/"/>
  </outline>
  <outline type="rss" text="title1" xmlUrl="https://baz.com/feed.xml" htmlUrl="https://baz.com/"/>
</body>
</opml>
`
	if !reflect.DeepEqual(want, have) {
		t.Logf("want: %s", want)
		t.Logf("have: %s", have)
		t.Fatal("invalid opml")
	}
}
