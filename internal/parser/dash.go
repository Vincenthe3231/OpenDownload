package parser

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"

	"github.com/opendownload/opendownload/internal/media"
)

type mpdRoot struct {
	XMLName                   xml.Name    `xml:"MPD"`
	MediaPresentationDuration string      `xml:"mediaPresentationDuration,attr"`
	BaseURL                   string      `xml:"BaseURL"`
	Periods                   []mpdPeriod `xml:"Period"`
}

type mpdPeriod struct {
	ID             string             `xml:"id,attr"`
	Duration       string             `xml:"duration,attr"`
	BaseURL        string             `xml:"BaseURL"`
	AdaptationSets []mpdAdaptationSet `xml:"AdaptationSet"`
}

type mpdAdaptationSet struct {
	MimeType        string              `xml:"mimeType,attr"`
	Codecs          string              `xml:"codecs,attr"`
	Lang            string              `xml:"lang,attr"`
	BaseURL         string              `xml:"BaseURL"`
	SegmentTemplate *mpdSegmentTemplate `xml:"SegmentTemplate"`
	SegmentList     *mpdSegmentList     `xml:"SegmentList"`
	Representations []mpdRepresentation `xml:"Representation"`
}

type mpdRepresentation struct {
	ID              string              `xml:"id,attr"`
	Bandwidth       int                 `xml:"bandwidth,attr"`
	Width           int                 `xml:"width,attr"`
	Height          int                 `xml:"height,attr"`
	Codecs          string              `xml:"codecs,attr"`
	MimeType        string              `xml:"mimeType,attr"`
	BaseURL         string              `xml:"BaseURL"`
	SegmentTemplate *mpdSegmentTemplate `xml:"SegmentTemplate"`
	SegmentList     *mpdSegmentList     `xml:"SegmentList"`
	SegmentBase     *mpdSegmentBase     `xml:"SegmentBase"`
}

type mpdSegmentTemplate struct {
	Initialization         string       `xml:"initialization,attr"`
	Media                  string       `xml:"media,attr"`
	StartNumber            int64        `xml:"startNumber,attr"`
	Duration               int64        `xml:"duration,attr"`
	Timescale              int64        `xml:"timescale,attr"`
	PresentationTimeOffset int64        `xml:"presentationTimeOffset,attr"`
	Timeline               *mpdTimeline `xml:"SegmentTimeline"`
}

type mpdTimeline struct {
	Segments []mpdTimelineS `xml:"S"`
}

type mpdTimelineS struct {
	T *int64 `xml:"t,attr"`
	D int64  `xml:"d,attr"`
	R int64  `xml:"r,attr"`
}

type mpdSegmentList struct {
	Duration       int64       `xml:"duration,attr"`
	Timescale      int64       `xml:"timescale,attr"`
	Initialization *mpdSegURL  `xml:"Initialization"`
	SegmentURLs    []mpdSegURL `xml:"SegmentURL"`
}

type mpdSegURL struct {
	Media      string `xml:"media,attr"`
	SourceURL  string `xml:"sourceURL,attr"`
	MediaRange string `xml:"mediaRange,attr"`
	IndexRange string `xml:"indexRange,attr"`
}

type mpdSegmentBase struct {
	IndexRange     string   `xml:"indexRange,attr"`
	Initialization *mpdInit `xml:"Initialization"`
}

type mpdInit struct {
	Range     string `xml:"range,attr"`
	SourceURL string `xml:"sourceURL,attr"`
}

