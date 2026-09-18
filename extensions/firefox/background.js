let capture = null;
let lastError = '';
const pending = new Map();

function nativeRequest(port, message) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('Native host response timed out.')), 5000);
    const receive = (response) => {
      clearTimeout(timer);
      port.onMessage.removeListener(receive);
      resolve(response);
    };
    port.onMessage.addListener(receive);
    try {
      port.postMessage(message);
    } catch (reason) {
      clearTimeout(timer);
      port.onMessage.removeListener(receive);
      reject(reason);
    }
  });
}

function normalizePairingCode(value) {
  const [endpoint, token] = value.trim().split('#');
  const parsed = new URL(endpoint);
  if (parsed.protocol !== 'http:' || parsed.hostname !== '127.0.0.1' || !parsed.port || !token) {
    throw new Error('Use the pairing code shown by OpenDownload.');
  }
  return { endpoint: endpoint.replace(/\/$/, ''), token };
}

function pathnameFromURL(value) {
  try {
    return new URL(value).pathname.toLowerCase();
  } catch {
    return '';
  }
}

function extensionFromPathname(pathname) {
  const match = pathname.match(/\.([a-z0-9]+)$/);
  return match ? match[1] : '';
}

function isMediaFragment(pathname, contentType) {
  const extension = extensionFromPathname(pathname);
  return CaptureConstants.fragmentExtensions.has(extension) || contentType.includes('iso.segment') || contentType.includes('mp2t');
}

function mediaType(url, headers) {
  const contentType = (headers.find((header) => header.name.toLowerCase() === 'content-type') || {}).value || '';
  const normalized = contentType.toLowerCase();
  const pathname = pathnameFromURL(url);
  if (!pathname || isMediaFragment(pathname, normalized)) return '';
  if (normalized.includes('mpegurl') || pathname.endsWith('.m3u8')) return 'hls';
  if (normalized.includes('dash+xml') || pathname.endsWith('.mpd')) return 'dash';
  if (normalized.startsWith('video/')) return 'video';
  if (normalized.startsWith('audio/')) return 'audio';
  if (CaptureConstants.directMediaExtensions.has(extensionFromPathname(pathname))) return 'video';
  return '';
}

function selectedHeaders(headers) {
  const result = {};
  for (const header of headers || []) {
    const key = header.name.toLowerCase();
    if (CaptureConstants.allowedRequestHeaders.has(key) && header.value) result[header.name] = header.value;
  }
  return result;
}

async function sendCapture(request, type) {
  const session = capture;
  if (!session) return;
  if (session.mode === 'automatic') {
    try {
      const response = await nativeRequest(session.port, { type: 'stream', tabId: session.tabId, stream: { url: request.url, type, headers: request.headers } });
      if (!response || response.type !== 'accepted' || capture !== session) session.error = 'OpenDownload did not accept the stream.';
    } catch {
      if (capture === session) session.error = 'Could not send the stream to OpenDownload.';
    }
    return;
  }
  const response = await fetch(`${session.endpoint}/v1/firefox/streams`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      OpenDownloadSession: session.token,
    },
    body: JSON.stringify({ url: request.url, type, headers: request.headers }),
  });
  if (!response.ok && capture === session) {
    if (response.status === 401) {
      capture = null;
      pending.clear();
      lastError = 'Pairing expired or was replaced. Paste the fresh code from OpenDownload and select Start capture.';
      return;
    }
    session.error = 'OpenDownload did not accept the stream.';
  }
}

function rememberRequest(requestID, request) {
  if (pending.size >= CaptureConstants.maxPendingRequests) {
    const oldestRequestID = pending.keys().next().value;
    if (oldestRequestID !== undefined) pending.delete(oldestRequestID);
  }
  pending.set(requestID, request);
}

browser.webRequest.onSendHeaders.addListener(
  (details) => {
    if (!capture || details.tabId !== capture.tabId) return;
    rememberRequest(details.requestId, { url: details.url, headers: selectedHeaders(details.requestHeaders) });
  },
  { urls: ['<all_urls>'] },
  ['requestHeaders'],
);

browser.webRequest.onHeadersReceived.addListener(
  (details) => {
    const request = pending.get(details.requestId);
    pending.delete(details.requestId);
    if (!capture || !request || details.tabId !== capture.tabId || details.statusCode < 200 || details.statusCode >= 300) return;
    const type = mediaType(request.url, details.responseHeaders || []);
    if (type) sendCapture(request, type).catch(() => {
      if (capture) capture.error = 'Could not send the stream to OpenDownload.';
    });
  },
  { urls: ['<all_urls>'] },
  ['responseHeaders'],
);

browser.webRequest.onErrorOccurred.addListener(
  (details) => pending.delete(details.requestId),
  { urls: ['<all_urls>'] },
);

function handleMessage(message) {
  if (message.type === 'start') {
    return browser.tabs.query({ active: true, currentWindow: true }).then(([tab]) => {
      if (!tab || tab.id === undefined || !/^https?:/.test(tab.url || '')) {
        throw new Error('Open a website in the active tab before starting capture.');
      }
      pending.clear();
      lastError = '';
	  if (message.manual === true) {
		const pairing = normalizePairingCode(message.code);
		capture = { ...pairing, tabId: tab.id, error: '' };
		return Promise.resolve(status());
	  }

	  try {
		const port = browser.runtime.connectNative('com.opendownload.capture');
		port.onDisconnect.addListener(() => {
			if (capture && capture.mode === 'automatic' && capture.port === port) {
				capture = null;
				pending.clear();
				lastError = 'Automatic capture host disconnected. Repair the native host or use manual pairing.';
			}
		});
		return nativeRequest(port, { type: 'start', browser: 'firefox', tabId: tab.id }).then((response) => {
			if (!response || response.type !== 'started') throw new Error(response && response.error ? response.error : 'Automatic capture is unavailable.');
			capture = { mode: 'automatic', tabId: tab.id, port, error: '' };
			return status();
		}).catch((reason) => {
			if (!message.code) throw reason;
			const pairing = normalizePairingCode(message.code);
			capture = { ...pairing, tabId: tab.id, error: '' };
			return status();
		});
	  } catch (reason) {
		if (!message.code) throw reason;
		const pairing = normalizePairingCode(message.code);
		capture = { ...pairing, tabId: tab.id, error: '' };
		return Promise.resolve(status());
	  }
    });
  }
  if (message.type === 'stop') {
    if (capture && capture.mode === 'automatic') {
      return nativeRequest(capture.port, { type: 'stop' }).catch(() => {}).then(() => {
        if (capture.port && typeof capture.port.disconnect === 'function') capture.port.disconnect();
        capture = null;
        pending.clear();
        lastError = '';
        return status();
      });
    }
    capture = null;
    pending.clear();
    lastError = '';
    return status();
  }
  return Promise.resolve(status());
}

browser.runtime.onMessage.addListener(handleMessage);

function status() {
  return capture ? { active: true, mode: capture.mode || 'manual', error: capture.error } : { active: false, mode: 'manual', error: lastError };
}
