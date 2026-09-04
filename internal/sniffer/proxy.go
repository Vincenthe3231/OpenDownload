package sniffer

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ProxyConfig struct {
	Port     int
	CACert   string
	CAKey    string
	Verbose  bool
	Detector *MediaDetector
}

type Proxy struct {
	config ProxyConfig
	server *http.Server
}

func NewProxy(config ProxyConfig) (*Proxy, error) {
	p := &Proxy{config: config}

	mux := http.NewServeMux()
	p.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: p,
	}
	_ = mux

	return p, nil
}

func (p *Proxy) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", p.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		<-ctx.Done()
		p.server.Close()
	}()

	err = p.server.Serve(ln)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

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

	resp, err := client.Do(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	contentLength, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	p.config.Detector.Inspect(targetURL, contentType, contentLength)

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		destConn.Close()
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		destConn.Close()
		return
	}

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	if p.config.CACert != "" && p.config.CAKey != "" {
		p.handleMITM(clientConn, destConn, r.Host)
		return
	}

	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
}

func (p *Proxy) handleMITM(clientConn, destConn net.Conn, host string) {
	defer clientConn.Close()
	defer destConn.Close()

	cert, err := tls.LoadX509KeyPair(p.config.CACert, p.config.CAKey)
	if err != nil {
		if p.config.Verbose {
			fmt.Printf("MITM cert error for %s: %v\n", host, err)
		}
		go transfer(destConn, clientConn)
		transfer(clientConn, destConn)
		return
	}

	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   hostname,
	}

	tlsClientConn := tls.Server(clientConn, tlsConfig)
	if err := tlsClientConn.Handshake(); err != nil {
		if p.config.Verbose {
			fmt.Printf("TLS handshake failed for %s: %v\n", host, err)
		}
		return
	}
	defer tlsClientConn.Close()

	tlsDestConfig := &tls.Config{
		ServerName: hostname,
	}
	tlsDestConn := tls.Client(destConn, tlsDestConfig)
	if err := tlsDestConn.Handshake(); err != nil {
		if p.config.Verbose {
			fmt.Printf("TLS upstream handshake failed for %s: %v\n", host, err)
		}
		return
	}
	defer tlsDestConn.Close()

	go transfer(tlsDestConn, tlsClientConn)
	transfer(tlsClientConn, tlsDestConn)
}

func transfer(dest io.WriteCloser, src io.ReadCloser) {
	defer dest.Close()
	defer src.Close()
	io.Copy(dest, src)
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
