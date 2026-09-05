package parser

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/opendownload/opendownload/internal/media"
)

// ParseHLS parses a master or media playlist and resolves all relative URLs
// against baseURL.
func ParseHLS(data []byte, baseURL string) (*HLSPlaylist, error) {
	content := string(data)
	if !strings.Contains(content, hlsHeaderTag) {
		return nil, validationError("HLS playlist", "missing #EXTM3U header")
	}

	playlist := &HLSPlaylist{
		BaseURL:     baseURL,
		AudioGroups: make(map[string][]HLSAudioTrack),
	}

	if strings.Contains(content, hlsStreamInfoTag) {
		playlist.IsMaster = true
		if err := parseMasterPlaylist(playlist, content, baseURL); err != nil {
			return nil, err
		}
	} else if err := parseMediaPlaylist(playlist, content, baseURL); err != nil {
		return nil, err
	}

	return playlist, nil
}

func parseMasterPlaylist(playlist *HLSPlaylist, content, baseURL string) error {
	scanner := newHLSScanner(content)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case strings.HasPrefix(line, hlsStreamInfoTag):
			attrs := parseAttributes(strings.TrimPrefix(line, hlsStreamInfoTag))
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

			if !scanner.Scan() {
				return validationError("HLS variant", "missing playlist URI")
			}
			uri := strings.TrimSpace(scanner.Text())
			if uri == "" || strings.HasPrefix(uri, "#") {
				return validationError("HLS variant", "missing playlist URI")
			}
			variant.URI = media.ResolveURL(baseURL, uri)
			playlist.Variants = append(playlist.Variants, variant)

		case strings.HasPrefix(line, hlsMediaTag):
			attrs := parseAttributes(strings.TrimPrefix(line, hlsMediaTag))
			if attrs["TYPE"] != "AUDIO" {
				continue
			}
			track := HLSAudioTrack{
				GroupID:  attrs["GROUP-ID"],
				Name:     attrs["NAME"],
				Language: attrs["LANGUAGE"],
				Default:  attrs["DEFAULT"] == "YES",
			}
			if uri, ok := attrs["URI"]; ok {
				track.URI = media.ResolveURL(baseURL, uri)
			}
			playlist.AudioGroups[track.GroupID] = append(playlist.AudioGroups[track.GroupID], track)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read HLS master playlist: %w", err)
	}
	return nil
}

func parseMediaPlaylist(playlist *HLSPlaylist, content, baseURL string) error {
	scanner := newHLSScanner(content)
	var currentEncryption *HLSEncryption
	var segDuration float64
	var segTitle string
	var sequence int64

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case strings.HasPrefix(line, hlsTargetDurationTag):
			value := strings.TrimSpace(strings.TrimPrefix(line, hlsTargetDurationTag))
			duration, err := strconv.ParseFloat(value, 64)
			if err != nil || duration < 0 {
				return validationError("HLS target duration", "must be a nonnegative number")
			}
			playlist.TargetDuration = duration

		case strings.HasPrefix(line, hlsMediaSequenceTag):
			value := strings.TrimSpace(strings.TrimPrefix(line, hlsMediaSequenceTag))
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed < 0 {
				return validationError("HLS media sequence", "must be a nonnegative integer")
			}
			sequence = parsed
			playlist.MediaSequence = parsed

		case strings.HasPrefix(line, hlsKeyTag):
			enc, err := parseHLSEncryption(strings.TrimPrefix(line, hlsKeyTag), baseURL)
			if err != nil {
				return err
			}
			currentEncryption = enc
			if playlist.Encryption == nil && enc != nil {
				playlist.Encryption = enc
			}

		case strings.HasPrefix(line, hlsInfoTag):
			value := strings.TrimPrefix(line, hlsInfoTag)
			parts := strings.SplitN(value, ",", 2)
			duration, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			if err != nil || duration < 0 {
				return validationError("HLS segment duration", "must be a nonnegative number")
			}
			segDuration = duration
			segTitle = ""
			if len(parts) > 1 {
				segTitle = strings.TrimSpace(parts[1])
			}

		case line != "" && !strings.HasPrefix(line, "#"):
			playlist.Segments = append(playlist.Segments, HLSSegment{
				URI:        media.ResolveURL(baseURL, line),
				Duration:   segDuration,
				Title:      segTitle,
				Encryption: currentEncryption,
				Sequence:   sequence,
			})
			sequence++
			segDuration = 0
			segTitle = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read HLS media playlist: %w", err)
	}
	return nil
}

func parseHLSEncryption(value, baseURL string) (*HLSEncryption, error) {
	attrs := parseAttributes(value)
	method := strings.ToUpper(strings.TrimSpace(attrs["METHOD"]))
	switch method {
	case hlsEncryptionNone:
		return nil, nil
	case hlsEncryptionAES128:
	default:
		if method == "" {
			return nil, validationError("HLS encryption", "missing METHOD")
		}
		return nil, validationError("HLS encryption", fmt.Sprintf("unsupported method %q", method))
	}

	if keyFormat, ok := attrs["KEYFORMAT"]; ok && !strings.EqualFold(keyFormat, hlsIdentityKeyFormat) {
		return nil, validationError("HLS encryption", fmt.Sprintf("unsupported key format %q", keyFormat))
	}
	keyURI := strings.TrimSpace(attrs["URI"])
	if keyURI == "" {
		return nil, validationError("HLS encryption", "AES 128 requires a key URI")
	}
	iv := strings.TrimSpace(attrs["IV"])
	if err := validateHLSIV(iv); err != nil {
		return nil, err
	}
	return &HLSEncryption{
		Method: method,
		URI:    media.ResolveURL(baseURL, keyURI),
		IV:     iv,
	}, nil
}

func validateHLSIV(iv string) error {
	if iv == "" {
		return nil
	}
	if !strings.HasPrefix(iv, "0x") && !strings.HasPrefix(iv, "0X") {
		return validationError("HLS encryption IV", "must use a 0x hexadecimal prefix")
	}
	encoded := iv[2:]
	if len(encoded) != hlsIVByteLength*2 {
		return validationError("HLS encryption IV", "must be exactly 16 bytes")
	}
	if _, err := hex.DecodeString(encoded); err != nil {
		return validationError("HLS encryption IV", "must contain valid hexadecimal bytes")
	}
	return nil
}

func newHLSScanner(content string) *bufio.Scanner {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 4*1024), 1024*1024)
	return scanner
}
