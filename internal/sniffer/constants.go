package sniffer

const (
	loopbackHost       = "127.0.0.1"
	detectorBufferSize = 100
	maxSeenMedia       = 1024
)

var ignoredURLPatterns = []string{
	"google-analytics.com",
	"googletagmanager.com",
	"facebook.com/tr",
	"doubleclick.net",
	".gif",
	".png",
	".jpg",
	".jpeg",
	".svg",
	".ico",
	".css",
	".js",
	".woff",
	".woff2",
	".ttf",
}
