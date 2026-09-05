package media

import (
	"fmt"
	"net/url"
	"strings"
)

// FormatBytes returns a compact binary byte count for display.
func FormatBytes(bytes int64) string {
	switch {
	case bytes >= bytesPerKiB*bytesPerKiB*bytesPerKiB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(bytesPerKiB*bytesPerKiB*bytesPerKiB))
	case bytes >= bytesPerKiB*bytesPerKiB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(bytesPerKiB*bytesPerKiB))
	case bytes >= bytesPerKiB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(bytesPerKiB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatBitrate returns a compact bitrate for display.
func FormatBitrate(bitsPerSecond int) string {
	switch {
	case bitsPerSecond >= 1_000_000:
		return fmt.Sprintf("%.1f Mbps", float64(bitsPerSecond)/1_000_000)
	case bitsPerSecond >= 1_000:
		return fmt.Sprintf("%d kbps", bitsPerSecond/1_000)
	default:
		return fmt.Sprintf("%d bps", bitsPerSecond)
	}
}

// FormatDuration returns a compact hours, minutes, and seconds duration.
func FormatDuration(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}

	totalSeconds := int(seconds)
	hours := totalSeconds / 3600
	minutes := totalSeconds % 3600 / 60
	remainingSeconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, remainingSeconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, remainingSeconds)
}

// ResolveURL resolves ref against base. A malformed base or reference leaves
// the reference unchanged so callers can report the original input.
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
