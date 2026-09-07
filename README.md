# OpenDownload

OpenDownload is a local desktop app and command line tool for saving media you are allowed to download. It supports direct HTTP and HTTPS files, HLS playlists (`.m3u8`), and DASH manifests (`.mpd`).

## Use the desktop app

1. Start the app in development with Wails.
   ```powershell
   wails dev
   ```
2. Paste a direct media URL, HLS playlist, or DASH manifest into **Video or stream URL**.
3. Optionally enter a destination folder. Leaving it empty saves to your Windows Downloads folder.
4. Select **Download** and follow the status in the Downloads panel.

The desktop workflow creates the destination folder when needed and selects the highest bandwidth video variant for HLS and DASH manifests. HLS output uses `.ts`; DASH output uses `.mp4` when the input URL ends in `.mpd`.

## Build a portable Windows app

The desktop build is the normal release path. Run the build helper without arguments, then launch the generated release artifact:

```powershell
.\scripts\build.ps1
.\build\bin\opendownload.exe
```

Do not use `go build .` or `go build -o opendownload.exe .` for the desktop application. Wails performs the frontend build, binding generation, Windows resource packaging, and production executable build.

## Advanced CLI build

Build the CLI entry point:

```powershell
.\scripts\build.ps1 -Target CLI
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

The pairing code combines a system selected loopback port with a fresh 256 bit cryptographic token. It is accepted only from `127.0.0.1` and expires for initial pairing after five minutes. Use **Refresh pairing code** in the desktop app to replace an expired code without relaunching OpenDownload. Refreshing invalidates the previous code. The token is never written to disk or displayed in captured stream metadata. Cookie and authorization values remain in memory for the active desktop session, are not displayed or logged, and are cleared when capture stops or the app exits.

This protects against network access and token guessing. It does not protect against another local process that obtains the active pairing code, so stop capture when you are finished. The add on requests broad host access because streams can come from a separate CDN domain, but it records only the active tab you explicitly selected.

## Proxy capture

The CLI proxy is an optional fallback for advanced use cases. Prefer the Firefox and Zen pairing flow above because it preserves ordinary browser certificate verification.

```powershell
.\build\bin\opendownload-cli.exe proxy run
```

Set your browser proxy to `127.0.0.1:9000`, load the page, and play the media. The proxy binds only to loopback, forwards ordinary HTTP traffic, and tunnels HTTPS without decrypting it. It cannot inspect encrypted media requests.

Use the Firefox and Zen add on path for encrypted stream capture because it preserves ordinary browser certificate verification and supplies the request context directly to the desktop app.

## Requirements and development

- Go 1.26 or later
- Node.js with pnpm
- Wails CLI for the desktop application: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

Run validation in this order because Go embeds the generated client bundle:

```powershell
pnpm --dir client typecheck
pnpm --dir client test
pnpm --dir client build
go test ./...
go vet ./...
go test -race ./...
wails build
```

Project layout:

- `cmd/` contains the Cobra CLI.
- `internal/download/` owns jobs, cancellation, output reservations, and publication.
- `internal/media/`, `internal/transport/`, `internal/capture/`, and `internal/sniffer/` each own one cohesive concern.
- `client/` contains the Vue desktop interface.
- `app.go` is the Wails bridge used by the desktop application.

## Current limitations

- DASH downloads select the best video representation. Audio track merging is not implemented.
- The Firefox and Zen add on is loaded temporarily for this initial release. Mozilla signing and public distribution are not included.
- Browser cookie import and custom headers are CLI only for manual URL downloads.

## License

MIT
