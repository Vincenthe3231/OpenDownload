const allowedHeaders = new Set([
  'accept',
  'accept-language',
  'authorization',
  'cookie',
  'origin',
  'referer',
  'user-agent',
]);

let capture = null;
const pending = new Map();

function normalizePairingCode(value) {
  const [endpoint, token] = value.trim().split('#');
  const parsed = new URL(endpoint);
  if (parsed.protocol !== 'http:' || parsed.hostname !== '127.0.0.1' || !parsed.port || !token) {
    throw new Error('Use the pairing code shown by OpenDownload.');
  }
  return { endpoint: endpoint.replace(/\/$/, ''), token };
}

function mediaType(url, headers) {
  const contentType = (headers.find((header) => header.name.toLowerCase() === 'content-type') || {}).value || '';
  const normalized = contentType.toLowerCase();
  const pathname = new URL(url).pathname.toLowerCase();
  if (normalized.includes('mpegurl') || pathname.endsWith('.m3u8')) return 'hls';
  if (normalized.includes('dash+xml') || pathname.endsWith('.mpd')) return 'dash';
  if (normalized.startsWith('video/')) return 'video';
  if (normalized.startsWith('audio/')) return 'audio';
  if (/\.(mp4|webm|mkv|mov|ts|m4s|mp3|m4a|aac|opus)$/.test(pathname)) return 'video';
  return '';
}

function selectedHeaders(headers) {
  const result = {};
  for (const header of headers || []) {
    const key = header.name.toLowerCase();
    if (allowedHeaders.has(key) && header.value) result[header.name] = header.value;
  }
  return result;
}

async function sendCapture(request, type) {
  if (!capture) return;
  const response = await fetch(`${capture.endpoint}/v1/firefox/streams`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      OpenDownloadSession: capture.token,
    },
    body: JSON.stringify({ url: request.url, type, headers: request.headers }),
  });
  if (!response.ok) {
    capture.error = response.status === 401 ? 'Pairing expired. Start capture again in OpenDownload.' : 'OpenDownload did not accept the stream.';
  }
}

browser.webRequest.onSendHeaders.addListener(
  (details) => {
    if (!capture || details.tabId !== capture.tabId) return;
    pending.set(details.requestId, { url: details.url, headers: selectedHeaders(details.requestHeaders) });
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
    if (type) sendCapture(request, type).catch(() => { capture.error = 'Could not send the stream to OpenDownload.'; });
  },
  { urls: ['<all_urls>'] },
  ['responseHeaders'],
);

browser.webRequest.onErrorOccurred.addListener(
  (details) => pending.delete(details.requestId),
  { urls: ['<all_urls>'] },
);

browser.runtime.onMessage.addListener(async (message) => {
  if (message.type === 'start') {
    const pairing = normalizePairingCode(message.code);
    const [tab] = await browser.tabs.query({ active: true, currentWindow: true });
    if (!tab || tab.id === undefined || !/^https?:/.test(tab.url || '')) throw new Error('Open a website in the active tab before starting capture.');
    pending.clear();
    capture = { ...pairing, tabId: tab.id, error: '' };
    return status();
  }
  if (message.type === 'stop') {
    capture = null;
    pending.clear();
    return status();
  }
  return status();
});

function status() {
  return capture ? { active: true, error: capture.error } : { active: false, error: '' };
}
