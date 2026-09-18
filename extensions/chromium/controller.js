import {
  STORAGE_KEY,
  isHttpURL,
  mediaType,
  normalizePairingCode,
  rememberRequest,
  selectedHeaders,
} from './capture.js';

const pairingExpiredMessage = 'Pairing expired or was replaced. Paste the fresh code from OpenDownload and select Start capture.';
const automaticCaptureMessage = 'Automatic capture is unavailable. Repair the OpenDownload native host or use manual pairing.';

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
        mode: capture.mode || 'manual',
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
    return capture ? { active: true, mode: capture.mode || 'manual', error: capture.error || '' } : { active: false, mode: 'manual', error: lastError };
  }

  async function start(code, manual = false) {
    await ready;
    const tabs = await api.queryActiveTab();
    const tab = tabs && tabs[0];
    if (!tab || !Number.isInteger(tab.id) || !isHttpURL(tab.url || '')) {
      throw new Error('Open a website in the active tab before starting capture.');
    }

    pending.clear();
    lastError = '';
	if (!manual && api.nativeConnect && api.nativeRequest) {
		try {
			const port = api.nativeConnect();
			const response = await api.nativeRequest(port, { type: 'start', browser: api.browserFamily || 'chromium', tabId: tab.id });
			if (!response || response.type !== 'started') throw new Error(response && response.error ? response.error : automaticCaptureMessage);
			capture = { mode: 'automatic', tabId: tab.id, port, error: '' };
			return status();
		} catch (reason) {
			lastError = reason instanceof Error ? reason.message : automaticCaptureMessage;
		}
	}

	if (!code) throw new Error(lastError || automaticCaptureMessage);
	const pairing = normalizePairingCode(code);
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
    if (message && message.type === 'start') return start(message.code, message.manual === true);
    if (message && message.type === 'stop') {
		if (capture && capture.mode === 'automatic' && api.nativeRequest) {
			try { await api.nativeRequest(capture.port, { type: 'stop' }); } catch { /* host already disconnected */ }
			if (capture.port && typeof capture.port.disconnect === 'function') capture.port.disconnect();
		}
      await clearSession();
      return status();
    }
    return status();
  }

  function handleNativeDisconnect(port) {
    if (!capture || capture.mode !== 'automatic' || capture.port !== port) return;
    capture = null;
    pending.clear();
    lastError = automaticCaptureMessage;
    void api.storageRemove(STORAGE_KEY);
  }

  return {
    initialize,
    handleMessage,
    onSendHeaders,
    onHeadersReceived,
    onErrorOccurred,
    handleNativeDisconnect,
    status,
    getPendingSize: () => pending.size,
    getLastError: () => lastError,
    formatError: (reason) => errorMessage(reason, 'Could not communicate with the OpenDownload extension.'),
  };
}
