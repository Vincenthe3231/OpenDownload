package media

import (
	"testing"
	"time"
)

func TestClassifyURL(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		expected SourceKind
	}{
		{name: "HLS path", rawURL: "https://example.test/master.M3U8?token=abc", expected: SourceHLS},
		{name: "DASH path", rawURL: "https://example.test/manifest.mpd", expected: SourceDASH},
		{name: "direct path", rawURL: "https://example.test/movie.mp4", expected: SourceDirect},
		{name: "malformed URL", rawURL: "://broken", expected: SourceDirect},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := ClassifyURL(test.rawURL); actual != test.expected {
				t.Fatalf("ClassifyURL(%q) = %q, want %q", test.rawURL, actual, test.expected)
			}
		})
	}
}

func TestClassifyContentTypeTakesPrecedence(t *testing.T) {
	if actual := Classify("https://example.test/stream", "application/vnd.apple.mpegurl"); actual != SourceHLS {
		t.Fatalf("Classify() = %q, want %q", actual, SourceHLS)
	}
}

func TestOutputFilename(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		kind     SourceKind
		expected string
	}{
		{name: "direct", rawURL: "https://example.test/movie.mp4", kind: SourceDirect, expected: "movie.mp4"},
		{name: "HLS", rawURL: "https://example.test/movie.m3u8", kind: SourceHLS, expected: "movie.ts"},
		{name: "DASH without extension", rawURL: "https://example.test/manifest", kind: SourceDASH, expected: "manifest.mp4"},
		{name: "unsafe name", rawURL: "https://example.test/file<name>.m3u8", kind: SourceHLS, expected: "file_name_.ts"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := OutputFilename(test.rawURL, test.kind); actual != test.expected {
				t.Fatalf("OutputFilename(%q, %q) = %q, want %q", test.rawURL, test.kind, actual, test.expected)
			}
		})
	}
}

func TestTimestampedOutputFilename(t *testing.T) {
	downloadedAt := time.Date(2026, time.September, 6, 3, 23, 1, 123000000, time.Local)
	tests := []struct {
		name     string
		rawURL   string
		kind     SourceKind
		expected string
	}{
		{name: "direct", rawURL: "https://example.test/movie.mp4", kind: SourceDirect, expected: "movie-download_20260906_032301_123.mp4"},
		{name: "HLS", rawURL: "https://example.test/video.m3u8", kind: SourceHLS, expected: "video-download_20260906_032301_123.ts"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := TimestampedOutputFilename(test.rawURL, test.kind, downloadedAt); actual != test.expected {
				t.Fatalf("TimestampedOutputFilename() = %q, want %q", actual, test.expected)
			}
		})
	}
}

func TestDisplayHelpers(t *testing.T) {
	if actual := FormatBytes(1536); actual != "1.50 KB" {
		t.Fatalf("FormatBytes() = %q, want %q", actual, "1.50 KB")
	}
	if actual := FormatBitrate(5_000_000); actual != "5.0 Mbps" {
		t.Fatalf("FormatBitrate() = %q, want %q", actual, "5.0 Mbps")
	}
	if actual := FormatDuration(3661); actual != "1:01:01" {
		t.Fatalf("FormatDuration() = %q, want %q", actual, "1:01:01")
	}
	if actual := ResolveURL("https://example.test/stream/master.m3u8", "720p.m3u8"); actual != "https://example.test/stream/720p.m3u8" {
		t.Fatalf("ResolveURL() = %q, want resolved URL", actual)
	}
}

func TestSelectFormat(t *testing.T) {
	formats := []Format{
		{ID: "144p", Bandwidth: 100_000},
		{ID: "720p", Bandwidth: 1_000_000},
		{ID: "1080p", Bandwidth: 3_000_000},
	}
	tests := []struct {
		name      string
		selection string
		expected  string
		found     bool
	}{
		{name: "best", selection: SelectionBest, expected: "1080p", found: true},
		{name: "worst", selection: SelectionWorst, expected: "144p", found: true},
		{name: "format ID", selection: "720p", expected: "720p", found: true},
		{name: "unknown ID", selection: "4k", found: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, found := SelectFormat(formats, test.selection)
			if found != test.found {
				t.Fatalf("SelectFormat(%q) found = %v, want %v", test.selection, found, test.found)
			}
			if found && actual.ID != test.expected {
				t.Fatalf("SelectFormat(%q) = %q, want %q", test.selection, actual.ID, test.expected)
			}
		})
	}
}
