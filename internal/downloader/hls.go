package downloader

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/transport"
)

type HLSDownloaderConfig struct {
	Workers    int
	Verbose    bool
	OnProgress ProgressCallback
}

type HLSDownloader struct {
	client   *transport.HTTPClient
	config   HLSDownloaderConfig
	keyCache sync.Map
}

// NewHLSDownloader creates an HLS transfer engine. The caller owns output
// publication and must supply an uncreated work file path to Download.
func NewHLSDownloader(client *transport.HTTPClient, config HLSDownloaderConfig) *HLSDownloader {
	if config.Workers <= 0 {
		config.Workers = 8
	}
	return &HLSDownloader{client: client, config: config}
}


// Download transfers playlist segments into the exact caller supplied work path.
// It never renames or removes a separate final destination.
func (d *HLSDownloader) Download(ctx context.Context, playlist *parser.HLSPlaylist, outPath string) error {
	if playlist == nil {
		return validationError("HLS playlist", "must not be nil")
	}
	total := len(playlist.Segments)
	reporter := newProgressReporter(d.config.OnProgress)
	defer reporter.finish()
	reporter.segments(0, 0, int64(total), true)

	var completed, downloaded int64
	return downloadOrderedSegments(ctx, total, d.config.Workers, outPath, func(fetchCtx context.Context, index int) ([]byte, error) {
		data, err := d.downloadSegment(fetchCtx, playlist.Segments[index])
		if err != nil {
			return nil, fmt.Errorf("segment %d: %w", index, err)
		}
		return data, nil
	}, func(bytes int) {
		completed++
		downloaded += int64(bytes)
		reporter.segments(downloaded, completed, int64(total), completed == int64(total))
	})
}

func (d *HLSDownloader) downloadSegment(ctx context.Context, seg parser.HLSSegment) ([]byte, error) {
	data, err := d.client.Get(ctx, seg.URI)
	if err != nil {
		return nil, transportError("fetch HLS segment", err)
	}

	if seg.Encryption != nil {
		if !strings.EqualFold(seg.Encryption.Method, "AES-128") {
			return nil, validationError("HLS encryption", fmt.Sprintf("unsupported method %q", seg.Encryption.Method))
		}
		data, err = d.decryptSegment(ctx, data, seg.Encryption, seg.Sequence)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt segment: %w", err)
		}
	}

	return data, nil
}

func (d *HLSDownloader) decryptSegment(ctx context.Context, data []byte, enc *parser.HLSEncryption, sequence int64) ([]byte, error) {
	if enc == nil {
		return nil, validationError("HLS encryption", "must not be nil")
	}
	key, err := d.fetchKey(ctx, enc.URI)
	if err != nil {
		return nil, fmt.Errorf("fetch key: %w", err)
	}
	if len(key) != aes.BlockSize {
		return nil, validationError("HLS AES 128 key", "must be exactly 16 bytes")
	}

	iv, err := hlsInitializationVector(enc.IV, sequence)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, validationError("HLS ciphertext", "must be a nonempty multiple of the AES block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, data)

	return pkcs7Unpad(data)
}

func (d *HLSDownloader) fetchKey(ctx context.Context, uri string) ([]byte, error) {
	if uri == "" {
		return nil, validationError("HLS key URI", "must not be empty")
	}
	if cached, ok := d.keyCache.Load(uri); ok {
		return cached.([]byte), nil
	}

	key, err := d.client.Get(ctx, uri)
	if err != nil {
		return nil, transportError("fetch HLS key", err)
	}

	d.keyCache.Store(uri, key)
	return key, nil
}

func hlsInitializationVector(encoded string, sequence int64) ([]byte, error) {
	if encoded == "" {
		if sequence < 0 {
			return nil, validationError("HLS media sequence", "must not be negative")
		}
		iv := make([]byte, aes.BlockSize)
		binary.BigEndian.PutUint64(iv[aes.BlockSize-8:], uint64(sequence))
		return iv, nil
	}

	value := strings.TrimPrefix(strings.TrimPrefix(encoded, "0x"), "0X")
	iv, err := hex.DecodeString(value)
	if err != nil {
		return nil, validationError("HLS encryption IV", "must contain valid hexadecimal bytes")
	}
	if len(iv) != aes.BlockSize {
		return nil, validationError("HLS encryption IV", "must be exactly 16 bytes")
	}
	return iv, nil
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, validationError("HLS ciphertext padding", "is missing")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > len(data) || padLen > aes.BlockSize {
		return nil, validationError("HLS ciphertext padding", "is invalid")
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, validationError("HLS ciphertext padding", "is invalid")
		}
	}
	return data[:len(data)-padLen], nil
}
