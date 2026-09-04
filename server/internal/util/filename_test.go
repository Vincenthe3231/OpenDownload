package util

import (
	"testing"
)

func TestFilenameFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://example.com/video.mp4", "video.mp4"},
		{"https://example.com/path/to/file.mkv", "file.mkv"},
		{"https://example.com/", "download"},
		{"https://example.com", "download"},
		{"https://example.com/video.mp4?token=abc", "video.mp4"},
		{"https://example.com/file<name>.mp4", "file_name_.mp4"},
	}

	for _, tc := range tests {
		result := FilenameFromURL(tc.url)
		if result != tc.expected {
			t.Errorf("FilenameFromURL(%q) = %q, want %q", tc.url, result, tc.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{1536, "1.50 KB"},
	}

	for _, tc := range tests {
		result := FormatBytes(tc.bytes)
		if result != tc.expected {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.bytes, result, tc.expected)
		}
	}
}

func TestFormatBitrate(t *testing.T) {
	tests := []struct {
		bps      int
		expected string
	}{
		{500, "500 bps"},
		{128000, "128 kbps"},
		{5000000, "5.0 Mbps"},
	}

	for _, tc := range tests {
		result := FormatBitrate(tc.bps)
		if result != tc.expected {
			t.Errorf("FormatBitrate(%d) = %q, want %q", tc.bps, result, tc.expected)
		}
	}
}

func TestResolveURL(t *testing.T) {
	tests := []struct {
		base     string
		ref      string
		expected string
	}{
		{"https://example.com/stream/master.m3u8", "720p.m3u8", "https://example.com/stream/720p.m3u8"},
		{"https://example.com/stream/master.m3u8", "/abs/path.m3u8", "https://example.com/abs/path.m3u8"},
		{"https://example.com/stream/master.m3u8", "https://other.com/video.m3u8", "https://other.com/video.m3u8"},
		{"https://example.com/a/b/c.m3u8", "../d.ts", "https://example.com/a/d.ts"},
	}

	for _, tc := range tests {
		result := ResolveURL(tc.base, tc.ref)
		if result != tc.expected {
			t.Errorf("ResolveURL(%q, %q) = %q, want %q", tc.base, tc.ref, result, tc.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0, "0:00"},
		{65, "1:05"},
		{3661, "1:01:01"},
	}

	for _, tc := range tests {
		result := FormatDuration(tc.seconds)
		if result != tc.expected {
			t.Errorf("FormatDuration(%f) = %q, want %q", tc.seconds, result, tc.expected)
		}
	}
}
