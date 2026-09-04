package parser

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/opendownload/opendownload/internal/util"
)

type DASHManifest struct {
	BaseURL  string
	Periods  []DASHPeriod
	Duration float64
}

type DASHPeriod struct {
	ID             string
	Duration       float64
	AdaptationSets []DASHAdaptationSet
}

type DASHAdaptationSet struct {
	MimeType        string
	Codecs          string
	Lang            string
	Representations []DASHRepresentation
}

type DASHRepresentation struct {
	ID         string
	Bandwidth  int
	Width      int
	Height     int
	Codecs     string
	MimeType   string
	Segments   []DASHSegment
	BaseURL    string
}

type DASHSegment struct {
	URL      string
	Duration float64
	Range    string
}

type mpdRoot struct {
	XMLName             xml.Name   `xml:"MPD"`
	MediaPresentationDuration string `xml:"mediaPresentationDuration,attr"`
	BaseURL             string     `xml:"BaseURL"`
	Periods             []mpdPeriod `xml:"Period"`
}

type mpdPeriod struct {
	ID             string            `xml:"id,attr"`
	Duration       string            `xml:"duration,attr"`
	BaseURL        string            `xml:"BaseURL"`
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
	Initialization string           `xml:"initialization,attr"`
	Media          string           `xml:"media,attr"`
	StartNumber    int              `xml:"startNumber,attr"`
	Duration       int              `xml:"duration,attr"`
	Timescale      int              `xml:"timescale,attr"`
	Timeline       *mpdTimeline     `xml:"SegmentTimeline"`
}

type mpdTimeline struct {
	Segments []mpdTimelineS `xml:"S"`
}

type mpdTimelineS struct {
	T int `xml:"t,attr"`
	D int `xml:"d,attr"`
	R int `xml:"r,attr"`
}

type mpdSegmentList struct {
	Duration       int              `xml:"duration,attr"`
	Timescale      int              `xml:"timescale,attr"`
	Initialization *mpdSegURL       `xml:"Initialization"`
	SegmentURLs    []mpdSegURL      `xml:"SegmentURL"`
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
	Range    string `xml:"range,attr"`
	SourceURL string `xml:"sourceURL,attr"`
}

func ParseDASH(data []byte, baseURL string) (*DASHManifest, error) {
	var mpd mpdRoot
	if err := xml.Unmarshal(data, &mpd); err != nil {
		return nil, fmt.Errorf("failed to parse MPD: %w", err)
	}

	manifest := &DASHManifest{
		BaseURL:  baseURL,
		Duration: parseDuration(mpd.MediaPresentationDuration),
	}

	mpdBaseURL := baseURL
	if mpd.BaseURL != "" {
		mpdBaseURL = util.ResolveURL(baseURL, mpd.BaseURL)
	}

	for _, p := range mpd.Periods {
		period := DASHPeriod{
			ID:       p.ID,
			Duration: parseDuration(p.Duration),
		}

		periodBaseURL := mpdBaseURL
		if p.BaseURL != "" {
			periodBaseURL = util.ResolveURL(mpdBaseURL, p.BaseURL)
		}

		for _, as := range p.AdaptationSets {
			adaptSet := DASHAdaptationSet{
				MimeType: as.MimeType,
				Codecs:   as.Codecs,
				Lang:     as.Lang,
			}

			asBaseURL := periodBaseURL
			if as.BaseURL != "" {
				asBaseURL = util.ResolveURL(periodBaseURL, as.BaseURL)
			}

			for _, r := range as.Representations {
				rep := DASHRepresentation{
					ID:        r.ID,
					Bandwidth: r.Bandwidth,
					Width:     r.Width,
					Height:    r.Height,
					Codecs:    r.Codecs,
					MimeType:  r.MimeType,
				}

				repBaseURL := asBaseURL
				if r.BaseURL != "" {
					repBaseURL = util.ResolveURL(asBaseURL, r.BaseURL)
				}
				rep.BaseURL = repBaseURL

				tmpl := r.SegmentTemplate
				if tmpl == nil {
					tmpl = as.SegmentTemplate
				}

				segList := r.SegmentList
				if segList == nil {
					segList = as.SegmentList
				}

				if tmpl != nil {
					rep.Segments = buildSegmentsFromTemplate(tmpl, r.ID, r.Bandwidth, repBaseURL)
				} else if segList != nil {
					rep.Segments = buildSegmentsFromList(segList, repBaseURL)
				} else if r.SegmentBase != nil {
					rep.Segments = []DASHSegment{{URL: repBaseURL}}
				}

				adaptSet.Representations = append(adaptSet.Representations, rep)
			}

			period.AdaptationSets = append(period.AdaptationSets, adaptSet)
		}

		manifest.Periods = append(manifest.Periods, period)
	}

	return manifest, nil
}

