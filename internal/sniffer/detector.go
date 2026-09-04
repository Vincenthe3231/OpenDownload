package sniffer

import (
	"net/url"
	"strings"
	"sync"
)

type MediaType string

const (
	MediaTypeVideo    MediaType = "video"
	MediaTypeAudio    MediaType = "audio"
	MediaTypeHLS      MediaType = "hls"
	MediaTypeDASH     MediaType = "dash"
	MediaTypeUnknown  MediaType = "media"
)

type DetectedMedia struct {
	URL         string
	Type        string
	ContentType string
	Size        int64
	Source      string
}

type MediaDetector struct {
	mediaCh chan DetectedMedia
	seen    sync.Map
}

func NewMediaDetector() *MediaDetector {
	return &MediaDetector{
		mediaCh: make(chan DetectedMedia, 100),
	}
}

func (d *MediaDetector) MediaChannel() <-chan DetectedMedia {
	return d.mediaCh
}

func (d *MediaDetector) Close() {
	close(d.mediaCh)
}

func (d *MediaDetector) Inspect(reqURL string, contentType string, contentLength int64) {
	if d.shouldIgnore(reqURL) {
		return
	}

	mediaType := d.detectType(reqURL, contentType)
	if mediaType == "" {
		return
	}

	if _, loaded := d.seen.LoadOrStore(reqURL, true); loaded {
		return
	}

	d.mediaCh <- DetectedMedia{
		URL:         reqURL,
		Type:        mediaType,
		ContentType: contentType,
		Size:        contentLength,
	}
}

func (d *MediaDetector) detectType(rawURL, contentType string) string {
	ct := strings.ToLower(contentType)

	if strings.HasPrefix(ct, "video/") {
		return string(MediaTypeVideo)
	}
	if strings.HasPrefix(ct, "audio/") && !strings.Contains(ct, "audio/mpegurl") {
		return string(MediaTypeAudio)
	}
	if strings.Contains(ct, "mpegurl") || strings.Contains(ct, "x-mpegurl") {
		return string(MediaTypeHLS)
	}
	if strings.Contains(ct, "dash+xml") {
		return string(MediaTypeDASH)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path := strings.ToLower(parsed.Path)

	switch {
	case strings.HasSuffix(path, ".m3u8"):
		return string(MediaTypeHLS)
	case strings.HasSuffix(path, ".mpd"):
		return string(MediaTypeDASH)
	case strings.HasSuffix(path, ".mp4"),
		strings.HasSuffix(path, ".webm"),
		strings.HasSuffix(path, ".mkv"),
		strings.HasSuffix(path, ".flv"),
		strings.HasSuffix(path, ".avi"),
		strings.HasSuffix(path, ".mov"),
		strings.HasSuffix(path, ".wmv"),
		strings.HasSuffix(path, ".ts"):
		return string(MediaTypeVideo)
	case strings.HasSuffix(path, ".mp3"),
		strings.HasSuffix(path, ".aac"),
		strings.HasSuffix(path, ".ogg"),
		strings.HasSuffix(path, ".opus"),
		strings.HasSuffix(path, ".flac"),
		strings.HasSuffix(path, ".m4a"):
		return string(MediaTypeAudio)
	}

	return ""
}

func (d *MediaDetector) shouldIgnore(rawURL string) bool {
	lower := strings.ToLower(rawURL)

	ignorePatterns := []string{
		"google-analytics.com",
		"googletagmanager.com",
		"facebook.com/tr",
		"doubleclick.net",
		".gif",
		".png",
		".jpg",
		".jpeg",
		".svg",
		".ico",
		".css",
		".js",
		".woff",
		".woff2",
		".ttf",
	}

	for _, pattern := range ignorePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
