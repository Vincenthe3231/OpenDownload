package parser

// Resolution describes the pixel dimensions advertised by an HLS variant.
type Resolution struct {
	Width  int
	Height int
}

// HLSEncryption describes the key material required for an encrypted HLS segment.
// AES 128 is the only encrypted method currently supported.
type HLSEncryption struct {
	Method string
	URI    string
	IV     string
}

// HLSSegment is one ordered media segment in an HLS media playlist.
type HLSSegment struct {
	URI        string
	Duration   float64
	Title      string
	ByteRange  *ByteRange
	Encryption *HLSEncryption
	Sequence   int64
}

// ByteRange identifies a byte interval in a media resource.
type ByteRange struct {
	Length int64
	Offset int64
}

// HLSVariant is one rendition declared by an HLS master playlist.
type HLSVariant struct {
	URI        string
	Bandwidth  int
	Resolution Resolution
	Codecs     string
	FrameRate  float64
	Audio      string
	Subtitles  string
}

// HLSAudioTrack is an alternate audio rendition in an HLS master playlist.
type HLSAudioTrack struct {
	GroupID  string
	Name     string
	Language string
	URI      string
	Default  bool
}

// HLSPlaylist is a parsed HLS master or media playlist.
type HLSPlaylist struct {
	IsMaster       bool
	Variants       []HLSVariant
	AudioGroups    map[string][]HLSAudioTrack
	Segments       []HLSSegment
	TargetDuration float64
	MediaSequence  int64
	Encryption     *HLSEncryption
	BaseURL        string
}

// DASHManifest is a parsed MPEG DASH manifest.
type DASHManifest struct {
	BaseURL  string
	Periods  []DASHPeriod
	Duration float64
}

// DASHPeriod groups media adaptation sets that share a presentation period.
type DASHPeriod struct {
	ID             string
	Duration       float64
	AdaptationSets []DASHAdaptationSet
}

// DASHAdaptationSet groups alternative representations of the same media type.
type DASHAdaptationSet struct {
	MimeType        string
	Codecs          string
	Lang            string
	Representations []DASHRepresentation
}

// DASHRepresentation describes a downloadable audio or video rendition.
type DASHRepresentation struct {
	ID        string
	Bandwidth int
	Width     int
	Height    int
	Codecs    string
	MimeType  string
	Segments  []DASHSegment
	BaseURL   string
}

// DASHSegment is one ordered resource or byte range in a DASH representation.
type DASHSegment struct {
	URL      string
	Duration float64
	Range    string
}
