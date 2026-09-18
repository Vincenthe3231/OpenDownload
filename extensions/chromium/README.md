# OpenDownload Chrome and Edge Capture

This is a static, unpacked Chromium WebExtension for Chrome and Edge 102 or later. It captures authorized media requests from the one tab you explicitly select and sends them to OpenDownload through native messaging. It uses MV3 `webRequest` observation only. It does not use debugger, cookies, tabs, or blocking permissions, and it does not intercept TLS.

## Load unpacked

1. Install OpenDownload with its Windows installer. This registers the native capture host for the current user.
2. Open `chrome://extensions` in Chrome or `edge://extensions` in Edge.
3. Enable **Developer mode**, select **Load unpacked**, and choose this `extensions/chromium` folder.
4. Open OpenDownload, then open this extension on the streaming tab and select **Start automatic capture**.
5. Play authorized media, then select a detected stream in OpenDownload to download it with the captured request context.

If automatic capture cannot connect, the popup offers **Use manual pairing** as a recovery path. It is not required for a normal installation.

The extension requests broad HTTP and HTTPS site access because media can come from a separate CDN domain. It still records only requests from the selected tab. Automatic capture keeps no credentials in extension storage. Manual-pairing metadata, captured headers, and request streams remain in memory only.

Chrome and Edge are supported. Other Chromium browsers are best effort. The receiver keeps the compatibility route `/v1/firefox/streams` and the desktop methods `StartFirefoxCapture` and `StopFirefoxCapture` for existing Firefox and Zen clients.

Chromium exposes the final request headers through `webRequest` according to browser and site policy. Cookie, Referer, and Authorization are forwarded when the browser exposes them. If a browser or site suppresses a header, OpenDownload does not add a debugger fallback.
