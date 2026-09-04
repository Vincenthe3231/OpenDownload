package parser

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/opendownload/opendownload/server/internal/util"
)

type Resolution struct {
	Width  int
	Height int
}

type HLSEncryption struct {
	Method string
	URI    string
	IV     string
}

type HLSSegment struct {
	URI        string
	Duration   float64
	Title      string
	ByteRange  *ByteRange
	Encryption *HLSEncryption
}

type ByteRange struct {
	Length int64
	Offset int64
}

type HLSVariant struct {
	URI        string
	Bandwidth  int
	Resolution Resolution
	Codecs     string
	FrameRate  float64
	Audio      string
	Subtitles  string
}

type HLSAudioTrack struct {
	GroupID  string
	Name     string
	Language string
	URI      string
	Default  bool
}

type HLSPlaylist struct {
	IsMaster       bool
	Variants       []HLSVariant
	AudioGroups    map[string][]HLSAudioTrack
	Segments       []HLSSegment
	TargetDuration float64
	Encryption     *HLSEncryption
	BaseURL        string
}

func ParseHLS(data []byte, baseURL string) (*HLSPlaylist, error) {
	content := string(data)
	if !strings.Contains(content, "#EXTM3U") {
		return nil, fmt.Errorf("not a valid M3U8 playlist")
	}

	playlist := &HLSPlaylist{
		BaseURL:     baseURL,
		AudioGroups: make(map[string][]HLSAudioTrack),
	}

	if strings.Contains(content, "#EXT-X-STREAM-INF") {
		playlist.IsMaster = true
		parseMasterPlaylist(playlist, content, baseURL)
	} else {
		parseMediaPlaylist(playlist, content, baseURL)
	}

	return playlist, nil
}

func parseMasterPlaylist(playlist *HLSPlaylist, content, baseURL string) {
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			attrs := parseAttributes(strings.TrimPrefix(line, "#EXT-X-STREAM-INF:"))
			variant := HLSVariant{}

			if bw, ok := attrs["BANDWIDTH"]; ok {
				variant.Bandwidth, _ = strconv.Atoi(bw)
			}
			if res, ok := attrs["RESOLUTION"]; ok {
				variant.Resolution = parseResolution(res)
			}
			if codecs, ok := attrs["CODECS"]; ok {
				variant.Codecs = codecs
			}
			if fps, ok := attrs["FRAME-RATE"]; ok {
				variant.FrameRate, _ = strconv.ParseFloat(fps, 64)
			}
			if audio, ok := attrs["AUDIO"]; ok {
				variant.Audio = audio
			}
			if subs, ok := attrs["SUBTITLES"]; ok {
				variant.Subtitles = subs
			}

			if scanner.Scan() {
				uri := strings.TrimSpace(scanner.Text())
				if uri != "" && !strings.HasPrefix(uri, "#") {
					variant.URI = util.ResolveURL(baseURL, uri)
				}
			}

			playlist.Variants = append(playlist.Variants, variant)
		}

		if strings.HasPrefix(line, "#EXT-X-MEDIA:") {
			attrs := parseAttributes(strings.TrimPrefix(line, "#EXT-X-MEDIA:"))
			if attrs["TYPE"] == "AUDIO" {
				track := HLSAudioTrack{
					GroupID:  attrs["GROUP-ID"],
					Name:     attrs["NAME"],
					Language: attrs["LANGUAGE"],
					Default:  attrs["DEFAULT"] == "YES",
				}
				if uri, ok := attrs["URI"]; ok {
					track.URI = util.ResolveURL(baseURL, uri)
				}
				playlist.AudioGroups[track.GroupID] = append(playlist.AudioGroups[track.GroupID], track)
			}
		}
	}
}

func parseMediaPlaylist(playlist *HLSPlaylist, content, baseURL string) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var currentEncryption *HLSEncryption
	var segDuration float64
	var segTitle string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			val := strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:")
			playlist.TargetDuration, _ = strconv.ParseFloat(val, 64)
		}

		if strings.HasPrefix(line, "#EXT-X-KEY:") {
			attrs := parseAttributes(strings.TrimPrefix(line, "#EXT-X-KEY:"))
			method := attrs["METHOD"]
			if method == "NONE" {
				currentEncryption = nil
			} else {
				enc := &HLSEncryption{
					Method: method,
					IV:     attrs["IV"],
				}
				if uri, ok := attrs["URI"]; ok {
					enc.URI = util.ResolveURL(baseURL, uri)
				}
				currentEncryption = enc
				if playlist.Encryption == nil {
					playlist.Encryption = enc
				}
			}
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			val := strings.TrimPrefix(line, "#EXTINF:")
			parts := strings.SplitN(val, ",", 2)
			segDuration, _ = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			if len(parts) > 1 {
				segTitle = strings.TrimSpace(parts[1])
			}
		}

		if line != "" && !strings.HasPrefix(line, "#") {
			seg := HLSSegment{
				URI:        util.ResolveURL(baseURL, line),
				Duration:   segDuration,
				Title:      segTitle,
				Encryption: currentEncryption,
			}
			playlist.Segments = append(playlist.Segments, seg)
			segDuration = 0
			segTitle = ""
		}
	}
}

func parseAttributes(s string) map[string]string {
	attrs := make(map[string]string)
	var key, value string
	inQuote := false
	state := 0 // 0=key, 1=value

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '=' && state == 0:
			state = 1
		case ch == '"':
			inQuote = !inQuote
		case ch == ',' && !inQuote:
			attrs[strings.TrimSpace(key)] = strings.TrimSpace(value)
			key = ""
			value = ""
			state = 0
		case state == 0:
			key += string(ch)
		case state == 1:
			value += string(ch)
		}
	}
	if key != "" {
		attrs[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return attrs
}

func parseResolution(s string) Resolution {
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return Resolution{}
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return Resolution{Width: w, Height: h}
}
