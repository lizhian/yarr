package rsshub

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const Scheme = "rsshub"

func IsLink(link string) bool {
	u, err := url.Parse(link)
	return err == nil && u.Scheme == Scheme
}

func NormalizeBase(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("RSSHub base URL must use http or https")
	}
	if u.Host == "" {
		return "", errors.New("RSSHub base URL must include a host")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func Resolve(link, base string) (string, error) {
	if !IsLink(link) {
		return link, nil
	}
	base, err := NormalizeBase(base)
	if err != nil {
		return "", err
	}
	if base == "" {
		return "", errors.New("RSSHub base URL is not configured")
	}
	u, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", errors.New("RSSHub link must use rsshub:// route format")
	}
	path := strings.TrimLeft(u.Host+u.EscapedPath(), "/")
	if path == "" {
		return "", fmt.Errorf("RSSHub link has no route")
	}
	resolved := base + "/" + path
	if u.RawQuery != "" {
		resolved += "?" + u.RawQuery
	}
	return resolved, nil
}
