let capture = null;
let lastFailure = null;
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

function rawHeaders(headers) {
  const result = {};
  for (const header of headers || []) {
    if (!header || typeof header.name !== 'string' || typeof header.value !== 'string') continue;
    if (Object.prototype.hasOwnProperty.call(result, header.name)) {
      result[header.name] = `${result[header.name]}, ${header.value}`;
    } else {
      result[header.name] = header.value;
    }
  }
  return result;
}

function debugContextFromRequest(details) {
  return {
    requestId: typeof details.requestId === 'string' ? details.requestId : '',
    requestUrl: details.url,
    requestMethod: typeof details.method === 'string' ? details.method : '',
    requestType: typeof details.type === 'string' ? details.type : '',
    requestTimestamp: typeof details.timeStamp === 'number' ? details.timeStamp : 0,
    requestFrameId: Number.isInteger(details.frameId) ? details.frameId : 0,
    requestParentFrameId: Number.isInteger(details.parentFrameId) ? details.parentFrameId : 0,
    requestDocumentUrl: typeof details.documentUrl === 'string' ? details.documentUrl : '',
    requestOriginUrl: typeof details.originUrl === 'string' ? details.originUrl : '',
    requestInitiator: typeof details.initiator === 'string' ? details.initiator : '',
    requestHeaders: rawHeaders(details.requestHeaders),
  };
}

function addResponseDebug(debug, details) {
  debug.responseHeaders = rawHeaders(details.responseHeaders);
  debug.responseStatus = Number.isInteger(details.statusCode) ? details.statusCode : 0;
  debug.responseStatusLine = typeof details.statusLine === 'string' ? details.statusLine : '';
  debug.responseFromCache = details.fromCache === true;
  debug.responseIp = typeof details.ip === 'string' ? details.ip : '';
}

async function sendCapture(request, type) {
  const session = capture;
  if (!session) return;
  if (session.mode === 'automatic') {
    try {
      const message = { protocolVersion: 1, type: 'stream', tabId: session.tabId, stream: { url: request.url, type, headers: request.headers } };
      if (session.diagnosticMode === true && request.debug) message.debug = request.debug;
      const response = await nativeRequest(session.port, message);
      if (!response || response.type !== 'accepted' || capture !== session) session.error = 'OpenDownload did not accept the stream.';
    } catch {
      if (capture === session) session.error = 'Could not send the stream to OpenDownload.';
    }
    return;
  }
  const payload = { url: request.url, type, headers: request.headers };
  if (session.diagnosticMode === true && request.debug) payload.debug = request.debug;
  const response = await fetch(`${session.endpoint}/v1/firefox/streams`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      OpenDownloadSession: session.token,
    },
    body: JSON.stringify(payload),
  });
  if (!response.ok && capture === session) {
    if (response.status === 401) {
      capture = null;
      pending.clear();
      lastFailure = 'Pairing expired or was replaced. Paste the fresh code from OpenDownload and select Start capture.';
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
    const request = { url: details.url, headers: selectedHeaders(details.requestHeaders) };
    if (capture.diagnosticMode === true) request.debug = debugContextFromRequest(details);
    rememberRequest(details.requestId, request);
  },
  { urls: ['<all_urls>'] },
  ['requestHeaders'],
);

browser.webRequest.onHeadersReceived.addListener(
  (details) => {
    const request = pending.get(details.requestId);
    pending.delete(details.requestId);
    if (!capture || !request || details.tabId !== capture.tabId || details.statusCode < 200 || details.statusCode >= 300) return;
    if (capture.diagnosticMode === true && request.debug) addResponseDebug(request.debug, details);
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

const automaticCaptureMessage = 'Automatic capture is unavailable. Open OpenDownload, then repair the native host or use manual pairing.';

function safeNativeFailure(response) {
  const failure = response && response.failure;
  return failure && typeof failure.userMessage === 'string' && failure.userMessage ? failure.userMessage : automaticCaptureMessage;
}

async function manualCaptureOptions(pairing) {
  try {
    const response = await fetch(`${pairing.endpoint}/v1/firefox/capture-options`, {
      headers: { OpenDownloadSession: pairing.token },
    });
    if (!response.ok) return false;
    const options = await response.json();
    return options && options.protocolVersion === 1 && options.diagnosticMode === true;
  } catch {
    return false;
  }
}

function handleMessage(message) {
  if (message.type === 'start') {
    return browser.tabs.query({ active: true, currentWindow: true }).then(async ([tab]) => {
      if (!tab || tab.id === undefined || !/^https?:/.test(tab.url || '')) {
        throw new Error('Open a website in the active tab before starting capture.');
      }
      pending.clear();
      lastFailure = null;
      if (message.manual === true) {
        const pairing = normalizePairingCode(message.code);
        const diagnosticMode = await manualCaptureOptions(pairing);
        capture = { ...pairing, mode: 'manual', diagnosticMode, tabId: tab.id, error: '' };
        return status();
      }

      let port;
      try {
        port = browser.runtime.connectNative('com.opendownload.capture');
      } catch {
        lastFailure = automaticCaptureMessage;
        throw new Error(lastFailure);
      }
      port.onDisconnect.addListener(() => {
        if (capture && capture.mode === 'automatic' && capture.port === port) {
          capture = null;
          pending.clear();
          lastFailure = 'Automatic capture host disconnected. Repair the native host or use manual pairing.';
        }
      });

      let failureMessage = automaticCaptureMessage;
      return nativeRequest(port, { protocolVersion: 1, type: 'start', browser: 'firefox', tabId: tab.id }).then((response) => {
        if (!response || response.type !== 'started') {
          failureMessage = safeNativeFailure(response);
          throw new Error(failureMessage);
        }
        capture = { mode: 'automatic', diagnosticMode: response.diagnosticMode === true, tabId: tab.id, port, error: '' };
        return status();
      }).catch(() => {
        pending.clear();
        lastFailure = failureMessage;
        if (port && typeof port.disconnect === 'function') port.disconnect();
        throw new Error(failureMessage);
      });
    });
  }
  if (message.type === 'stop') {
    if (capture && capture.mode === 'automatic') {
      return nativeRequest(capture.port, { protocolVersion: 1, type: 'stop' }).catch(() => {}).then(() => {
        if (capture.port && typeof capture.port.disconnect === 'function') capture.port.disconnect();
        capture = null;
        pending.clear();
        lastFailure = null;
        return status();
      });
    }
    capture = null;
    pending.clear();
    lastFailure = null;
    return Promise.resolve(status());
  }
  return Promise.resolve(status());
}

browser.runtime.onMessage.addListener(handleMessage);

function status() {
  return capture ? { active: true, mode: capture.mode || 'manual', error: capture.error || '' } : { active: false, mode: 'manual', error: lastFailure || '' };
}
