package downloader

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/util"
)

type HLSDownloaderConfig struct {
	Workers    int
	Verbose    bool
	OnProgress ProgressCallback
}

type HLSDownloader struct {
	client   *util.HTTPClient
	config   HLSDownloaderConfig
	keyCache sync.Map
}

func NewHLSDownloader(client *util.HTTPClient, config HLSDownloaderConfig) *HLSDownloader {
	if config.Workers <= 0 {
		config.Workers = 8
	}
	return &HLSDownloader{client: client, config: config}
}

func (d *HLSDownloader) Download(ctx context.Context, playlist *parser.HLSPlaylist, outPath string) error {
	total := len(playlist.Segments)
	reporter := newProgressReporter(d.config.OnProgress)
	defer reporter.finish()
	reporter.segments(0, 0, int64(total), true)

	var completed, downloaded int64
	return downloadOrderedSegments(ctx, total, d.config.Workers, outPath, func(fetchCtx context.Context, index int) ([]byte, error) {
		data, err := d.downloadSegment(fetchCtx, playlist.Segments[index], index)
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

func (d *HLSDownloader) downloadSegment(ctx context.Context, seg parser.HLSSegment, index int) ([]byte, error) {
	data, err := d.client.Get(ctx, seg.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch segment: %w", err)
	}

	if seg.Encryption != nil && seg.Encryption.Method == "AES-128" {
		data, err = d.decryptSegment(ctx, data, seg.Encryption, index)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt segment: %w", err)
		}
	}

	return data, nil
}

func (d *HLSDownloader) decryptSegment(ctx context.Context, data []byte, enc *parser.HLSEncryption, index int) ([]byte, error) {
	key, err := d.fetchKey(ctx, enc.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch key: %w", err)
	}

	var iv []byte
	if enc.IV != "" {
		ivStr := strings.TrimPrefix(enc.IV, "0x")
		ivStr = strings.TrimPrefix(ivStr, "0X")
		iv, err = hex.DecodeString(ivStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode IV: %w", err)
		}
	} else {
		iv = make([]byte, 16)
		iv[15] = byte(index)
		iv[14] = byte(index >> 8)
		iv[13] = byte(index >> 16)
		iv[12] = byte(index >> 24)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, data)

	data = pkcs7Unpad(data)
	return data, nil
}

func (d *HLSDownloader) fetchKey(ctx context.Context, uri string) ([]byte, error) {
	if cached, ok := d.keyCache.Load(uri); ok {
		return cached.([]byte), nil
	}

	key, err := d.client.Get(ctx, uri)
	if err != nil {
		return nil, err
	}

	d.keyCache.Store(uri, key)
	return key, nil
}

func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padLen := int(data[len(data)-1])
	if padLen > len(data) || padLen > aes.BlockSize {
		return data
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return data
		}
	}
	return data[:len(data)-padLen]
}
