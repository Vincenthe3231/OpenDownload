import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import {
  directMediaExtensions,
  mediaType,
  normalizePairingCode,
  selectedHeaders,
} from '../capture.js';
import { createCaptureController } from '../controller.js';

function fakeApi({ restored } = {}) {
  const storage = restored ? { captureSession: { ...restored } } : {};
  const posts = [];
  return {
    storage,
    posts,
    storageGet: async () => storage,
    storageSet: async (value) => Object.assign(storage, value),
    storageRemove: async (key) => { delete storage[key]; },
    queryActiveTab: async () => [{ id: 7, url: 'https://example.test/watch' }],
    postCapture: async (session, request, type) => {
      posts.push({ session, request, type });
      return { ok: true, status: 201 };
    },
  };
}

test('manifest is a directly loadable MV3 package with observation-only permissions', async () => {
  const manifest = JSON.parse(await readFile(new URL('../manifest.json', import.meta.url), 'utf8'));
  assert.equal(manifest.manifest_version, 3);
  assert.equal(manifest.minimum_chrome_version, '102');
  assert.deepEqual(manifest.permissions.sort(), ['activeTab', 'storage', 'webRequest'].sort());
  assert.deepEqual(manifest.host_permissions.sort(), ['http://*/*', 'https://*/*'].sort());
  assert.equal(manifest.background.type, 'module');
  assert.equal(manifest.background.service_worker, 'background.js');
  assert.ok(!manifest.permissions.includes('debugger'));
  assert.ok(!manifest.permissions.includes('cookies'));
  assert.ok(!manifest.permissions.includes('tabs'));
  assert.ok(!manifest.permissions.includes('webRequestBlocking'));
});

test('pairing validation accepts only the loopback format', () => {
  assert.deepEqual(
    normalizePairingCode('http://127.0.0.1:43123#token_value'),
    { endpoint: 'http://127.0.0.1:43123', token: 'token_value' },
  );
  for (const value of [
    '',
    'https://127.0.0.1:43123#token',
    'http://localhost:43123#token',
    'http://127.0.0.1#token',
    'http://127.0.0.1:43123/path#token',
    'http://127.0.0.1:43123#token with spaces',
  ]) {
    assert.throws(() => normalizePairingCode(value));
  }
});

test('request header allowlist preserves browser-provided capture context', () => {
  const headers = selectedHeaders([
    { name: 'Cookie', value: 'session=secret' },
    { name: 'Referer', value: 'https://example.test/watch' },
    { name: 'Authorization', value: 'Bearer secret' },
    { name: 'X-Untrusted', value: 'discard' },
    { name: 'Empty', value: '' },
  ]);
  assert.deepEqual(headers, {
    Cookie: 'session=secret',
    Referer: 'https://example.test/watch',
    Authorization: 'Bearer secret',
  });
});

test('media classification covers playlists, direct media, audio, and fragments', () => {
  assert.equal(mediaType('https://cdn.test/live/index.m3u8', []), 'hls');
  assert.equal(mediaType('https://cdn.test/live/manifest', [{ name: 'Content-Type', value: 'application/dash+xml' }]), 'dash');
  assert.equal(mediaType('https://cdn.test/audio', [{ name: 'Content-Type', value: 'audio/mp4' }]), 'audio');
  assert.equal(mediaType('https://cdn.test/video.mp4', []), 'video');
  assert.equal(mediaType('https://cdn.test/segment.ts', [{ name: 'Content-Type', value: 'video/mp2t' }]), '');
  assert.ok(directMediaExtensions.has('mp4'));
});

test('capture is isolated to the selected tab and clears on receiver 401', async () => {
  const api = fakeApi();
  api.postCapture = async (session, request, type) => {
    api.posts.push({ session, request, type });
    return { ok: false, status: 401 };
  };
  const controller = createCaptureController(api);
  await controller.initialize();
  await controller.handleMessage({ type: 'start', code: 'http://127.0.0.1:43123#token_value' });

  await controller.onSendHeaders({ tabId: 8, requestId: 'wrong-tab', url: 'https://cdn.test/video.mp4', requestHeaders: [] });
  await controller.onHeadersReceived({ tabId: 8, requestId: 'wrong-tab', statusCode: 200, responseHeaders: [] });
  assert.equal(api.posts.length, 0);

  await controller.onSendHeaders({ tabId: 7, requestId: 'selected-tab', url: 'https://cdn.test/video.mp4', requestHeaders: [{ name: 'Cookie', value: 'secret' }] });
  await controller.onHeadersReceived({ tabId: 7, requestId: 'selected-tab', statusCode: 200, responseHeaders: [] });
  assert.equal(api.posts.length, 1);
  assert.equal(api.storage.captureSession, undefined);
  assert.equal(controller.status().active, false);
  assert.match(controller.status().error, /Pairing expired/);
});

test('service-worker restoration restores pairing metadata without storing headers', async () => {
  const api = fakeApi({ restored: { endpoint: 'http://127.0.0.1:43123', token: 'token_value', tabId: 7, error: '' } });
  const controller = createCaptureController(api);
  await controller.initialize();
  assert.deepEqual(controller.status(), { active: true, error: '' });

  await controller.onSendHeaders({ tabId: 7, requestId: 'restored', url: 'https://cdn.test/video.mp4', requestHeaders: [{ name: 'Cookie', value: 'secret' }] });
  assert.deepEqual(api.storage.captureSession, {
    endpoint: 'http://127.0.0.1:43123',
    token: 'token_value',
    tabId: 7,
    error: '',
  });
  assert.equal(JSON.stringify(api.storage).includes('secret'), false);
});
