package htmlutil

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	nodeNameRegex    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$|^\*$`)
	identifierRegex  = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	simpleSelectorRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*|\*)?([.#])?([A-Za-z0-9_-]+)?$`)
)

func FindNodes(node *html.Node, match func(*html.Node) bool) []*html.Node {
	nodes := make([]*html.Node, 0)

	queue := make([]*html.Node, 0)
	queue = append(queue, node)
	for len(queue) > 0 {
		var n *html.Node
		n, queue = queue[0], queue[1:]
		if match(n) {
			nodes = append(nodes, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			queue = append(queue, c)
		}
	}
	return nodes
}

func Query(node *html.Node, sel string) []*html.Node {
	matcher := NewMatcher(sel)
	return FindNodes(node, matcher.Match)
}

func QueryWithError(node *html.Node, sel string) ([]*html.Node, error) {
	matcher, err := CompileSelector(sel)
	if err != nil {
		return nil, err
	}
	return FindNodes(node, matcher.Match), nil
}

func Closest(node *html.Node, sel string) *html.Node {
	matcher := NewMatcher(sel)
	for cur := node; cur != nil; cur = cur.Parent {
		if matcher.Match(cur) {
			return cur
		}
	}
	return nil
}

func NewMatcher(sel string) Matcher {
	matcher, err := CompileSelector(sel)
	if err != nil {
		panic(err)
	}
	return matcher
}

func CompileSelector(sel string) (Matcher, error) {
	multi := MultiMatch{}
	parts := strings.Split(sel, ",")
	for _, part := range parts {
		part := strings.TrimSpace(part)
		matcher, err := parseSimpleSelector(part)
		if err != nil {
			return nil, err
		}
		multi.Add(matcher)
	}
	if len(multi.matchers) == 0 {
		return nil, fmt.Errorf("empty selector")
	}
	return multi, nil
}

type Matcher interface {
	Match(*html.Node) bool
}

type ElementMatch struct {
	Name  string
	Class string
	ID    string
}

func (m ElementMatch) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if m.Name != "" && m.Name != "*" && n.Data != m.Name {
		return false
	}
	if m.ID != "" && Attr(n, "id") != m.ID {
		return false
	}
	if m.Class != "" && !hasClass(n, m.Class) {
		return false
	}
	return true
}

type MultiMatch struct {
	matchers []Matcher
}

func (m *MultiMatch) Add(matcher Matcher) {
	m.matchers = append(m.matchers, matcher)
}

func (m MultiMatch) Match(n *html.Node) bool {
	for _, matcher := range m.matchers {
		if matcher.Match(n) {
			return true
		}
	}
	return false
}

func parseSimpleSelector(sel string) (Matcher, error) {
	if sel == "" {
		return nil, fmt.Errorf("empty selector")
	}
	if nodeNameRegex.MatchString(sel) {
		return ElementMatch{Name: strings.ToLower(sel)}, nil
	}

	matches := simpleSelectorRe.FindStringSubmatch(sel)
	if len(matches) == 0 || matches[2] == "" || matches[3] == "" || !identifierRegex.MatchString(matches[3]) {
		return nil, fmt.Errorf("unsupported selector: %s", sel)
	}

	match := ElementMatch{Name: strings.ToLower(matches[1])}
	if matches[2] == "." {
		match.Class = matches[3]
	} else {
		match.ID = matches[3]
	}
	return match, nil
}

func hasClass(node *html.Node, className string) bool {
	for _, class := range strings.Fields(Attr(node, "class")) {
		if class == className {
			return true
		}
	}
	return false
}
