package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var defaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
}

type HTTPClientConfig struct {
	UserAgent  string
	ProxyURL   string
	Verbose    bool
	Timeout    time.Duration
	MaxRetries int
	Headers    map[string]string
	Cookie     string
	CookieFile string
	Limiter    *ConnectionLimiter
}

// ConnectionLimiter caps simultaneous HTTP response bodies across clients.
// A nil limiter leaves the client unconstrained for standalone CLI use.
type ConnectionLimiter struct {
	slots  chan struct{}
	active atomic.Int64
}

func NewConnectionLimiter(limit int) *ConnectionLimiter {
	if limit <= 0 {
		return nil
	}
	return &ConnectionLimiter{slots: make(chan struct{}, limit)}
}

func (l *ConnectionLimiter) acquire(ctx context.Context) error {
	if l == nil {
		return nil
	}
	select {
	case l.slots <- struct{}{}:
		l.active.Add(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *ConnectionLimiter) release() {
	if l == nil {
		return
	}
	<-l.slots
	l.active.Add(-1)
}

func (l *ConnectionLimiter) Active() int64 {
	if l == nil {
		return 0
	}
	return l.active.Load()
}

type limitedReadCloser struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *limitedReadCloser) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}

type HTTPClient struct {
	client *http.Client
	config HTTPClientConfig
}

func NewHTTPClient(config HTTPClientConfig) *HTTPClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: false},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if config.ProxyURL != "" {
		proxyURL, err := url.Parse(config.ProxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &HTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
			Jar:       jar,
		},
		config: config,
	}
}

func (c *HTTPClient) userAgent() string {
	if c.config.UserAgent != "" {
		return c.config.UserAgent
	}
	return defaultUserAgents[rand.Intn(len(defaultUserAgents))]
}

func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent())
	}
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}
	if c.config.Cookie != "" {
		req.Header.Set("Cookie", c.config.Cookie)
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-time.After(backoff):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		if err = c.config.Limiter.acquire(req.Context()); err != nil {
			return nil, err
		}
		resp, err = c.client.Do(req)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
			c.config.Limiter.release()
			resp = nil
		}
		if err == nil && resp.StatusCode < 500 {
			if c.config.Limiter != nil {
				resp.Body = &limitedReadCloser{ReadCloser: resp.Body, release: c.config.Limiter.release}
			}
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
			if c.config.Limiter != nil {
				c.config.Limiter.release()
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", c.config.MaxRetries, err)
	}
	return resp, nil
}

func (c *HTTPClient) Get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func (c *HTTPClient) Head(ctx context.Context, rawURL string) (size int64, resumable bool, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, false, "", err
	}

	resp, err := c.Do(req)
	if err != nil {
		return 0, false, "", err
	}
	defer resp.Body.Close()

	contentType = resp.Header.Get("Content-Type")
	acceptRanges := resp.Header.Get("Accept-Ranges")
	resumable = acceptRanges == "bytes"

	cl := resp.Header.Get("Content-Length")
	if cl != "" {
		size, _ = strconv.ParseInt(cl, 10, 64)
	}

	return size, resumable, contentType, nil
}

func (c *HTTPClient) GetRange(ctx context.Context, rawURL string, start, end int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	if end >= start {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	} else {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
	}

	return c.Do(req)
}

func (c *HTTPClient) GetBody(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}
