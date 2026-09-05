# OpenDownload

OpenDownload is a local desktop app and command line tool for saving media you are allowed to download. It supports direct HTTP and HTTPS files, HLS playlists (`.m3u8`), and DASH manifests (`.mpd`).

## Use the desktop app

1. Start the app in development with Wails.
   ```powershell
   wails dev
   ```
2. Paste a direct media URL, HLS playlist, or DASH manifest into **Video or stream URL**.
3. Optionally enter a destination folder. Leaving it empty saves to `./download`.
4. Select **Download** and follow the status in the Downloads panel.

The desktop workflow creates the destination folder when needed and selects the highest bandwidth video variant for HLS and DASH manifests. HLS output uses `.ts`; DASH output uses `.mp4` when the input URL ends in `.mpd`.

## Build a portable Windows app

Build the desktop application with Wails, then launch the generated release artifact:

```powershell
wails build
.\build\bin\opendownload.exe
```

Do not use `go build .` for the desktop application. It does not supply the Wails desktop build tags and produces an executable that cannot open the app window.

## Use the CLI

Build the CLI entry point:

```powershell
go build -o build\bin\opendownload-cli.exe ./cmd/cli
```

Download a URL:

```powershell
.\build\bin\opendownload-cli.exe download "https://example.com/video.mp4"
.\build\bin\opendownload-cli.exe download "https://example.com/playlist.m3u8" --output .\downloads
```

For protected content you are authorized to access, provide the required request information:

```powershell
.\build\bin\opendownload-cli.exe download "https://example.com/playlist.m3u8" --header "Referer: https://example.com/" --cookie "session=value"
```

Inspect a URL before downloading:

```powershell
.\build\bin\opendownload-cli.exe info "https://example.com/playlist.m3u8"
```

## Firefox and Zen capture

The desktop app can capture authorized media requests from Firefox or Zen without installing a local root certificate or intercepting TLS. It captures only the tab you select in the browser add on.

1. Open `about:debugging#/runtime/this-firefox` in Firefox Developer Edition or Zen.
2. Select **Load Temporary Add-on** and choose `extensions/firefox/manifest.json`.
3. In the OpenDownload desktop app, find the **Detected streams** card and select **Start capture**.
4. OpenDownload displays a long pairing code in that card. Select the copy icon beside it.
5. Open the browser add on, paste that code into **Code from OpenDownload desktop app**, then select **Start capture** while the streaming tab is active.
6. Play the media, then select a detected stream in OpenDownload to download it with the captured request context.

The pairing code expires after five minutes. Cookie and authorization values remain in memory for the active desktop session, are not displayed or logged, and are cleared when capture stops or the app exits. The add on requests broad host access because streams can come from a separate CDN domain, but it records only the active tab you explicitly selected.

## Proxy capture

The CLI proxy remains available for advanced use cases:

```powershell
.\build\bin\opendownload-cli.exe proxy run
```

Set your browser proxy to `127.0.0.1:9000`, load the page, and play the media. HTTPS request inspection requires a trusted local certificate and key:

```powershell
.\build\bin\opendownload-cli.exe proxy run --ca-cert .\ca.crt --ca-key .\ca.key
```

Only install a certificate you control, remove it when you no longer need it, and comply with the site terms and applicable law. The Firefox and Zen add on path is the preferred desktop capture workflow because it preserves ordinary browser certificate verification.

## Requirements and development

- Go 1.26 or later
- Node.js with pnpm
- Wails CLI for the desktop application: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

Run validation in this order because Go embeds the generated client bundle:

```powershell
pnpm --dir client build
go test ./...
wails build
```

Project layout:

- `cmd/` contains the Cobra CLI.
- `internal/` contains HTTP, HLS, DASH, detection, and utility packages.
- `client/` contains the Vue desktop interface.
- `app.go` is the Wails bridge used by the desktop application.

## Current limitations

- DASH downloads select the best video representation. Audio track merging is not implemented.
- The Firefox and Zen add on is loaded temporarily for this initial release. Mozilla signing and public distribution are not included.
- Browser cookie import and custom headers are CLI only for manual URL downloads.

## License

MIT
