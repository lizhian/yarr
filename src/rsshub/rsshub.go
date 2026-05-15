package rsshub

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const Scheme = "rsshub"

var (
	bilibiliUIDRe       = regexp.MustCompile(`^[0-9]+$`)
	bilibiliUIDPrefixRe = regexp.MustCompile(`(?i)^uid\s*:\s*([0-9]+)$`)
	telegramIDRe        = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
)

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

func NormalizeSubscriptionInput(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw, false
	}

	if uid := parseBilibiliUIDPrefix(raw); uid != "" {
		return bilibiliUserVideoLink(uid), true
	}
	if link, ok := normalizeBilibiliSubscriptionInput(raw); ok {
		return link, true
	}
	if link, ok := normalizeTelegramSubscriptionInput(raw); ok {
		return link, true
	}

	return raw, false
}

func NormalizeBilibiliInput(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if bilibiliUIDRe.MatchString(raw) {
		return bilibiliUserVideoLink(raw), true
	}
	if uid := parseBilibiliUIDPrefix(raw); uid != "" {
		return bilibiliUserVideoLink(uid), true
	}
	return normalizeBilibiliSubscriptionInput(raw)
}

func NormalizeTelegramInput(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	id := strings.TrimPrefix(raw, "@")
	if telegramIDRe.MatchString(id) {
		return telegramChannelLink(id), true
	}
	return normalizeTelegramSubscriptionInput(raw)
}

func normalizeBilibiliSubscriptionInput(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return raw, false
	}
	if strings.ToLower(u.Hostname()) != "space.bilibili.com" {
		return raw, false
	}
	parts := pathParts(u.EscapedPath())
	if len(parts) == 0 || !bilibiliUIDRe.MatchString(parts[0]) {
		return raw, false
	}
	if len(parts) == 1 || (len(parts) == 2 && parts[1] == "dynamic") || (len(parts) == 3 && parts[1] == "upload" && parts[2] == "video") {
		return bilibiliUserVideoLink(parts[0]), true
	}
	return raw, false
}

func normalizeTelegramSubscriptionInput(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return raw, false
	}
	host := strings.ToLower(u.Hostname())
	if host != "t.me" && host != "telegram.me" {
		return raw, false
	}
	parts := pathParts(u.EscapedPath())
	if len(parts) == 1 && parts[0] != "s" && telegramIDRe.MatchString(parts[0]) && !strings.HasPrefix(parts[0], "+") {
		return telegramChannelLink(parts[0]), true
	}
	if len(parts) == 2 && parts[0] == "s" && telegramIDRe.MatchString(parts[1]) {
		return telegramChannelLink(parts[1]), true
	}
	return raw, false
}

func bilibiliUserVideoLink(uid string) string {
	return "rsshub://bilibili/user/video/" + uid
}

func parseBilibiliUIDPrefix(raw string) string {
	match := bilibiliUIDPrefixRe.FindStringSubmatch(strings.TrimSpace(raw))
	if match == nil {
		return ""
	}
	return match[1]
}

func telegramChannelLink(id string) string {
	return "rsshub://telegram/channel/" + id
}

func pathParts(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
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
