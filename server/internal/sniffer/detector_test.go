package sniffer

import (
	"testing"
)

func TestMediaDetectorContentType(t *testing.T) {
	d := NewMediaDetector()

	tests := []struct {
		url         string
		contentType string
		shouldMatch bool
		mediaType   string
	}{
		{"https://example.com/video.mp4", "video/mp4", true, "video"},
		{"https://example.com/stream.m3u8", "application/x-mpegURL", true, "hls"},
		{"https://example.com/manifest.mpd", "application/dash+xml", true, "dash"},
		{"https://example.com/audio.mp3", "audio/mpeg", true, "audio"},
		{"https://example.com/page.html", "text/html", false, ""},
		{"https://example.com/style.css", "text/css", false, ""},
	}

	for _, tc := range tests {
		mediaType := d.detectType(tc.url, tc.contentType)
		if tc.shouldMatch && mediaType == "" {
			t.Errorf("expected match for %s (%s)", tc.url, tc.contentType)
		}
		if !tc.shouldMatch && mediaType != "" {
			t.Errorf("expected no match for %s (%s), got %s", tc.url, tc.contentType, mediaType)
		}
		if tc.shouldMatch && mediaType != tc.mediaType {
			t.Errorf("expected type %s for %s, got %s", tc.mediaType, tc.url, mediaType)
		}
	}
}

func TestMediaDetectorURLPattern(t *testing.T) {
	d := NewMediaDetector()

	tests := []struct {
		url         string
		shouldMatch bool
	}{
		{"https://cdn.example.com/video.mp4", true},
		{"https://cdn.example.com/stream.m3u8", true},
		{"https://cdn.example.com/manifest.mpd", true},
		{"https://cdn.example.com/audio.mp3", true},
		{"https://cdn.example.com/song.flac", true},
		{"https://cdn.example.com/clip.webm", true},
		{"https://cdn.example.com/page.html", false},
		{"https://cdn.example.com/script.js", false},
		{"https://cdn.example.com/data.json", false},
	}

	for _, tc := range tests {
		mediaType := d.detectType(tc.url, "")
		if tc.shouldMatch && mediaType == "" {
			t.Errorf("expected URL pattern match for %s", tc.url)
		}
		if !tc.shouldMatch && mediaType != "" {
			t.Errorf("expected no match for %s, got %s", tc.url, mediaType)
		}
	}
}

func TestMediaDetectorDedup(t *testing.T) {
	d := NewMediaDetector()

	d.Inspect("https://example.com/video.mp4", "video/mp4", 1000)
	d.Inspect("https://example.com/video.mp4", "video/mp4", 1000)

	count := 0
	for {
		select {
		case <-d.MediaChannel():
			count++
		default:
			if count != 1 {
				t.Errorf("expected 1 detection, got %d", count)
			}
			return
		}
	}
}

func TestMediaDetectorIgnorePatterns(t *testing.T) {
	d := NewMediaDetector()

	if !d.shouldIgnore("https://www.google-analytics.com/analytics.js") {
		t.Error("should ignore google analytics")
	}
	if !d.shouldIgnore("https://example.com/icon.png") {
		t.Error("should ignore .png")
	}
	if d.shouldIgnore("https://cdn.example.com/video.mp4") {
		t.Error("should not ignore video.mp4")
	}
}
