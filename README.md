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

## Build the Windows installer

Use the desktop build helper for installer builds:

```powershell
.\scripts\build.ps1 -Target Desktop
```

This command builds `opendownload-native-host.exe` first, verifies that the output is fresh and non-empty, then runs Wails with NSIS packaging. The generated installer is placed at:

```text
.\build\bin\OpenDownload-amd64-installer.exe
```

Do not use `wails build -nsis` by itself for a release installer. Direct Wails packaging does not rebuild the native host, so it can package an older `build\bin\opendownload-native-host.exe` beside a newer desktop executable. If you must invoke Wails directly, rebuild the host first:

```powershell
go build -o .\build\bin\opendownload-native-host.exe .\cmd\opendownload-native-host
wails build -nsis -installscope user
```

The installer copies the desktop executable, native host, Gecko registration script, and Firefox extension ID configuration. It registers the host for the current Windows user under `HKCU\Software\Mozilla\NativeMessagingHosts\com.opendownload.capture`.

After installation, start the installed OpenDownload application before selecting automatic capture in the Firefox or Zen extension. Automatic capture requires both the registered native host and the desktop named-pipe server. Use the registration script's inspection action when troubleshooting:

```powershell
$installRoot = "$env:LOCALAPPDATA\Programs\OpenDownload"
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File "$installRoot\register-native-host.ps1" `
  -Action Inspect `
  -InstallRoot $installRoot `
  -ConfigPath "$installRoot\browser-ids.json"
```

An inspection result with `"healthy":true` confirms static host registration, manifest paths, and the Firefox extension ID. It does not confirm that OpenDownload is running or that the native host can connect to the desktop named pipe. Close older OpenDownload processes, launch the installed executable, and reload the extension before retrying automatic capture.

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

## Browser capture

The desktop app can capture authorized media requests from Chrome, Edge, Firefox, or Zen without installing a local root certificate or intercepting TLS. It captures only the tab you select in the browser extension.

### Firefox and Zen

1. Open `about:debugging#/runtime/this-firefox` in Firefox Developer Edition or Zen.
2. Select **Load Temporary Add-on** and choose `extensions/firefox/manifest.json`.
3. In the OpenDownload desktop app, find the **Detected streams** card and select **Start capture**.
4. OpenDownload displays a long pairing code in that card. Select the copy icon beside it.
5. Open the browser extension, paste that code into **Code from OpenDownload desktop app**, then select **Start capture** while the streaming tab is active.
6. Play the media, then select a detected stream in OpenDownload to download it with the captured request context.

### Chrome and Edge

1. Open `chrome://extensions` in Chrome or `edge://extensions` in Edge.
2. Enable **Developer mode**, select **Load unpacked**, and choose `extensions/chromium`.
3. In OpenDownload, find the **Detected streams** card and select **Generate pairing code**.
4. Copy the pairing code, open the OpenDownload Capture extension, paste the code, and select **Start capture** while the streaming tab is active.
5. Play the media, then select a detected stream in OpenDownload to download it with the captured request context.

The Chrome and Edge package is an unpacked MV3 extension for Chromium 102 or later. It requests broad HTTP and HTTPS site access because media can come from a separate CDN domain. Chrome and Edge are supported, and other Chromium browsers are best effort. See [extensions/chromium/README.md](extensions/chromium/README.md) for browser-specific behavior and final-header notes.

The pairing code combines a system selected loopback port with a fresh 256 bit cryptographic token. It is accepted only from `127.0.0.1` and expires for initial pairing after five minutes. Use **Refresh pairing code** in the desktop app to replace an expired code without relaunching OpenDownload. Refreshing invalidates the previous code. The token is never written to disk or displayed in captured stream metadata. Cookie and authorization values remain in memory for the active desktop session, are not displayed or logged, and are cleared when capture stops or the app exits.

This protects against network access and token guessing. It does not protect against another local process that obtains the active pairing code, so stop capture when you are finished. The browser extensions request broad host access because streams can come from a separate CDN domain, but they record only the active tab you explicitly selected.

## Proxy capture

The CLI proxy is an optional fallback for advanced use cases. Prefer the Firefox and Zen pairing flow above because it preserves ordinary browser certificate verification.

```powershell
.\build\bin\opendownload-cli.exe proxy run
```

Set your browser proxy to `127.0.0.1:9000`, load the page, and play the media. The proxy binds only to loopback, forwards ordinary HTTP traffic, and tunnels HTTPS without decrypting it. It cannot inspect encrypted media requests.

Use the browser extension path for encrypted stream capture because it preserves ordinary browser certificate verification and supplies the request context directly to the desktop app.

## Requirements and development

- Go 1.26 or later
- Node.js with pnpm
- Wails CLI for the desktop application: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

Run validation in this order because Go embeds the generated client bundle:

```powershell
pnpm lint
```

The combined lint command runs ESLint for the Vue and TypeScript client, followed by `golangci-lint run ./...` for the Go application and server packages. Install `golangci-lint` and make sure it is available on `PATH` before running it.

```powershell
pnpm --dir client typecheck
pnpm --dir client test
pnpm --dir client build
go test ./...
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
- The Firefox and Zen extension is loaded temporarily, and the Chrome and Edge extension is loaded unpacked for this initial release. Store signing and public distribution are not included.
- Browser cookie import and custom headers are CLI only for manual URL downloads.

## License

MIT
