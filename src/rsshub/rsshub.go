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

func ValidateLink(link string) error {
	u, err := url.Parse(link)
	if err != nil {
		return err
	}
	if u.Scheme != Scheme {
		return nil
	}
	if u.Host == "" {
		return errors.New("RSSHub link must use rsshub:// route format")
	}
	path := strings.TrimLeft(u.Host+u.EscapedPath(), "/")
	if path == "" {
		return fmt.Errorf("RSSHub link has no route")
	}
	return nil
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

type Base struct {
	URL      string
	Disabled bool
}

func ParseBaseList(raw string) ([]Base, error) {
	result := make([]Base, 0)
	seen := make(map[string]bool)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		disabled := false
		if strings.HasPrefix(line, "#") {
			disabled = true
			line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
		base, err := NormalizeBase(line)
		if err != nil {
			return nil, err
		}
		if base == "" {
			return nil, errors.New("RSSHub base URL must not be empty")
		}
		if seen[base] {
			continue
		}
		seen[base] = true
		result = append(result, Base{URL: base, Disabled: disabled})
	}
	return result, nil
}

func NormalizeBaseList(raw string) (string, error) {
	bases, err := ParseBaseList(raw)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(bases))
	for _, base := range bases {
		line := base.URL
		if base.Disabled {
			line = "#" + line
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}

func EnabledBases(raw string) ([]string, error) {
	bases, err := ParseBaseList(raw)
	if err != nil {
		return nil, err
	}
	enabled := make([]string, 0, len(bases))
	for _, base := range bases {
		if !base.Disabled {
			enabled = append(enabled, base.URL)
		}
	}
	return enabled, nil
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

func ResolveWithBases(link string, bases []string) ([]string, error) {
	if !IsLink(link) {
		return []string{link}, nil
	}
	if len(bases) == 0 {
		return nil, errors.New("RSSHub base URL is not configured")
	}
	result := make([]string, 0, len(bases))
	for _, base := range bases {
		resolved, err := Resolve(link, base)
		if err != nil {
			return nil, err
		}
		result = append(result, resolved)
	}
	return result, nil
}

func ResolveWithBaseList(link, rawBases string, limit int) ([]string, error) {
	if !IsLink(link) {
		return []string{link}, nil
	}
	bases, err := EnabledBases(rawBases)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(bases) > limit {
		bases = bases[:limit]
	}
	return ResolveWithBases(link, bases)
}
