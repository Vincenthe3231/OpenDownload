package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"time"
)

// HTTPClientConfig configures a reusable media transfer client.
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

// HTTPClient applies common headers, retries transient failures, and limits
// concurrently open response bodies.
type HTTPClient struct {
	client    *http.Client
	config    HTTPClientConfig
	configErr error
}

// NewHTTPClient constructs an HTTP transfer client. Configuration errors are
// returned by its first request because this compatibility constructor does
// not return an error itself.
func NewHTTPClient(config HTTPClientConfig) *HTTPClient {
	config = normalizedConfig(config)
	defaultJar, err := cookiejar.New(nil)
	if err != nil {
		return &HTTPClient{config: config, configErr: fmt.Errorf("create cookie jar: %w", err)}
	}
	var jar http.CookieJar = defaultJar

	if config.CookieFile != "" {
		loadedJar, cookieErr := loadNetscapeCookieJar(config.CookieFile)
		if cookieErr != nil {
			return &HTTPClient{config: config, configErr: cookieErr}
		}
		jar = loadedJar
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: dialKeepAlive,
		}).DialContext,
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnsPerHost,
		IdleConnTimeout:     defaultIdleConnectionAge,
		TLSHandshakeTimeout: tlsHandshakeTimeout,
	}

	if config.ProxyURL != "" {
		proxyURL, parseErr := url.Parse(config.ProxyURL)
		if parseErr != nil {
			return &HTTPClient{config: config, configErr: fmt.Errorf("parse proxy URL: %w", parseErr)}
		}
		transport.Proxy = http.ProxyURL(proxyURL)
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

func normalizedConfig(config HTTPClientConfig) HTTPClientConfig {
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = defaultMaxRetries
	}
	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}
	if config.Headers != nil {
		headers := make(map[string]string, len(config.Headers))
		for key, value := range config.Headers {
			headers[key] = value
		}
		config.Headers = headers
	}
	return config
}

func (client *HTTPClient) userAgent() string {
	if client.config.UserAgent != "" {
		return client.config.UserAgent
	}
	return defaultUserAgents[rand.Intn(len(defaultUserAgents))]
}

// Do sends req with the configured request policy. Callers must close every
// returned response body to release a configured connection limiter slot.
func (client *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	if client == nil {
		return nil, errors.New("HTTP client is nil")
	}
	if client.configErr != nil {
		return nil, client.configErr
	}
	if req == nil {
		return nil, errors.New("HTTP request is nil")
	}

	request := req.Clone(req.Context())
	request.Header = req.Header.Clone()
	client.applyHeaders(request)

	attempts := 1
	if canRetry(request) {
		attempts += client.config.MaxRetries
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := waitForRetry(request.Context(), attempt); err != nil {
				return nil, err
			}
		}

		attemptRequest, err := requestForAttempt(request, attempt)
		if err != nil {
			return nil, err
		}
		response, err := client.doOnce(attemptRequest)
		if err != nil {
			lastErr = err
			continue
		}
		if response.StatusCode < http.StatusInternalServerError || attempt == attempts-1 {
			return response, nil
		}

		if err := response.Body.Close(); err != nil {
			lastErr = fmt.Errorf("close transient response body: %w", err)
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", attempts-1, lastErr)
	}
	return nil, fmt.Errorf("request failed after %d retries", attempts-1)
}

func (client *HTTPClient) doOnce(request *http.Request) (*http.Response, error) {
	if err := client.config.Limiter.acquire(request.Context()); err != nil {
		return nil, err
	}

	response, err := client.client.Do(request)
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		client.config.Limiter.release()
		return nil, err
	}
	if response == nil || response.Body == nil {
		client.config.Limiter.release()
		return nil, errors.New("HTTP client returned an empty response")
	}

	if client.config.Limiter != nil {
		response.Body = &limitedReadCloser{ReadCloser: response.Body, release: client.config.Limiter.release}
	}
	return response, nil
}

func (client *HTTPClient) applyHeaders(request *http.Request) {
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", client.userAgent())
	}
	for key, value := range client.config.Headers {
		request.Header.Set(key, value)
	}
	if client.config.Cookie != "" {
		request.Header.Set("Cookie", client.config.Cookie)
	}
}

func canRetry(request *http.Request) bool {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		return false
	}
	return request.Body == nil || request.GetBody != nil
}

func requestForAttempt(request *http.Request, attemptNumber int) (*http.Request, error) {
	attempt := request.Clone(request.Context())
	attempt.Header = request.Header.Clone()
	if attemptNumber == 0 || request.Body == nil || request.GetBody == nil {
		return attempt, nil
	}

	body, err := request.GetBody()
	if err != nil {
		return nil, fmt.Errorf("reopen request body for retry: %w", err)
	}
	attempt.Body = body
	return attempt, nil
}

func waitForRetry(ctx context.Context, attempt int) error {
	delay := initialRetryBackoff
	for retry := 1; retry < attempt && delay < maximumRetryBackoff; retry++ {
		delay *= 2
	}
	if delay > maximumRetryBackoff {
		delay = maximumRetryBackoff
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Get fetches a complete successful response body.
func (client *HTTPClient) Get(ctx context.Context, rawURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create GET request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, statusError(response)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read HTTP response body: %w", err)
	}
	return body, nil
}

// Head returns metadata from a successful HEAD response.
func (client *HTTPClient) Head(ctx context.Context, rawURL string) (size int64, resumable bool, contentType string, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, false, "", fmt.Errorf("create HEAD request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, false, "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, false, "", statusError(response)
	}

	contentType = response.Header.Get("Content-Type")
	resumable = response.Header.Get("Accept-Ranges") == "bytes"
	contentLength := response.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, resumable, contentType, nil
	}

	size, err = strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, false, "", fmt.Errorf("parse content length %q: %w", contentLength, err)
	}
	return size, resumable, contentType, nil
}

// GetRange fetches a byte range. Callers validate the response status and
// content range because servers can ignore Range headers.
func (client *HTTPClient) GetRange(ctx context.Context, rawURL string, start, end int64) (*http.Response, error) {
	if start < 0 {
		return nil, fmt.Errorf("range start must not be negative: %d", start)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create range request: %w", err)
	}
	if end >= start {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	} else {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
	}
	return client.Do(request)
}

// GetBody fetches a response body without applying a status policy.
func (client *HTTPClient) GetBody(ctx context.Context, rawURL string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create GET request: %w", err)
	}
	return client.Do(request)
}

// CloseIdleConnections closes idle keep alive connections held by the client.
func (client *HTTPClient) CloseIdleConnections() {
	if client != nil && client.client != nil {
		client.client.CloseIdleConnections()
	}
}

func statusError(response *http.Response) error {
	return &HTTPStatusError{StatusCode: response.StatusCode, Status: response.Status}
}
