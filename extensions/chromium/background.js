import { createCaptureController } from './controller.js';

const controller = createCaptureController({
  storageGet: (key) => chrome.storage.session.get(key),
  storageSet: (value) => chrome.storage.session.set(value),
  storageRemove: (key) => chrome.storage.session.remove(key),
  queryActiveTab: () => new Promise((resolve) => {
    chrome.tabs.query({ active: true, currentWindow: true }, resolve);
  }),
  postCapture: (session, request, type) => fetch(`${session.endpoint}/v1/firefox/streams`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      OpenDownloadSession: session.token,
    },
    body: JSON.stringify({ url: request.url, type, headers: request.headers }),
  }).then((response) => ({ ok: response.ok, status: response.status })),
});

// Register listeners synchronously. Each handler waits for session restoration
// before touching capture state, so worker startup cannot lose the pairing.
chrome.webRequest.onSendHeaders.addListener(
  (details) => { void controller.onSendHeaders(details); },
  { urls: ['http://*/*', 'https://*/*'] },
  ['requestHeaders', 'extraHeaders'],
);

chrome.webRequest.onHeadersReceived.addListener(
  (details) => { void controller.onHeadersReceived(details); },
  { urls: ['http://*/*', 'https://*/*'] },
  ['responseHeaders', 'extraHeaders'],
);

chrome.webRequest.onErrorOccurred.addListener(
  (details) => controller.onErrorOccurred(details),
  { urls: ['http://*/*', 'https://*/*'] },
);

void controller.initialize();

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  controller.handleMessage(message).then(sendResponse).catch((reason) => {
    sendResponse({ active: false, error: controller.formatError(reason) });
  });
  return true;
});
