
---

## OpenDownload — MVP Plan

### Recommended Stack: **Go**

**Why Go:**
- Single static binary — no runtime deps, easy distribution (like yt-dlp's standalone binary)
- Native concurrency (goroutines) — ideal for multi-threaded/segmented downloads
- Excellent networking stdlib — HTTP client, proxy support, TLS
- Cross-platform (Windows/macOS/Linux) with zero config
- MediaGo's backend already validates Go for this domain
- Fast startup — important for CLI tools

**Key libraries:**
| Concern | Library |
|---|---|
| CLI framework | `cobra` + `viper` (config) |
| HTTP client | `net/http` + `golang.org/x/net` |
| HLS parsing | Custom (port from hls-downloader's parser) |
| DASH/MPD parsing | `github.com/zencoder/go-dash` or custom |
| Progress display | `github.com/vbauerster/mpb` |
| Logging | `log/slog` (stdlib) |
| Muxing/conversion | FFmpeg (exec, not linked — optional dep) |
| Encryption | `crypto/aes` (stdlib) |
| Testing | stdlib `testing` + `testify` |

---

### Architecture

```
opendownload/
├── cmd/                    # CLI entry points (cobra commands)
│   ├── root.go             # Base command, global flags
│   ├── sniff.go            # Intercept mode — monitor a URL for media
│   ├── download.go         # Download a direct or stream URL
│   └── info.go             # Inspect URL, list available formats
├── internal/
│   ├── sniffer/            # Network traffic inspection
│   │   ├── sniffer.go      # HTTP response interception engine
│   │   ├── detector.go     # Content-Type & URL pattern matching
│   │   └── proxy.go        # Local MITM proxy for HTTPS sniffing
│   ├── parser/             # Stream manifest parsers
│   │   ├── hls.go          # M3U8 master + media playlist parser
│   │   ├── dash.go         # MPD manifest parser
│   │   └── common.go       # Shared types (Variant, Segment, etc.)
│   ├── downloader/         # Download engine
│   │   ├── engine.go       # Orchestrator — picks strategy per protocol
│   │   ├── http.go         # Direct HTTP download (Range-based segments)
│   │   ├── hls.go          # HLS fragment downloader
│   │   ├── dash.go         # DASH fragment downloader
│   │   ├── fragment.go     # Shared fragment reassembly logic
│   │   └── state.go        # Persist/resume download state
│   ├── crypto/             # Decryption
│   │   └── aes.go          # AES-128-CBC for HLS segments
│   ├── muxer/              # Post-processing
│   │   └── ffmpeg.go       # FFmpeg wrapper (merge, remux, convert)
│   └── util/               # Shared helpers
│       ├── http.go         # HTTP client with retry, UA rotation
│       └── filename.go     # Safe filename generation
├── go.mod
├── go.sum
└── main.go
```

---

### Task Breakdown

#### Phase 1 — Core Foundation
1. **Project scaffolding** — Go module, cobra CLI skeleton, CI basics
2. **HTTP client layer** — configurable client with retry, timeouts, User-Agent rotation, cookie support, proxy support
3. **Direct HTTP downloader** — single-file download with progress bar, Range-based resume, multi-segment parallel download

#### Phase 2 — Stream Support
4. **M3U8/HLS parser** — parse master playlists (variants, qualities, audio tracks) and media playlists (segments, encryption keys, byte ranges)
5. **HLS downloader** — parallel fragment download, AES-128 decryption, segment reassembly
6. **MPD/DASH parser** — parse adaptation sets, representations, segment templates
7. **DASH downloader** — parallel fragment download, segment reassembly

#### Phase 3 — Network Sniffing
8. **Local HTTP proxy** — forward proxy that intercepts HTTP traffic, logs requests/responses
9. **HTTPS MITM proxy** — generate local CA cert, TLS interception for HTTPS traffic
10. **Media detector** — inspect `Content-Type` headers + URL patterns to identify video/audio streams in proxied traffic
11. **`sniff` command** — start proxy, display detected media streams in real-time, let user pick which to download

#### Phase 4 — Post-Processing & Polish
12. **FFmpeg integration** — merge video+audio, remux to MP4/MKV, extract audio
13. **`info` command** — inspect a URL, display available formats/qualities without downloading
14. **Download state persistence** — save progress to disk, resume interrupted downloads across sessions
15. **Config file support** — YAML/TOML config for defaults (output dir, quality prefs, proxy settings)

#### Phase 5 — Stretch / V2
16. **Browser cookie import** — read cookies from Chrome/Firefox for authenticated downloads
17. **Batch mode** — download from URL list file
18. **Subtitle support** — detect, download, and embed WebVTT/SRT subtitles
19. **Site-specific extractors** — pluggable extractor pattern (like yt-dlp) for popular sites

---

### Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Language | Go | Single binary, native concurrency, great HTTP stack |
| CLI framework | Cobra | Industry standard for Go CLIs (kubectl, docker, gh all use it) |
| MITM approach | Local proxy + generated CA | Client-side only, no cloud. User configures browser/system to use proxy |
| FFmpeg | External binary (exec) | Avoid CGo complexity; FFmpeg is ubiquitous |
| State format | JSON file per download | Simple, human-readable, easy to debug |
| Extensibility | Interface-based downloaders/parsers | Easy to add new protocols |

### MVP Scope (Phases 1-3)

The MVP delivers: **download any direct video URL or HLS/DASH stream with multi-threaded acceleration, plus sniff network traffic through a local proxy to auto-detect downloadable media** — the core UC/Aloha experience as a CLI.