// ParseDASH parses a static DASH manifest and resolves all relative segment URLs
// against baseURL.
func ParseDASH(data []byte, baseURL string) (*DASHManifest, error) {
	var mpd mpdRoot
	if err := xml.Unmarshal(data, &mpd); err != nil {
		return nil, fmt.Errorf("parse MPD: %w", err)
	}
	if mpd.XMLName.Local != "MPD" {
		return nil, validationError("DASH manifest", "root element must be MPD")
	}

	manifest := &DASHManifest{
		BaseURL:  baseURL,
		Duration: parseDuration(mpd.MediaPresentationDuration),
	}

	mpdBaseURL := baseURL
	if mpd.BaseURL != "" {
		mpdBaseURL = media.ResolveURL(baseURL, mpd.BaseURL)
	}

	for _, sourcePeriod := range mpd.Periods {
		period := DASHPeriod{
			ID:       sourcePeriod.ID,
			Duration: parseDuration(sourcePeriod.Duration),
		}

		periodBaseURL := mpdBaseURL
		if sourcePeriod.BaseURL != "" {
			periodBaseURL = media.ResolveURL(mpdBaseURL, sourcePeriod.BaseURL)
		}
		periodDuration := effectivePeriodDuration(period.Duration, manifest.Duration, len(mpd.Periods))

		for _, sourceSet := range sourcePeriod.AdaptationSets {
			adaptationSet := DASHAdaptationSet{
				MimeType: sourceSet.MimeType,
				Codecs:   sourceSet.Codecs,
				Lang:     sourceSet.Lang,
			}

			setBaseURL := periodBaseURL
			if sourceSet.BaseURL != "" {
				setBaseURL = media.ResolveURL(periodBaseURL, sourceSet.BaseURL)
			}

			for _, sourceRepresentation := range sourceSet.Representations {
				representation := DASHRepresentation{
					ID:        sourceRepresentation.ID,
					Bandwidth: sourceRepresentation.Bandwidth,
					Width:     sourceRepresentation.Width,
					Height:    sourceRepresentation.Height,
					Codecs:    sourceRepresentation.Codecs,
					MimeType:  sourceRepresentation.MimeType,
				}

				representationBaseURL := setBaseURL
				if sourceRepresentation.BaseURL != "" {
					representationBaseURL = media.ResolveURL(setBaseURL, sourceRepresentation.BaseURL)
				}
				representation.BaseURL = representationBaseURL

				template := sourceRepresentation.SegmentTemplate
				if template == nil {
					template = sourceSet.SegmentTemplate
				}
				segmentList := sourceRepresentation.SegmentList
				if segmentList == nil {
					segmentList = sourceSet.SegmentList
				}

				var err error
				switch {
				case template != nil:
					representation.Segments, err = buildSegmentsFromTemplate(template, sourceRepresentation.ID, sourceRepresentation.Bandwidth, representationBaseURL, periodDuration)
				case segmentList != nil:
					representation.Segments = buildSegmentsFromList(segmentList, representationBaseURL)
				case sourceRepresentation.SegmentBase != nil:
					representation.Segments = []DASHSegment{{URL: representationBaseURL}}
				}
				if err != nil {
					return nil, fmt.Errorf("representation %q: %w", sourceRepresentation.ID, err)
				}

				adaptationSet.Representations = append(adaptationSet.Representations, representation)
			}

			period.AdaptationSets = append(period.AdaptationSets, adaptationSet)
		}

		manifest.Periods = append(manifest.Periods, period)
	}

	return manifest, nil
}

func effectivePeriodDuration(periodDuration, manifestDuration float64, periodCount int) float64 {
	if periodDuration > 0 {
		return periodDuration
	}
	if periodCount == 1 {
		return manifestDuration
	}
	return 0
}

func buildSegmentsFromList(segmentList *mpdSegmentList, baseURL string) []DASHSegment {
	segments := make([]DASHSegment, 0, len(segmentList.SegmentURLs)+1)
	if segmentList.Initialization != nil {
		source := segmentList.Initialization.SourceURL
		if source == "" {
			source = segmentList.Initialization.Media
		}
		if source != "" {
			segments = append(segments, DASHSegment{
				URL:   media.ResolveURL(baseURL, source),
				Range: segmentList.Initialization.MediaRange,
			})
		}
	}

	for _, segmentURL := range segmentList.SegmentURLs {
		source := segmentURL.Media
		if source == "" {
			source = segmentURL.SourceURL
		}
		segment := DASHSegment{Range: segmentURL.MediaRange}
		if source != "" {
			segment.URL = media.ResolveURL(baseURL, source)
		} else {
			segment.URL = baseURL
		}
		segments = append(segments, segment)
	}
	return segments
}

var iso8601DurationRE = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?$`)

func parseDuration(value string) float64 {
	if value == "" {
		return 0
	}
	matches := iso8601DurationRE.FindStringSubmatch(value)
	if matches == nil {
		return 0
	}
	var total float64
	if matches[1] != "" {
		hours, _ := strconv.ParseFloat(matches[1], 64)
		total += hours * 3600
	}
	if matches[2] != "" {
		minutes, _ := strconv.ParseFloat(matches[2], 64)
		total += minutes * 60
	}
	if matches[3] != "" {
		seconds, _ := strconv.ParseFloat(matches[3], 64)
		total += seconds
	}
	return total
}
