# OpenDownload

OpenDownload is a high-performance cross-platform tool for network traffic interception and video stream downloading.

## Installation

### 1. Requirements
- [Go](https://go.dev/doc/install)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) (for GUI)
    ```powershell
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    ```

### 2. Build
```bash
git clone ...
cd opendownload
go build -o opendownload .
```

## Usage

### 1. GUI Mode (Requires Wails)
```powershell
wails dev
```

### 2. Manual Download (CLI)
If the GUI isn't available, use the CLI directly:
```powershell
./opendownload download "URL" --cookies-from-browser chrome
```

### 3. Pro Workflow: Finding the Stream URL (Without Proxy)
When the proxy Sniffer (Section 4) doesn't catch the stream, do this manually:
1. Open the video page in your favorite browser.
2. Press `F12` to open **Developer Tools**.
3. Go to the **Network** tab.
4. Set the filter to **XHR**, **Fetch**, or **Media**.
5. Refresh the page and play the video.
6. Look for a file ending in `.m3u8` or `.mpd` (the playlist/manifest file).
7. Right-click that file -> **Copy Link Address**.
8. Paste this URL into `opendownload download <URL>`.

### 4. Traffic Sniffing (Capture Mode)
1. Run the sniffer:
   ```bash
   ./opendownload.exe sniff -p 9000 --ca-cert ca.crt --ca-key ca.key -v
   ```
2. Set your browser proxy to `127.0.0.1:9000`.
3. Streams will log automatically to your terminal.

## Extractor Framework
Add site-specific logic to `internal/extractors/` to automatically bypass headers/tokens.

## Configuration
Supports `opendownload.yaml` (in current dir or home), environment variables (`OD_`), or flags.
