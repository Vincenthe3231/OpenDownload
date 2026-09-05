package parser

import (
	"fmt"
	"math"

	"github.com/opendownload/opendownload/internal/media"
)

func buildTimelineSegments(template *mpdSegmentTemplate, representationID string, bandwidth int, baseURL string, startNumber int64, periodDuration float64, timescale int64) ([]DASHSegment, error) {
	if len(template.Timeline.Segments) == 0 {
		return nil, validationError("DASH segment timeline", "contains no segments")
	}

	segments := make([]DASHSegment, 0)
	currentTime := int64(0)
	number := startNumber
	for index, entry := range template.Timeline.Segments {
		if entry.T != nil {
			if *entry.T < 0 {
				return nil, validationError("DASH segment timeline", "contains a negative start time")
			}
			if *entry.T < currentTime {
				return nil, validationError("DASH segment timeline", "moves backward in time")
			}
			currentTime = *entry.T
		}
		if entry.D <= 0 {
			return nil, validationError("DASH segment timeline", "contains a nonpositive duration")
		}

		count, err := timelineSegmentCount(template, index, currentTime, entry.D, entry.R, periodDuration, timescale)
		if err != nil {
			return nil, err
		}
		if count > maxDASHSegments-int64(len(segments)) {
			return nil, validationError("DASH segment timeline", fmt.Sprintf("expands to more than %d segments", maxDASHSegments))
		}

		for repeat := int64(0); repeat < count; repeat++ {
			mediaURL := replaceTemplateVars(template.Media, representationID, bandwidth, number, currentTime)
			segments = append(segments, DASHSegment{
				URL:      media.ResolveURL(baseURL, mediaURL),
				Duration: float64(entry.D) / float64(timescale),
			})
			number++
			currentTime += entry.D
		}
	}
	return segments, nil
}

func timelineSegmentCount(template *mpdSegmentTemplate, index int, start, duration, repeat int64, periodDuration float64, timescale int64) (int64, error) {
	switch {
	case repeat >= 0:
		if repeat >= maxDASHSegments {
			return 0, validationError("DASH segment timeline", "repeat count is too large")
		}
		return repeat + 1, nil
	case repeat != -1:
		return 0, validationError("DASH segment timeline", "repeat count must be at least negative one")
	}

	if nextStart, found := nextExplicitTimelineStart(template.Timeline.Segments[index+1:]); found {
		if nextStart <= start {
			return 0, validationError("DASH segment timeline", "negative repeat does not advance to the next start time")
		}
		span := nextStart - start
		if span%duration != 0 {
			return 0, validationError("DASH segment timeline", "negative repeat does not align with the next start time")
		}
		return span / duration, nil
	}

	end, err := timelinePeriodEnd(template.PresentationTimeOffset, periodDuration, timescale)
	if err != nil {
		return 0, err
	}
	if end <= start {
		return 0, validationError("DASH segment timeline", "negative repeat has no bounded period end")
	}
	span := end - start
	return 1 + (span-1)/duration, nil
}

func nextExplicitTimelineStart(entries []mpdTimelineS) (int64, bool) {
	for _, entry := range entries {
		if entry.T != nil {
			return *entry.T, true
		}
	}
	return 0, false
}

func timelinePeriodEnd(presentationTimeOffset int64, duration float64, timescale int64) (int64, error) {
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0, validationError("DASH segment timeline", "negative repeat requires a period or manifest duration")
	}
	units := duration * float64(timescale)
	if units > float64(math.MaxInt64-presentationTimeOffset) {
		return 0, validationError("DASH period duration", "is too large")
	}
	return presentationTimeOffset + int64(math.Ceil(units)), nil
}
