package util

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var unsafeChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

func FilenameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "download"
	}

	name := path.Base(parsed.Path)
	if name == "" || name == "." || name == "/" {
		name = "download"
	}

	if idx := strings.Index(name, "?"); idx != -1 {
		name = name[:idx]
	}

	name = unsafeChars.ReplaceAllString(name, "_")

	if len(name) > 200 {
		ext := path.Ext(name)
		name = name[:200-len(ext)] + ext
	}

	return name
}

func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func FormatBitrate(bps int) string {
	if bps >= 1_000_000 {
		return fmt.Sprintf("%.1f Mbps", float64(bps)/1_000_000)
	}
	if bps >= 1000 {
		return fmt.Sprintf("%d kbps", bps/1000)
	}
	return fmt.Sprintf("%d bps", bps)
}

func ResolveURL(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}

	return baseURL.ResolveReference(refURL).String()
}

func FormatDuration(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
