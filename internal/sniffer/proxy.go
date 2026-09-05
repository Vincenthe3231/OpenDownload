package sniffer

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ProxyConfig configures the local media detector proxy.
type ProxyConfig struct {
	Port     int
	Verbose  bool
	Detector *MediaDetector
}

// Proxy is a loopback only HTTP proxy that tunnels HTTPS without interception.
type Proxy struct {
	config ProxyConfig
	server *http.Server
}

// NewProxy creates a loopback only proxy.
func NewProxy(config ProxyConfig) (*Proxy, error) {
	if config.Detector == nil {
		return nil, fmt.Errorf("media detector is required")
	}
	if config.Port < 0 || config.Port > 65535 {
		return nil, fmt.Errorf("proxy port must be between 0 and 65535")
	}
	address := net.JoinHostPort(loopbackHost, strconv.Itoa(config.Port))
	return &Proxy{
		config: config,
		server: &http.Server{
			Addr:    address,
			Handler: nil,
		},
	}, nil
}

// Start serves until the context is cancelled.
func (p *Proxy) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", p.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	p.server.Handler = p
	go func() {
		<-ctx.Done()
		_ = p.server.Close()
	}()

	err = p.server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// ServeHTTP forwards plain HTTP requests and tunnels CONNECT requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handleHTTP(w, r)
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.String()
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "http://" + r.Host + r.URL.String()
	}

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	copyHeaders(outReq.Header, r.Header)
	outReq.Header.Del("Proxy-Connection")
	outReq.Header.Del("Proxy-Authorization")

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()

	contentLength, _ := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	p.config.Detector.Inspect(targetURL, response.Header.Get("Content-Type"), contentLength)
	copyHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	destination, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		_ = destination.Close()
		return
	}
	client, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		_ = destination.Close()
		return
	}

	if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = client.Close()
		_ = destination.Close()
		return
	}
	go transfer(destination, client)
	go transfer(client, destination)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	_, _ = io.Copy(destination, source)
}

func copyHeaders(destination, source http.Header) {
	for key, values := range source {
		for _, value := range values {
			destination.Add(key, value)
		}
	}
}
