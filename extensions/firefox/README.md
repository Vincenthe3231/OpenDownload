# OpenDownload Firefox Capture

This temporary Firefox WebExtension captures media requests from the tab you explicitly select and sends them to a paired OpenDownload desktop app. It does not intercept TLS or change certificate trust. Chrome and Edge use the separate unpacked package in `extensions/chromium`.

## Load temporarily

1. Open `about:debugging#/runtime/this-firefox` in Firefox Developer Edition or Zen.
2. Select **Load Temporary Add-on**.
3. Choose `manifest.json` from this folder.
4. Open the OpenDownload desktop app. In its **Detected streams** card, select **Start capture**.
5. OpenDownload shows a long pairing code in that card. Select its copy button. Confirm the add on panel title shows version `0.1.1` or later.
6. Open the extension, paste that code, then select **Start capture** while the streaming tab is active.

The pairing code expires after five minutes. Use **Refresh pairing code** in OpenDownload to create a new one without relaunching the desktop app. This invalidates the old code. If Firefox says `Receiving end does not exist`, open `about:debugging`, reload **OpenDownload Capture**, then close and reopen the add on panel. Stop capture when finished. Captured cookies and authorization headers remain in memory only and are not displayed by OpenDownload.
