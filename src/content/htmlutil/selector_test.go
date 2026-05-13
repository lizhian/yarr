package htmlutil

import "testing"

func TestInnerHTMLBySelector(t *testing.T) {
	content, found, err := InnerHTMLBySelector(`
		<html><body>
			<article class="content"><h1>Title</h1><p>Body</p></article>
			<article class="content"><p>Other</p></article>
		</body></html>
	`, ".content")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected match")
	}
	if content != `<h1>Title</h1><p>Body</p>` {
		t.Fatalf("got %q", content)
	}
}

func TestInnerHTMLBySelectorNoMatch(t *testing.T) {
	_, found, err := InnerHTMLBySelector(`<html><body><p>Body</p></body></html>`, ".missing")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("unexpected match")
	}
}
