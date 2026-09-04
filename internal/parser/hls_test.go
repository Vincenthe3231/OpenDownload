package parser

import (
	"testing"
)

func TestParseHLSMasterPlaylist(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1280000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2",FRAME-RATE=30
720p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2560000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2",FRAME-RATE=30
1080p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=640000,RESOLUTION=640x360,CODECS="avc1.4d401e,mp4a.40.2"
360p.m3u8
`)

	playlist, err := ParseHLS(data, "https://example.com/stream/master.m3u8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !playlist.IsMaster {
		t.Error("expected master playlist")
	}

	if len(playlist.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(playlist.Variants))
	}

	v := playlist.Variants[0]
	if v.Bandwidth != 1280000 {
		t.Errorf("expected bandwidth 1280000, got %d", v.Bandwidth)
	}
	if v.Resolution.Width != 1280 || v.Resolution.Height != 720 {
		t.Errorf("expected 1280x720, got %dx%d", v.Resolution.Width, v.Resolution.Height)
	}
	if v.FrameRate != 30 {
		t.Errorf("expected frame rate 30, got %f", v.FrameRate)
	}
	if v.URI != "https://example.com/stream/720p.m3u8" {
		t.Errorf("unexpected URI: %s", v.URI)
	}

	best := playlist.Variants[1]
	if best.Bandwidth != 2560000 {
		t.Errorf("expected highest bandwidth 2560000, got %d", best.Bandwidth)
	}
}

func TestParseHLSMediaPlaylist(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:9.009,
segment0.ts
#EXTINF:9.009,
segment1.ts
#EXTINF:9.009,
segment2.ts
#EXT-X-ENDLIST
`)

	playlist, err := ParseHLS(data, "https://example.com/stream/720p.m3u8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if playlist.IsMaster {
		t.Error("expected media playlist")
	}

	if playlist.TargetDuration != 10 {
		t.Errorf("expected target duration 10, got %f", playlist.TargetDuration)
	}

	if len(playlist.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(playlist.Segments))
	}

	if playlist.Segments[0].URI != "https://example.com/stream/segment0.ts" {
		t.Errorf("unexpected segment URI: %s", playlist.Segments[0].URI)
	}

	if playlist.Segments[0].Duration != 9.009 {
		t.Errorf("expected duration 9.009, got %f", playlist.Segments[0].Duration)
	}
}

func TestParseHLSEncryptedPlaylist(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-KEY:METHOD=AES-128,URI="key.bin",IV=0x00000000000000000000000000000001
#EXTINF:10.0,
enc_seg0.ts
#EXTINF:10.0,
enc_seg1.ts
`)

	playlist, err := ParseHLS(data, "https://example.com/stream/enc.m3u8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if playlist.Encryption == nil {
		t.Fatal("expected encryption info")
	}

	if playlist.Encryption.Method != "AES-128" {
		t.Errorf("expected AES-128, got %s", playlist.Encryption.Method)
	}

	if playlist.Encryption.URI != "https://example.com/stream/key.bin" {
		t.Errorf("unexpected key URI: %s", playlist.Encryption.URI)
	}

	if playlist.Segments[0].Encryption == nil {
		t.Error("expected segment to inherit encryption")
	}
}

func TestParseHLSWithAudioGroups(t *testing.T) {
	data := []byte(`#EXTM3U
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",DEFAULT=YES,URI="audio_en.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="Spanish",LANGUAGE="es",DEFAULT=NO,URI="audio_es.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1920x1080,AUDIO="audio"
video.m3u8
`)

	playlist, err := ParseHLS(data, "https://example.com/master.m3u8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tracks, ok := playlist.AudioGroups["audio"]
	if !ok {
		t.Fatal("expected audio group 'audio'")
	}

	if len(tracks) != 2 {
		t.Fatalf("expected 2 audio tracks, got %d", len(tracks))
	}

	if tracks[0].Language != "en" || !tracks[0].Default {
		t.Error("expected first track to be English default")
	}
}

func TestParseHLSInvalidData(t *testing.T) {
	_, err := ParseHLS([]byte("not a playlist"), "https://example.com/bad.m3u8")
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func TestParseAttributes(t *testing.T) {
	attrs := parseAttributes(`BANDWIDTH=1280000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2"`)

	if attrs["BANDWIDTH"] != "1280000" {
		t.Errorf("expected BANDWIDTH=1280000, got %s", attrs["BANDWIDTH"])
	}
	if attrs["RESOLUTION"] != "1280x720" {
		t.Errorf("expected RESOLUTION=1280x720, got %s", attrs["RESOLUTION"])
	}
	if attrs["CODECS"] != "avc1.64001f,mp4a.40.2" {
		t.Errorf("expected CODECS with comma, got %s", attrs["CODECS"])
	}
}
