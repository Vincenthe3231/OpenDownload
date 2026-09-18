import {
  STORAGE_KEY,
  isHttpURL,
  mediaType,
  normalizePairingCode,
  rememberRequest,
  selectedHeaders,
} from './capture.js';

const pairingExpiredMessage = 'Pairing expired or was replaced. Paste the fresh code from OpenDownload and select Start capture.';

function errorMessage(reason, fallback) {
  return reason instanceof Error ? reason.message : fallback;
}

function storedCapture(value) {
  if (typeof value !== 'object' || value === null) return null;
  const candidate = value;
  if (
    typeof candidate.endpoint !== 'string' ||
    typeof candidate.token !== 'string' ||
    !candidate.token ||
    !Number.isInteger(candidate.tabId) ||
    candidate.tabId < 0 ||
    (candidate.error !== undefined && typeof candidate.error !== 'string')
  ) {
    return null;
  }
  return {
    endpoint: candidate.endpoint,
    token: candidate.token,
    tabId: candidate.tabId,
    error: candidate.error || '',
  };
}

export function createCaptureController(api) {
  let capture = null;
  let lastError = '';
  const pending = new Map();
  let ready = Promise.resolve();

  async function persist() {
    if (!capture) return;
    await api.storageSet({
      [STORAGE_KEY]: {
        endpoint: capture.endpoint,
        token: capture.token,
        tabId: capture.tabId,
        error: capture.error || '',
      },
    });
  }

  async function clearSession(error = '') {
    capture = null;
    pending.clear();
    lastError = error;
    await api.storageRemove(STORAGE_KEY);
  }

  async function initialize() {
    ready = Promise.resolve(api.storageGet(STORAGE_KEY)).then((result) => {
      const restored = storedCapture(result && result[STORAGE_KEY]);
      capture = restored;
      lastError = '';
    });
    await ready;
  }

  function status() {
    return capture ? { active: true, error: capture.error || '' } : { active: false, error: lastError };
  }

  async function start(code) {
    await ready;
    const pairing = normalizePairingCode(code);
    const tabs = await api.queryActiveTab();
    const tab = tabs && tabs[0];
    if (!tab || !Number.isInteger(tab.id) || !isHttpURL(tab.url || '')) {
      throw new Error('Open a website in the active tab before starting capture.');
    }

    pending.clear();
    lastError = '';
    capture = { ...pairing, tabId: tab.id, error: '' };
    await persist();
    return status();
  }

  async function sendCapture(request, type) {
    const session = capture;
    if (!session) return;

    try {
      const response = await api.postCapture(session, request, type);
      if (response.ok || capture !== session) return;
      if (response.status === 401) {
        await clearSession(pairingExpiredMessage);
        return;
      }
      session.error = 'OpenDownload did not accept the stream.';
      await persist();
    } catch {
      if (capture === session) {
        session.error = 'Could not send the stream to OpenDownload.';
        await persist();
      }
    }
  }

  async function onSendHeaders(details) {
    await ready;
    if (!capture || details.tabId !== capture.tabId || !isHttpURL(details.url)) return;
    rememberRequest(pending, details.requestId, {
      url: details.url,
      headers: selectedHeaders(details.requestHeaders),
    });
  }

  async function onHeadersReceived(details) {
    await ready;
    const request = pending.get(details.requestId);
    pending.delete(details.requestId);
    if (
      !capture ||
      !request ||
      details.tabId !== capture.tabId ||
      details.statusCode < 200 ||
      details.statusCode >= 300
    ) {
      return;
    }

    const type = mediaType(request.url, details.responseHeaders || []);
    if (type) await sendCapture(request, type);
  }

  function onErrorOccurred(details) {
    pending.delete(details.requestId);
  }

  async function handleMessage(message) {
    await ready;
    if (message && message.type === 'start') return start(message.code);
    if (message && message.type === 'stop') {
      await clearSession();
      return status();
    }
    return status();
  }

  return {
    initialize,
    handleMessage,
    onSendHeaders,
    onHeadersReceived,
    onErrorOccurred,
    status,
    getPendingSize: () => pending.size,
    getLastError: () => lastError,
    formatError: (reason) => errorMessage(reason, 'Could not communicate with the OpenDownload extension.'),
  };
}
