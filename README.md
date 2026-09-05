# OpenDownload

OpenDownload is a local desktop app and command line tool for saving media you are allowed to download. It supports direct HTTP and HTTPS files, HLS playlists (`.m3u8`), and DASH manifests (`.mpd`).

## Use the desktop app

1. Build the client, then start Wails.
   ```powershell
   pnpm --dir client build
   wails dev
   ```
2. Paste a direct media URL, HLS playlist, or DASH manifest into **Video or stream URL**.
3. Optionally enter a destination folder. Leaving it empty saves to `./download`.
4. Select **Download** and follow the status in the Downloads panel.

The desktop workflow creates the destination folder when needed and selects the highest bandwidth video variant for HLS and DASH manifests. HLS output uses `.ts`; DASH output uses `.mp4` when the input URL ends in `.mpd`.

## Use the CLI

Build the CLI entry point:

```powershell
go build -o opendownload.exe ./cmd/cli
```

Download a URL:

```powershell
.\opendownload.exe download "https://example.com/video.mp4"
.\opendownload.exe download "https://example.com/playlist.m3u8" --output .\downloads
```

For protected content you are authorized to access, provide the required request information:

```powershell
.\opendownload.exe download "https://example.com/playlist.m3u8" --header "Referer: https://example.com/" --cookie "session=value"
```

Inspect a URL before downloading:

```powershell
.\opendownload.exe info "https://example.com/playlist.m3u8"
```

## Capture workflow

The current desktop client does not yet control the capture proxy. Use the CLI to discover streams, then paste the detected URL into the app or download it through the CLI.

```powershell
.\opendownload.exe proxy run
```

Set your browser proxy to `127.0.0.1:9000`, load the page, and play the media. For HTTPS interception, pass a trusted local certificate and key:

```powershell
.\opendownload.exe proxy run --ca-cert .\ca.crt --ca-key .\ca.key
```

Only install a certificate you control, remove it when you no longer need it, and comply with the site terms and applicable law.

## Requirements and development

- Go 1.26 or later
- Node.js with pnpm
- Wails CLI for the desktop application: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

Run validation in this order because Go embeds the generated client bundle:

```powershell
pnpm --dir client build
go test ./...
```

Project layout:

- `cmd/` contains the Cobra CLI.
- `internal/` contains HTTP, HLS, DASH, detection, and utility packages.
- `client/` contains the Vue desktop interface.
- `app.go` is the Wails bridge used by the desktop application.

## Current limitations

- DASH downloads select the best video representation. Audio track merging is not implemented.
- Capture proxy controls and live progress are CLI only.
- Browser cookie import and custom headers are CLI only.

## License

MIT
