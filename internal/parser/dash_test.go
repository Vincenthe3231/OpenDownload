package parser

import (
	"testing"
)

func TestParseDASHManifest(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<MPD mediaPresentationDuration="PT1H30M0S">
  <Period id="1" duration="PT1H30M0S">
    <AdaptationSet mimeType="video/mp4" codecs="avc1.640028" lang="en">
      <SegmentTemplate initialization="init-$RepresentationID$.m4s" media="seg-$RepresentationID$-$Number$.m4s" startNumber="1" duration="4000" timescale="1000"/>
      <Representation id="1" bandwidth="5000000" width="1920" height="1080"/>
      <Representation id="2" bandwidth="2500000" width="1280" height="720"/>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4" codecs="mp4a.40.2" lang="en">
      <SegmentTemplate initialization="init-audio-$RepresentationID$.m4s" media="seg-audio-$RepresentationID$-$Number$.m4s" startNumber="1" duration="4000" timescale="1000"/>
      <Representation id="audio1" bandwidth="128000"/>
    </AdaptationSet>
  </Period>
</MPD>`)

	manifest, err := ParseDASH(data, "https://example.com/dash/manifest.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manifest.Duration != 5400 {
		t.Errorf("expected duration 5400, got %f", manifest.Duration)
	}

	if len(manifest.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(manifest.Periods))
	}

	period := manifest.Periods[0]
	if len(period.AdaptationSets) != 2 {
		t.Fatalf("expected 2 adaptation sets, got %d", len(period.AdaptationSets))
	}

	videoAS := period.AdaptationSets[0]
	if videoAS.MimeType != "video/mp4" {
		t.Errorf("expected video/mp4, got %s", videoAS.MimeType)
	}

	if len(videoAS.Representations) != 2 {
		t.Fatalf("expected 2 video representations, got %d", len(videoAS.Representations))
	}

	rep1080 := videoAS.Representations[0]
	if rep1080.Width != 1920 || rep1080.Height != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", rep1080.Width, rep1080.Height)
	}
	if rep1080.Bandwidth != 5000000 {
		t.Errorf("expected bandwidth 5000000, got %d", rep1080.Bandwidth)
	}

	if len(rep1080.Segments) == 0 {
		t.Error("expected segments to be generated from template")
	}

	if rep1080.Segments[0].URL != "https://example.com/dash/init-1.m4s" {
		t.Errorf("unexpected init segment URL: %s", rep1080.Segments[0].URL)
	}
}

func TestParseDASHWithTimeline(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<MPD mediaPresentationDuration="PT10S">
  <Period>
    <AdaptationSet mimeType="video/mp4">
      <SegmentTemplate initialization="init.m4s" media="seg-$Time$.m4s" timescale="1000">
        <SegmentTimeline>
          <S t="0" d="2000" r="4"/>
        </SegmentTimeline>
      </SegmentTemplate>
      <Representation id="v1" bandwidth="1000000" width="1280" height="720"/>
    </AdaptationSet>
  </Period>
</MPD>`)

	manifest, err := ParseDASH(data, "https://example.com/timeline.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reps := manifest.Periods[0].AdaptationSets[0].Representations
	if len(reps) != 1 {
		t.Fatalf("expected 1 representation, got %d", len(reps))
	}

	segs := reps[0].Segments
	if len(segs) != 6 {
		t.Fatalf("expected 6 segments (1 init + 5 media), got %d", len(segs))
	}

	if segs[0].URL != "https://example.com/init.m4s" {
		t.Errorf("unexpected init URL: %s", segs[0].URL)
	}
}

func TestParseDASHWithSegmentList(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<MPD>
  <Period>
    <AdaptationSet mimeType="video/mp4">
      <Representation id="1" bandwidth="2000000" width="1280" height="720">
        <SegmentList>
          <Initialization sourceURL="init.mp4"/>
          <SegmentURL media="seg1.m4s"/>
          <SegmentURL media="seg2.m4s"/>
          <SegmentURL media="seg3.m4s"/>
        </SegmentList>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`)

	manifest, err := ParseDASH(data, "https://example.com/seglist.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rep := manifest.Periods[0].AdaptationSets[0].Representations[0]
	if len(rep.Segments) != 4 {
		t.Fatalf("expected 4 segments (1 init + 3), got %d", len(rep.Segments))
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"PT1H30M0S", 5400},
		{"PT10S", 10},
		{"PT1M30S", 90},
		{"PT2H", 7200},
		{"PT0.5S", 0.5},
		{"", 0},
	}

	for _, tc := range tests {
		result := parseDuration(tc.input)
		if result != tc.expected {
			t.Errorf("parseDuration(%q) = %f, want %f", tc.input, result, tc.expected)
		}
	}
}

func TestParseDASHInvalidXML(t *testing.T) {
	_, err := ParseDASH([]byte("not xml"), "https://example.com/bad.mpd")
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}
