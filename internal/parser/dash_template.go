package parser

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/opendownload/opendownload/internal/media"
)

func buildSegmentsFromTemplate(template *mpdSegmentTemplate, representationID string, bandwidth int, baseURL string, periodDuration float64) ([]DASHSegment, error) {
	if template == nil {
		return nil, validationError("DASH segment template", "is missing")
	}
	timescale, err := dashTimescale(template.Timescale)
	if err != nil {
		return nil, err
	}
	startNumber, err := dashStartNumber(template.StartNumber)
	if err != nil {
		return nil, err
	}

	segments := make([]DASHSegment, 0)
	if template.Initialization != "" {
		initializationURL := replaceTemplateVars(template.Initialization, representationID, bandwidth, startNumber, template.PresentationTimeOffset)
		segments = append(segments, DASHSegment{URL: media.ResolveURL(baseURL, initializationURL)})
	}
	if template.Media == "" {
		return nil, validationError("DASH segment template", "missing media URL template")
	}

	if template.Timeline != nil {
		mediaSegments, err := buildTimelineSegments(template, representationID, bandwidth, baseURL, startNumber, periodDuration, timescale)
		if err != nil {
			return nil, err
		}
		return append(segments, mediaSegments...), nil
	}

	if template.Duration <= 0 {
		return nil, validationError("DASH segment template", "requires a positive duration or SegmentTimeline")
	}
	segmentCount, err := dashSegmentCount(periodDuration, timescale, template.Duration)
	if err != nil {
		return nil, err
	}
	if segmentCount > maxDASHSegments {
		return nil, validationError("DASH segment template", fmt.Sprintf("expands to more than %d segments", maxDASHSegments))
	}

	segmentDuration := float64(template.Duration) / float64(timescale)
	for index := int64(0); index < segmentCount; index++ {
		number := startNumber + index
		presentationTime := template.PresentationTimeOffset + index*template.Duration
		mediaURL := replaceTemplateVars(template.Media, representationID, bandwidth, number, presentationTime)
		segments = append(segments, DASHSegment{
			URL:      media.ResolveURL(baseURL, mediaURL),
			Duration: segmentDuration,
		})
	}
	return segments, nil
}

func dashTimescale(value int64) (int64, error) {
	if value == 0 {
		return defaultDASHTimescale, nil
	}
	if value < 0 {
		return 0, validationError("DASH timescale", "must be positive")
	}
	return value, nil
}

func dashStartNumber(value int64) (int64, error) {
	if value == 0 {
		return defaultDASHStartNumber, nil
	}
	if value < 0 {
		return 0, validationError("DASH start number", "must not be negative")
	}
	return value, nil
}

func dashSegmentCount(periodDuration float64, timescale, segmentDuration int64) (int64, error) {
	if periodDuration <= 0 || math.IsNaN(periodDuration) || math.IsInf(periodDuration, 0) {
		return 0, validationError("DASH segment template", "cannot determine segment count without a period or manifest duration")
	}
	if segmentDuration <= 0 {
		return 0, validationError("DASH segment duration", "must be positive")
	}
	durationUnits := periodDuration * float64(timescale)
	if durationUnits > float64(math.MaxInt64) {
		return 0, validationError("DASH period duration", "is too large")
	}
	wholeUnits := int64(math.Ceil(durationUnits))
	if wholeUnits <= 0 {
		return 0, validationError("DASH period duration", "must be positive")
	}
	return 1 + (wholeUnits-1)/segmentDuration, nil
}

var templateIntegerFormatRE = regexp.MustCompile(`\$(Number|Time)%0?(\d+)d\$`)

func replaceTemplateVars(template, representationID string, bandwidth int, number, presentationTime int64) string {
	const escapedDollar = "\x00"
	value := strings.ReplaceAll(template, "$$", escapedDollar)
	value = strings.ReplaceAll(value, "$RepresentationID$", representationID)
	value = strings.ReplaceAll(value, "$Bandwidth$", strconv.Itoa(bandwidth))
	value = replaceTemplateInteger(value, "Number", number)
	value = replaceTemplateInteger(value, "Time", presentationTime)
	return strings.ReplaceAll(value, escapedDollar, "$")
}

func replaceTemplateInteger(value, variable string, replacement int64) string {
	value = strings.ReplaceAll(value, "$"+variable+"$", strconv.FormatInt(replacement, 10))
	return templateIntegerFormatRE.ReplaceAllStringFunc(value, func(match string) string {
		parts := templateIntegerFormatRE.FindStringSubmatch(match)
		if len(parts) != 3 || parts[1] != variable {
			return match
		}
		width, err := strconv.Atoi(parts[2])
		if err != nil || width <= 0 {
			return match
		}
		return fmt.Sprintf("%0*d", width, replacement)
	})
}
