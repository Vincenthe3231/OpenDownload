package downloader

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opendownload/opendownload/server/internal/parser"
	"github.com/opendownload/opendownload/server/internal/util"
)

type HLSDownloaderConfig struct {
	Workers int
	Verbose bool
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
	var completed atomic.Int64
	start := time.Now()

	type indexedSegment struct {
		index int
		data  []byte
	}

	results := make([][]byte, total)
	var mu sync.Mutex
	var downloadErr error

	sem := make(chan struct{}, d.config.Workers)
	var wg sync.WaitGroup

	for i, seg := range playlist.Segments {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, segment parser.HLSSegment) {
			defer wg.Done()
			defer func() { <-sem }()

			data, err := d.downloadSegment(ctx, segment, idx)
			if err != nil {
				mu.Lock()
				if downloadErr == nil {
					downloadErr = fmt.Errorf("segment %d: %w", idx, err)
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			results[idx] = data
			mu.Unlock()

			done := completed.Add(1)
			elapsed := time.Since(start).Seconds()
			if elapsed == 0 {
				elapsed = 0.001
			}
			pct := float64(done) / float64(total) * 100
			fmt.Printf("\r  Segments: %d/%d (%.1f%%)   ", done, total, pct)
		}(i, seg)
	}

	wg.Wait()
	fmt.Println()

	if downloadErr != nil {
		return downloadErr
	}

	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	for i, data := range results {
		if data == nil {
			return fmt.Errorf("missing data for segment %d", i)
		}
		if _, err := file.Write(data); err != nil {
			return fmt.Errorf("failed to write segment %d: %w", i, err)
		}
	}

	return nil
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

type hlsProgressWriter struct {
	total     int
	completed *atomic.Int64
	out       io.Writer
}

func (w *hlsProgressWriter) update() {
	done := w.completed.Load()
	pct := float64(done) / float64(w.total) * 100
	fmt.Fprintf(w.out, "\r  Segments: %d/%d (%.1f%%)   ", done, w.total, pct)
}
