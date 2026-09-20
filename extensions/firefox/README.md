# OpenDownload Firefox Capture

This temporary Firefox WebExtension captures media requests from the tab you explicitly select and sends them to OpenDownload through native messaging. It does not intercept TLS or change certificate trust. Chrome and Edge use the separate unpacked package in `extensions/chromium`.

## Automatic capture

1. Install OpenDownload with its Windows installer. This registers the native capture host for the current user.
2. Open `about:debugging#/runtime/this-firefox` in Firefox Developer Edition or Zen.
3. Select **Load Temporary Add-on** and choose `manifest.json` from this folder.
4. Start OpenDownload, then open the extension on the streaming tab and select **Start automatic capture**.
5. Play authorized media and download a detected stream in OpenDownload. Automatic capture uses the registered native host and does not need a proxy, root certificate, or TLS interception.

## Manual pairing recovery

If automatic capture cannot connect, select **Use manual pairing** in the popup only as a recovery path. Copy the code from the OpenDownload desktop app, paste it into **Code from OpenDownload desktop app**, and start capture while the streaming tab is active.

If Firefox says `Receiving end does not exist`, open `about:debugging`, reload **OpenDownload Capture**, then close and reopen the add on panel. Stop capture when finished. Captured cookies and authorization headers remain in memory only and are not displayed by OpenDownload.
