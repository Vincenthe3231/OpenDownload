# OpenDownload Firefox Capture

This temporary Firefox WebExtension captures media requests from the tab you explicitly select and sends them to OpenDownload through native messaging. It does not intercept TLS or change certificate trust. Chrome and Edge use the separate unpacked package in `extensions/chromium`.

## Load temporarily

1. Install OpenDownload with its Windows installer. This registers the native capture host for the current user.
2. Open `about:debugging#/runtime/this-firefox` in Firefox Developer Edition or Zen.
3. Select **Load Temporary Add-on** and choose `manifest.json` from this folder.
4. Open OpenDownload, then open the extension on the streaming tab and select **Start automatic capture**.
5. Play authorized media and download a detected stream in OpenDownload.

If automatic capture cannot connect, the popup offers **Use manual pairing** as a recovery path.

If Firefox says `Receiving end does not exist`, open `about:debugging`, reload **OpenDownload Capture**, then close and reopen the add on panel. Stop capture when finished. Captured cookies and authorization headers remain in memory only and are not displayed by OpenDownload.
