export const STORAGE_KEY = 'captureSession';
export const maxPendingRequests = 256;

export const allowedRequestHeaders = new Set([
  'accept',
  'accept-language',
  'authorization',
  'cookie',
  'origin',
  'referer',
  'user-agent',
]);

export const fragmentExtensions = new Set(['cmfa', 'cmfv', 'm4f', 'm4s', 'ts']);
export const directMediaExtensions = new Set(['aac', 'm4a', 'mkv', 'mov', 'mp3', 'mp4', 'opus', 'webm']);

export function normalizePairingCode(value) {
  if (typeof value !== 'string') {
    throw new Error('Use the pairing code shown by OpenDownload.');
  }

  const parts = value.trim().split('#');
  if (parts.length !== 2) {
    throw new Error('Use the pairing code shown by OpenDownload.');
  }

  const [endpoint, token] = parts;
  let parsed;
  try {
    parsed = new URL(endpoint);
  } catch {
    throw new Error('Use the pairing code shown by OpenDownload.');
  }

  if (
    parsed.protocol !== 'http:' ||
    parsed.hostname !== '127.0.0.1' ||
    !parsed.port ||
    parsed.username ||
    parsed.password ||
    (parsed.pathname !== '' && parsed.pathname !== '/') ||
    parsed.search ||
    !token ||
    /\s/.test(token)
  ) {
    throw new Error('Use the pairing code shown by OpenDownload.');
  }

  return { endpoint: endpoint.replace(/\/$/, ''), token };
}

export function isHttpURL(value) {
  return typeof value === 'string' && /^https?:\/\//i.test(value);
}

export function pathnameFromURL(value) {
  try {
    return new URL(value).pathname.toLowerCase();
  } catch {
    return '';
  }
}

export function extensionFromPathname(pathname) {
  const match = pathname.match(/\.([a-z0-9]+)$/);
  return match ? match[1] : '';
}

export function isMediaFragment(pathname, contentType) {
  const extension = extensionFromPathname(pathname);
  return fragmentExtensions.has(extension) || contentType.includes('iso.segment') || contentType.includes('mp2t');
}

export function mediaType(url, headers) {
  const contentType = (headers.find((header) => header.name.toLowerCase() === 'content-type') || {}).value || '';
  const normalized = contentType.toLowerCase();
  const pathname = pathnameFromURL(url);
  if (!pathname || isMediaFragment(pathname, normalized)) return '';
  if (normalized.includes('mpegurl') || pathname.endsWith('.m3u8')) return 'hls';
  if (normalized.includes('dash+xml') || pathname.endsWith('.mpd')) return 'dash';
  if (normalized.startsWith('video/')) return 'video';
  if (normalized.startsWith('audio/')) return 'audio';
  if (directMediaExtensions.has(extensionFromPathname(pathname))) return 'video';
  return '';
}

export function selectedHeaders(headers) {
  const result = {};
  for (const header of headers || []) {
    const key = header.name.toLowerCase();
    if (allowedRequestHeaders.has(key) && header.value) result[header.name] = header.value;
  }
  return result;
}

export function rememberRequest(pending, requestID, request) {
  if (pending.size >= maxPendingRequests) {
    const oldestRequestID = pending.keys().next().value;
    if (oldestRequestID !== undefined) pending.delete(oldestRequestID);
  }
  pending.set(requestID, request);
}
