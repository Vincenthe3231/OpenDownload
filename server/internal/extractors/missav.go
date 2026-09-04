package extractors

import (
	"context"
	"strings"
)

type MissAVExtractor struct{}

func (e *MissAVExtractor) Name() string { return "missav.ws" }

func (e *MissAVExtractor) Match(url string) bool {
	return strings.Contains(url, "missav") || strings.Contains(url, "surrit")
}

func (e *MissAVExtractor) Extract(ctx context.Context, url string) (*ExtractionResult, error) {
	parts := strings.Split(url, "/")
	title := "missav_video"
	if len(parts) > 3 {
		title = "missav_" + parts[3]
	}

	return &ExtractionResult{
		Title:     title,
		StreamURL: url,
		Headers: map[string]string{
			"Referer": "https://missav.ws/",
			"Origin":  "https://missav.ws/",
		},
	}, nil
}

func init() {
	Register(&MissAVExtractor{})
}
