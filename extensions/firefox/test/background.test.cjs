const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const backgroundSource = fs.readFileSync(path.join(__dirname, '..', 'background.js'), 'utf8');

function createEvent() {
  const listeners = new Set();
  return {
    addListener(listener) {
      listeners.add(listener);
    },
    removeListener(listener) {
      listeners.delete(listener);
    },
    emit(...args) {
      for (const listener of [...listeners]) listener(...args);
    },
    get listeners() {
      return [...listeners];
    },
  };
}

function createHarness({ startResponse = { type: 'started', diagnosticMode: false }, connectError, tab = { id: 7, url: 'https://example.test/watch' }, fetchImpl } = {}) {
  const onSendHeaders = createEvent();
  const onHeadersReceived = createEvent();
  const onErrorOccurred = createEvent();
  const onRuntimeMessage = createEvent();
  const posts = [];
  const fetchCalls = [];
  let connectCalls = 0;
  let port;

  const responseFor = (message) => {
    if (message.type === 'start') return startResponse;
    if (message.type === 'stream') return { type: 'accepted' };
    if (message.type === 'stop') return { type: 'stopped' };
    return undefined;
  };

  port = {
    onMessage: createEvent(),
    onDisconnect: createEvent(),
    postMessage(message) {
      posts.push(message);
      const response = responseFor(message);
      if (response !== undefined) queueMicrotask(() => port.onMessage.emit(response));
    },
    disconnect() {
      port.onDisconnect.emit({});
    },
  };

  const browser = {
    tabs: {
      query: async () => [tab],
    },
    runtime: {
      onMessage: onRuntimeMessage,
      connectNative() {
        connectCalls += 1;
        if (connectError) throw connectError;
        return port;
      },
    },
    webRequest: {
      onSendHeaders,
      onHeadersReceived,
      onErrorOccurred,
    },
  };

  const context = vm.createContext({
    browser,
    CaptureConstants: {
      allowedRequestHeaders: new Set(['accept', 'accept-language', 'authorization', 'cookie', 'origin', 'referer', 'user-agent']),
      fragmentExtensions: new Set(['cmfa', 'cmfv', 'm4f', 'm4s', 'ts']),
      directMediaExtensions: new Set(['aac', 'm4a', 'mkv', 'mov', 'mp3', 'mp4', 'opus', 'webm']),
      maxPendingRequests: 256,
    },
    URL,
    Promise,
    console,
    fetch: async (url, options) => {
      fetchCalls.push({ url, options });
      if (fetchImpl) return fetchImpl(url, options);
      return url.endsWith('/capture-options')
        ? { ok: true, status: 200, json: async () => ({ protocolVersion: 1, diagnosticMode: true }) }
        : { ok: true, status: 201 };
    },
    setTimeout,
    clearTimeout,
    queueMicrotask,
  });
  vm.runInContext(backgroundSource, context, { filename: 'background.js' });

  return {
    events: { onSendHeaders, onHeadersReceived, onErrorOccurred },
    posts,
    fetchCalls,
    port,
    get connectCalls() {
      return connectCalls;
    },
    async send(message) {
      const handler = onRuntimeMessage.listeners[0];
      assert.ok(handler, 'Firefox background message handler should be registered');
      return handler(message);
    },
  };
}

async function flushCapture() {
  await new Promise((resolve) => setImmediate(resolve));
  await new Promise((resolve) => setImmediate(resolve));
}

function plain(value) {
  return JSON.parse(JSON.stringify(value));
}

function mediaRequest(overrides = {}) {
  return {
    tabId: 7,
    requestId: 'request-1',
    url: 'https://example.test/media.mp4',
    method: 'GET',
    type: 'media',
    timeStamp: 123.5,
    frameId: 0,
    parentFrameId: -1,
    documentUrl: 'https://example.test/watch',
    originUrl: 'https://example.test',
    initiator: 'https://example.test',
    requestHeaders: [
      { name: 'Authorization', value: 'Bearer secret' },
      { name: 'Cookie', value: 'session=secret' },
      { name: 'X-Raw-Diagnostic', value: 'kept only in developer diagnostics' },
    ],
    ...overrides,
  };
}

function mediaResponse(overrides = {}) {
  return {
    tabId: 7,
    requestId: 'request-1',
    url: 'https://example.test/media.mp4',
    statusCode: 206,
    statusLine: 'HTTP/1.1 206 Partial Content',
    fromCache: false,
    ip: '203.0.113.10',
    responseHeaders: [
      { name: 'Content-Type', value: 'video/mp4' },
      { name: 'Set-Cookie', value: 'response-secret=secret' },
    ],
    ...overrides,
  };
}

test('manual pairing latches authenticated diagnostic options for stream capture', async () => {
  const harness = createHarness();
  const status = await harness.send({ type: 'start', manual: true, code: 'http://127.0.0.1:43127#test-token' });

  assert.deepEqual(plain(status), { active: true, mode: 'manual', error: '' });
  assert.equal(harness.fetchCalls.length, 1);
  assert.equal(harness.fetchCalls[0].url, 'http://127.0.0.1:43127/v1/firefox/capture-options');
  assert.equal(harness.fetchCalls[0].options.headers.OpenDownloadSession, 'test-token');

  harness.events.onSendHeaders.emit(mediaRequest());
  harness.events.onHeadersReceived.emit(mediaResponse());
  await flushCapture();

  const streamCall = harness.fetchCalls.find((call) => call.url.endsWith('/streams'));
  assert.ok(streamCall);
  const body = JSON.parse(streamCall.options.body);
  assert.equal(body.headers.Authorization, 'Bearer secret');
  assert.equal(body.debug.requestId, 'request-1');
  assert.equal(body.debug.responseStatus, 206);
  assert.equal(body.debug.requestHeaders['X-Raw-Diagnostic'], 'kept only in developer diagnostics');
});

