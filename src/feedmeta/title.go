package feedmeta

import "strings"

var titleSuffixes = []string{
	" 的 bilibili 动态",
	" 的 bilibili 空间",
	" - Telegram Channel",
}

func CleanTitle(title string) string {
	title = strings.TrimSpace(title)
	for _, suffix := range titleSuffixes {
		if strings.HasSuffix(title, suffix) {
			return strings.TrimSpace(strings.TrimSuffix(title, suffix))
		}
	}
	return title
}
