package media

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

var unsafeFilenameCharacters = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// FilenameFromURL returns a filesystem safe filename derived from a URL.
func FilenameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fallbackFilename
	}
	return SanitizeFilename(path.Base(parsed.Path))
}

// SanitizeFilename replaces unsafe filename characters and applies the shared
// maximum filename length. Empty names use the fallback download name.
func SanitizeFilename(name string) string {
	name = unsafeFilenameCharacters.ReplaceAllString(name, "_")
	if name == "" || name == "." || name == "/" {
		return fallbackFilename
	}

	if len(name) <= maxFilenameLength {
		return name
	}

	extension := path.Ext(name)
	if len(extension) >= maxFilenameLength {
		return name[:maxFilenameLength]
	}
	return name[:maxFilenameLength-len(extension)] + extension
}

// WithExtension replaces a filename extension with extension. The extension
// may be supplied with or without its leading period.
func WithExtension(name, extension string) string {
	name = SanitizeFilename(name)
	if extension == "" {
		return name
	}
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}

	stem := strings.TrimSuffix(name, path.Ext(name))
	if stem == "" {
		stem = fallbackFilename
	}
	return SanitizeFilename(stem + extension)
}

// OutputFilename returns the filename that should be reserved before a source
// is transferred. Playlist and manifest names are converted to their final
// container extensions before reservation.
func OutputFilename(rawURL string, kind SourceKind) string {
	filename := FilenameFromURL(rawURL)
	if extension := DefaultExtension(kind); extension != "" {
		return WithExtension(filename, extension)
	}
	return filename
}

// HostFromURL returns the hostname in rawURL, or an empty string when it is not
// a valid URL.
func HostFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// GetHostFromURL preserves the existing helper name while callers migrate to
// HostFromURL.
func GetHostFromURL(rawURL string) string {
	return HostFromURL(rawURL)
}