test('manual pairing stays safe when diagnostic options fetch fails', async () => {
  const harness = createHarness({
    fetchImpl: async (url) => {
      if (url.endsWith('/capture-options')) throw new Error('options unavailable');
      return { ok: true, status: 201 };
    },
  });
  const status = await harness.send({ type: 'start', manual: true, code: 'http://127.0.0.1:43127#test-token' });

  assert.deepEqual(plain(status), { active: true, mode: 'manual', error: '' });
  harness.events.onSendHeaders.emit(mediaRequest());
  harness.events.onHeadersReceived.emit(mediaResponse());
  await flushCapture();

  const streamCall = harness.fetchCalls.find((call) => call.url.endsWith('/streams'));
  assert.ok(streamCall);
  assert.equal(JSON.parse(streamCall.options.body).debug, undefined);
});

test('automatic start requires no pairing code and sends protocol version 1', async () => {
  const harness = createHarness({ startResponse: { type: 'started', diagnosticMode: false } });

  const status = await harness.send({ type: 'start', manual: false });

  assert.deepEqual(plain(status), { active: true, mode: 'automatic', error: '' });
  assert.equal(harness.connectCalls, 1);
  assert.deepEqual(plain(harness.posts[0]), {
    protocolVersion: 1,
    type: 'start',
    browser: 'firefox',
    tabId: 7,
  });
});

test('manual pairing is available only through the explicit manual action', async () => {
  const harness = createHarness({ connectError: new Error('native host should not be called') });

  const status = await harness.send({
    type: 'start',
    manual: true,
    code: 'http://127.0.0.1:4567#pairing-token',
  });

  assert.deepEqual(plain(status), { active: true, mode: 'manual', error: '' });
  assert.equal(harness.connectCalls, 0);
});

test('automatic start does not fall back to a supplied pairing code', async () => {
  const harness = createHarness({ connectError: new Error('native host missing') });

  await assert.rejects(
    () => harness.send({
      type: 'start',
      manual: false,
      code: 'http://127.0.0.1:4567#pairing-token',
    }),
    /Automatic capture is unavailable/,
  );

  assert.deepEqual(plain(await harness.send({ type: 'status' })), {
    active: false,
    mode: 'manual',
    error: 'Automatic capture is unavailable. Open OpenDownload, then repair the native host or use manual pairing.',
  });
});

test('structured native failures expose only their safe user message', async () => {
  const harness = createHarness({
    startResponse: {
      type: 'error',
      failure: {
        code: 'APP_NOT_RUNNING',
        userMessage: 'Open OpenDownload, then try again.',
        retryable: true,
        diagnosticId: 'opaque-id',
      },
    },
  });

  await assert.rejects(
    () => harness.send({ type: 'start', manual: false }),
    /Open OpenDownload, then try again\./,
  );
  assert.equal((await harness.send({ type: 'status' })).error, 'Open OpenDownload, then try again.');
});

test('diagnostic mode sends raw request and response context only when enabled', async () => {
  const harness = createHarness({ startResponse: { type: 'started', diagnosticMode: true } });
  await harness.send({ type: 'start', manual: false });

  harness.events.onSendHeaders.emit(mediaRequest());
  harness.events.onHeadersReceived.emit(mediaResponse());
  await flushCapture();

  const stream = harness.posts.find((message) => message.type === 'stream');
  assert.ok(stream);
  assert.equal(stream.protocolVersion, 1);
  assert.equal(stream.stream.headers.Authorization, 'Bearer secret');
  assert.equal(stream.debug.requestUrl, 'https://example.test/media.mp4');
  assert.equal(stream.debug.requestHeaders['X-Raw-Diagnostic'], 'kept only in developer diagnostics');
  assert.equal(stream.debug.responseHeaders['Set-Cookie'], 'response-secret=secret');
  assert.equal(stream.debug.responseStatus, 206);
});

test('diagnostic mode off sends selected headers without a debug payload', async () => {
  const harness = createHarness({ startResponse: { type: 'started', diagnosticMode: false } });
  await harness.send({ type: 'start', manual: false });

  harness.events.onSendHeaders.emit(mediaRequest());
  harness.events.onHeadersReceived.emit(mediaResponse());
  await flushCapture();

  const stream = harness.posts.find((message) => message.type === 'stream');
  assert.ok(stream);
  assert.equal(Object.hasOwn(stream, 'debug'), false);
  assert.deepEqual(plain(stream.stream.headers), {
    Authorization: 'Bearer secret',
    Cookie: 'session=secret',
  });
});

test('wrong-tab requests are ignored and host disconnect clears automatic capture', async () => {
  const harness = createHarness({ startResponse: { type: 'started', diagnosticMode: false } });
  await harness.send({ type: 'start', manual: false });

  harness.events.onSendHeaders.emit(mediaRequest({ tabId: 99, requestId: 'wrong-tab' }));
  harness.events.onHeadersReceived.emit(mediaResponse({ tabId: 99, requestId: 'wrong-tab' }));
  await flushCapture();
  assert.equal(harness.posts.some((message) => message.type === 'stream'), false);

  harness.port.onDisconnect.emit({});
  assert.deepEqual(plain(await harness.send({ type: 'status' })), {
    active: false,
    mode: 'manual',
    error: 'Automatic capture host disconnected. Repair the native host or use manual pairing.',
  });
});
