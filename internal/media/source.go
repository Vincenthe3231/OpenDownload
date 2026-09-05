package media

import (
	"net/url"
	"path"
	"strings"
)

// SourceKind describes how a URL should be resolved before it is transferred.
type SourceKind string

// Classify returns the source kind indicated by a content type or URL path.
// Content type takes precedence because a manifest URL does not always have an
// identifying file extension.
func Classify(rawURL, contentType string) SourceKind {
	contentType = strings.ToLower(contentType)
	switch {
	case strings.Contains(contentType, "mpegurl"):
		return SourceHLS
	case strings.Contains(contentType, "dash+xml"):
		return SourceDASH
	default:
		return ClassifyURL(rawURL)
	}
}

// ClassifyURL returns the source kind indicated by the URL path. Unrecognised
// paths are treated as direct downloads.
func ClassifyURL(rawURL string) SourceKind {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return SourceDirect
	}

	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".m3u8":
		return SourceHLS
	case ".mpd":
		return SourceDASH
	default:
		return SourceDirect
	}
}

// DefaultExtension returns the final container extension for a source kind.
// Direct downloads preserve their URL filename extension.
func DefaultExtension(kind SourceKind) string {
	switch kind {
	case SourceHLS:
		return hlsExtension
	case SourceDASH:
		return dashExtension
	default:
		return ""
	}
}
