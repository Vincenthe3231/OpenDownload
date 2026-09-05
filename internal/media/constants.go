// Package media contains media source classification, output naming, display,
// and format selection helpers shared by application adapters.
package media

const (
	// SourceDirect identifies a regular HTTP or HTTPS file.
	SourceDirect SourceKind = "direct"
	// SourceHLS identifies an HTTP Live Streaming playlist.
	SourceHLS SourceKind = "hls"
	// SourceDASH identifies a Dynamic Adaptive Streaming over HTTP manifest.
	SourceDASH SourceKind = "dash"
)

const (
	// SelectionBest selects the format with the largest advertised bandwidth.
	SelectionBest = "best"
	// SelectionWorst selects the format with the smallest advertised bandwidth.
	SelectionWorst = "worst"
)

const (
	fallbackFilename        = "download"
	maxFilenameLength       = 200
	hlsExtension            = ".ts"
	dashExtension           = ".mp4"
	bytesPerKiB       int64 = 1024
)
