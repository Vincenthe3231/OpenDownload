package sniffer

import (
	"net/url"
	"strings"
	"sync"
)

// MediaType classifies a detected downloadable media request.
type MediaType string

const (
	MediaTypeVideo   MediaType = "video"
	MediaTypeAudio   MediaType = "audio"
	MediaTypeHLS     MediaType = "hls"
	MediaTypeDASH    MediaType = "dash"
	MediaTypeUnknown MediaType = "media"
)

// DetectedMedia contains public request metadata emitted by the proxy detector.
type DetectedMedia struct {
	URL         string
	Type        string
	ContentType string
	Size        int64
	Source      string
}

// MediaDetector deduplicates recent media without blocking proxy requests.
type MediaDetector struct {
	mediaCh chan DetectedMedia
	mu      sync.Mutex
	seen    map[string]struct{}
	order   []string
	closed  bool
}

// NewMediaDetector creates a detector with bounded memory and event buffering.
func NewMediaDetector() *MediaDetector {
	return &MediaDetector{
		mediaCh: make(chan DetectedMedia, detectorBufferSize),
		seen:    make(map[string]struct{}),
	}
}

// MediaChannel returns the asynchronous stream of detected media.
func (d *MediaDetector) MediaChannel() <-chan DetectedMedia {
	return d.mediaCh
}

// Close closes the event stream once.
func (d *MediaDetector) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.closed = true
	close(d.mediaCh)
}

// Inspect classifies a response and queues it when capacity is available.
func (d *MediaDetector) Inspect(reqURL string, contentType string, contentLength int64) {
	if d.shouldIgnore(reqURL) {
		return
	}
	mediaType := d.detectType(reqURL, contentType)
	if mediaType == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	if _, exists := d.seen[reqURL]; exists {
		return
	}
	media := DetectedMedia{
		URL:         reqURL,
		Type:        mediaType,
		ContentType: contentType,
		Size:        contentLength,
	}
	select {
	case d.mediaCh <- media:
		d.rememberLocked(reqURL)
	default:
		return
	}
}

func (d *MediaDetector) rememberLocked(rawURL string) {
	if len(d.order) == maxSeenMedia {
		oldest := d.order[0]
		d.order = d.order[1:]
		delete(d.seen, oldest)
	}
	d.seen[rawURL] = struct{}{}
	d.order = append(d.order, rawURL)
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
	for _, pattern := range ignoredURLPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
