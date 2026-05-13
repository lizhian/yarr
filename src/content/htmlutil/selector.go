package htmlutil

import (
	"strings"

	"golang.org/x/net/html"
)

func InnerHTMLBySelector(input, selector string) (string, bool, error) {
	root, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return "", false, err
	}
	nodes, err := QueryWithError(root, selector)
	if err != nil {
		return "", false, err
	}
	if len(nodes) == 0 {
		return "", false, nil
	}
	return InnerHTML(nodes[0]), true, nil
}
