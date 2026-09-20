# OpenDownload release checklist

Run this checklist for a Windows release. Automatic Firefox and Zen capture is the primary browser workflow. Manual pairing is a recovery path and is not a release substitute for native-host registration.

## Build artifacts

- [ ] `pnpm --dir client test` passes.
- [ ] `go test ./...` passes.
- [ ] `packaging/windows/test-register-native-host.ps1 -SkipRegistry` passes.
- [ ] `scripts/build.ps1 -Target Desktop` completes successfully.
- [ ] The desktop installer includes `opendownload-native-host.exe`, `register-native-host.ps1`, `browser-ids.json`, and the desktop executable.
- [ ] The Gecko extension is packaged from `extensions/firefox` and its `manifest.json` contains the expected Mozilla extension ID.
- [ ] The CI installer artifact and Gecko extension artifact can be downloaded and opened.

## Windows capture acceptance

- [ ] Install the Windows installer for the current user.
- [ ] On the installer finish page, confirm the checked **Launch OpenDownload Desktop** option opens the desktop app. Repeat once with the option unchecked and confirm the installer exits without launching it.
- [ ] Mozilla native-host registration passes with the installer’s **Inspect** action and points to the installed manifest.
- [ ] Open OpenDownload before opening the extension.
- [ ] Firefox automatic capture works on an authorized media tab without a proxy or local root certificate.
- [ ] Zen smoke test passes using the same Gecko extension and automatic capture flow.
- [ ] Close OpenDownload while automatic capture is connected and confirm opendownload-native-host.exe exits as well.
- [ ] Manual pairing remains available only as recovery when automatic capture needs attention.
- [ ] A captured stream can be downloaded with its request context.

## Diagnostics privacy acceptance

- [ ] Enable developer diagnostics, induce a failed capture or download, and confirm the safe message and diagnostic ID are visible.
- [ ] Confirm technical details are shown only while developer diagnostics is enabled and contain only safe fields.
- [ ] Disable developer diagnostics and confirm visible technical details disappear.
- [ ] Stop capture and confirm sensitive request and response context is erased.
- [ ] Exit OpenDownload and confirm sensitive diagnostics do not survive the process.
