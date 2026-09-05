package parser

const (
	hlsHeaderTag           = "#EXTM3U"
	hlsMediaSequenceTag    = "#EXT-X-MEDIA-SEQUENCE:"
	hlsTargetDurationTag   = "#EXT-X-TARGETDURATION:"
	hlsStreamInfoTag       = "#EXT-X-STREAM-INF:"
	hlsMediaTag            = "#EXT-X-MEDIA:"
	hlsKeyTag              = "#EXT-X-KEY:"
	hlsInfoTag             = "#EXTINF:"
	hlsEncryptionNone      = "NONE"
	hlsEncryptionAES128    = "AES-128"
	hlsIdentityKeyFormat   = "identity"
	hlsIVByteLength        = 16
	defaultDASHTimescale   = int64(1)
	defaultDASHStartNumber = int64(1)
	maxDASHSegments        = int64(1_000_000)
)
