# OpenDownload MVP Plan

## Goal
Build an MVP network traffic interception + video downloading command-line tool that resembles UC browser and Aloha browser. The features all conduct on client side without relying any infra or cloud.

## Recommended Stack: **Go**

**Why Go:**
- Single static binary — no runtime deps, easy distribution.
- Native concurrency (goroutines) — ideal for multi-threaded/segmented downloads.
- Excellent networking stdlib — HTTP client, proxy support, TLS.
- Cross-platform usage.

## Architecture

```
opendownload/
├── cmd/                    # CLI entry points (cobra commands)
├── internal/
│   ├── sniffer/            # Network traffic inspection
│   ├── parser/             # Stream manifest parsers
│   ├── downloader/         # Download engine
│   ├── extractors/         # Site-specific extraction logic
│   ├── muxer/              # Post-processing (FFmpeg)
│   └── util/               # Shared helpers
```

## Task Breakdown

### Phase 1 — Foundation (Completed)
- Project scaffolding
- HTTP client layer
- Direct HTTP downloader

### Phase 2 — Stream Support (Completed)
- M3U8/HLS parser & downloader
- MPD/DASH parser & downloader

### Phase 3 — Network Sniffing (Completed)
- Local HTTP/HTTPS MITM proxy
- Media detection (Content-Type/URL matching)
- `sniff` command

### Phase 4 — Robustness & Automation (In Progress)
- **Automated Cookie Import**: Automatically load browser cookies (Completed).
- **Extractor Framework**: Implement a plugin-based system to automatically extract stream URLs, headers, and tokens for specific sites (e.g., missav.ws, hanime).
- **Dynamic Stream Monitoring**: Handle playlist refreshes during HLS/DASH downloads.

### Phase 5 — GUI & Polish
- **GUI Application**: Develop a lightweight desktop frontend (e.g., using Wails or Electron) that leverages the existing Go engine.
- **Batch downloader**
- **Subtitle downloading/embedding**
