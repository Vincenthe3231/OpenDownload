package capture

import "time"

const (
	pairingLifetime    = 5 * time.Minute
	maxPayloadBytes    = 64 << 10
	maxCapturedStreams = 100
	captureRoute       = "/v1/firefox/streams"
	sessionHeader      = "OpenDownloadSession"
)

var allowedHeaders = map[string]string{
	"accept":          "Accept",
	"accept-language": "Accept-Language",
	"authorization":   "Authorization",
	"cookie":          "Cookie",
	"origin":          "Origin",
	"referer":         "Referer",
	"user-agent":      "User-Agent",
}
