package extractors

import (
	"context"
)

// Extractor interface for site-specific logic
type Extractor interface {
	Name() string
	Match(url string) bool
	Extract(ctx context.Context, url string) (*ExtractionResult, error)
}

type ExtractionResult struct {
	Title     string // Add title field
	StreamURL string
	Headers   map[string]string
	Cookies   string
}

var registry []Extractor

func Register(e Extractor) {
	registry = append(registry, e)
}

func GetExtractor(url string) Extractor {
	for _, e := range registry {
		if e.Match(url) {
			return e
		}
	}
	return nil
}
