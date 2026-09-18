# OpenDownload Chrome and Edge Capture

This is a static, unpacked Chromium WebExtension for Chrome and Edge 102 or later. It captures authorized media requests from the one tab you explicitly select and sends them to a paired OpenDownload desktop app. It uses MV3 `webRequest` observation only. It does not use debugger, cookies, tabs, or blocking permissions, and it does not intercept TLS.

## Load unpacked

1. Open `chrome://extensions` in Chrome or `edge://extensions` in Edge.
2. Enable **Developer mode**.
3. Select **Load unpacked** and choose this `extensions/chromium` folder.
4. In OpenDownload, find **Detected streams** and select **Generate pairing code**.
5. Copy the pairing code, open this extension in the toolbar, paste the code, and select **Start capture** while the streaming tab is active.
6. Play authorized media, then select a detected stream in OpenDownload to download it with the captured request context.

The extension requests broad HTTP and HTTPS site access because media can come from a separate CDN domain. It still records only requests from the selected tab. Pairing metadata is held in `chrome.storage.session`; captured headers and request streams are kept only in memory and are never written to extension storage.

Chrome and Edge are supported. Other Chromium browsers are best effort. The receiver keeps the compatibility route `/v1/firefox/streams` and the desktop methods `StartFirefoxCapture` and `StopFirefoxCapture` for existing Firefox and Zen clients.

Chromium exposes the final request headers through `webRequest` according to browser and site policy. Cookie, Referer, and Authorization are forwarded when the browser exposes them. If a browser or site suppresses a header, OpenDownload does not add a debugger fallback.
