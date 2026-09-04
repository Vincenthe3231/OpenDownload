# OpenDownload

OpenDownload is a high-performance, cross-platform CLI tool for network traffic interception and video stream downloading. It enables seamless downloading of media from direct URLs, HLS (.m3u8), and DASH (.mpd) streams, with robust support for bypassing CDN protection through cookie and header injection.

## Features

- **Protocol Support**: Direct HTTP(S), HLS (including AES-128 encrypted), and DASH.
- **Acceleration**: Multi-threaded, segmented downloads with range-based resume support.
- **Traffic Interception**: Built-in HTTP/HTTPS proxy to sniff and auto-detect media streams in your browser.
- **Robustness**: Header and Cookie injection to bypass site-specific protection.
- **Post-Processing**: Automatic integration with FFmpeg for muxing and format conversion.
- **Lightweight**: Zero runtime dependencies, single static binary (written in Go).

## Installation

Ensure you have [Go](https://go.dev/doc/install) installed.

```bash
# Clone the repository
git clone https://github.com/opendownload/opendownload
cd opendownload

# Build the binary
go build -o opendownload .
```

## Usage

### 1. Downloading Video Streams
Download a stream directly by providing the URL. Use headers/cookies to bypass protection on sites like `missav.ws`.

```bash
# Basic download
./opendownload download "https://example.com/stream.m3u8"

# With authentication
./opendownload download "https://example.com/playlist.m3u8" \
  --header "Referer: https://protected-site.com/" \
  --cookie "session_id=your_cookie_value"
```

### 2. Inspecting Streams
List available formats, qualities, and resolution info without downloading.

```bash
./opendownload info "https://example.com/stream.m3u8"
```

### 3. Sniffing Network Traffic
Start a local proxy, configure your browser to use it, and browse normally to detect media.

```bash
# Start proxy on port 9000
./opendownload sniff -p 9000

# Enable HTTPS interception (requires generating a local CA)
./opendownload sniff -p 9000 --ca-cert ca.crt --ca-key ca.key -v
```

## Configuration

OpenDownload supports configuration via a `.opendownload.yaml` file, environment variables (prefixed with `OD_`), or command-line flags.

**Order of precedence:** CLI Flags > Env Vars > Config File.

Example `.opendownload.yaml`:
```yaml
output: ./downloads
workers: 8
proxy: http://127.0.0.1:9000
```

## Development

The project follows a modular, interface-based architecture:
- `cmd/`: CLI command definitions (Cobra).
- `internal/downloader/`: Concurrent download engines (HTTP, HLS, DASH).
- `internal/parser/`: Manifest parsers (M3U8, MPD).
- `internal/sniffer/`: MITM proxy and media detection logic.
- `internal/muxer/`: FFmpeg wrappers for post-processing.

To run tests:
```bash
go test ./...
```

## License
MIT