func buildSegmentsFromTemplate(tmpl *mpdSegmentTemplate, repID string, bw int, baseURL string) []DASHSegment {
	var segments []DASHSegment

	if tmpl.Initialization != "" {
		initURL := replaceTemplateVars(tmpl.Initialization, repID, bw, 0, 0)
		segments = append(segments, DASHSegment{
			URL: util.ResolveURL(baseURL, initURL),
		})
	}

	timescale := tmpl.Timescale
	if timescale == 0 {
		timescale = 1
	}

	if tmpl.Timeline != nil {
		currentTime := 0
		for _, s := range tmpl.Timeline.Segments {
			if s.T > 0 {
				currentTime = s.T
			}
			repeat := s.R + 1
			for i := 0; i < repeat; i++ {
				mediaURL := replaceTemplateVars(tmpl.Media, repID, bw, len(segments), currentTime)
				segments = append(segments, DASHSegment{
					URL:      util.ResolveURL(baseURL, mediaURL),
					Duration: float64(s.D) / float64(timescale),
				})
				currentTime += s.D
			}
		}
	} else if tmpl.Duration > 0 {
		segDuration := float64(tmpl.Duration) / float64(timescale)
		num := tmpl.StartNumber

		totalSegments := 100
		for i := 0; i < totalSegments; i++ {
			mediaURL := replaceTemplateVars(tmpl.Media, repID, bw, num, num*tmpl.Duration)
			segments = append(segments, DASHSegment{
				URL:      util.ResolveURL(baseURL, mediaURL),
				Duration: segDuration,
			})
			num++
		}
	}

	return segments
}

func buildSegmentsFromList(segList *mpdSegmentList, baseURL string) []DASHSegment {
	var segments []DASHSegment

	if segList.Initialization != nil {
		src := segList.Initialization.SourceURL
		if src == "" {
			src = segList.Initialization.Media
		}
		if src != "" {
			segments = append(segments, DASHSegment{
				URL:   util.ResolveURL(baseURL, src),
				Range: segList.Initialization.MediaRange,
			})
		}
	}

	for _, su := range segList.SegmentURLs {
		src := su.Media
		if src == "" {
			src = su.SourceURL
		}
		seg := DASHSegment{
			Range: su.MediaRange,
		}
		if src != "" {
			seg.URL = util.ResolveURL(baseURL, src)
		} else {
			seg.URL = baseURL
		}
		segments = append(segments, seg)
	}

	return segments
}

func replaceTemplateVars(tmpl, repID string, bw, number, time int) string {
	s := tmpl
	s = strings.ReplaceAll(s, "$RepresentationID$", repID)
	s = strings.ReplaceAll(s, "$Bandwidth$", strconv.Itoa(bw))

	s = replaceNumberVar(s, "$Number$", number)
	s = replaceNumberVar(s, "$Time$", time)

	return s
}

var numberFormatRe = regexp.MustCompile(`\$(Number|Time)%(\d+)d\$`)

func replaceNumberVar(s, simple string, val int) string {
	s = strings.ReplaceAll(s, simple, strconv.Itoa(val))
	s = numberFormatRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := numberFormatRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		width, _ := strconv.Atoi(sub[2])
		return fmt.Sprintf("%0*d", width, val)
	})
	return s
}

var iso8601Re = regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?`)

func parseDuration(s string) float64 {
	if s == "" {
		return 0
	}
	matches := iso8601Re.FindStringSubmatch(s)
	if matches == nil {
		return 0
	}
	var total float64
	if matches[1] != "" {
		h, _ := strconv.ParseFloat(matches[1], 64)
		total += h * 3600
	}
	if matches[2] != "" {
		m, _ := strconv.ParseFloat(matches[2], 64)
		total += m * 60
	}
	if matches[3] != "" {
		sec, _ := strconv.ParseFloat(matches[3], 64)
		total += sec
	}
	return total
}
