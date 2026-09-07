<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { ArrowDownTrayIcon, ClipboardDocumentIcon, PlayIcon } from '@heroicons/vue/24/outline';
import { queueCapturedStream, startFirefoxCapture } from '../api';
import { subscribeToCaptureEvents, type CaptureEventSubscription } from '../events';
import { useCaptureStore } from '../store';
import { useDownloadStore } from '../../downloads/store';
import type { CaptureStreamSummary } from '../types';
import { formatDateTime } from '../../../shared/formatters/dateTime';

const capture = useCaptureStore();
const downloads = useDownloadStore();
const pairingCode = ref('');
const pairingExpiresAt = ref('');
const currentTime = ref(Date.now());
const error = ref('');
const loading = ref(false);
let eventSubscription: CaptureEventSubscription | undefined;
let expiryTimer: ReturnType<typeof setInterval> | undefined;

const isCaptureActive = computed(() => capture.session.active && pairingCode.value !== '');
const pairingExpired = computed(() => {
  const expiresAt = Date.parse(pairingExpiresAt.value);
  return !Number.isNaN(expiresAt) && currentTime.value >= expiresAt;
});
const expiresText = computed(() => {
  if (!pairingExpiresAt.value) return '';
  return pairingExpired.value
    ? 'This pairing code has expired. Generate a fresh code below without restarting OpenDownload.'
    : `Pairing code expires at ${formatDateTime(pairingExpiresAt.value)}.`;
});

watch(
  () => capture.session.active,
  (active) => {
    if (!active) {
      pairingCode.value = '';
      pairingExpiresAt.value = '';
    }
  },
);

async function start(): Promise<void> {
  error.value = '';
  loading.value = true;
  try {
    const pairing = await startFirefoxCapture();
    pairingCode.value = pairing.code;
    pairingExpiresAt.value = pairing.expiresAt;
    capture.setSession({ active: true, paired: false, expiresAt: pairing.expiresAt });
    await eventSubscription?.hydrate();
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Could not start Firefox capture.';
  } finally {
    loading.value = false;
  }
}

async function copyPairingCode(): Promise<void> {
  try {
    await navigator.clipboard.writeText(pairingCode.value);
  } catch {
    error.value = 'Copy the pairing code directly from the field.';
  }
}

async function download(stream: CaptureStreamSummary): Promise<void> {
  error.value = '';
  const id = crypto.randomUUID();
  try {
    downloads.apply(await queueCapturedStream({ id, capturedStreamId: stream.id, outputDir: '' }));
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Could not download the captured stream.';
  }
}

onMounted(() => {
  expiryTimer = setInterval(() => {
    currentTime.value = Date.now();
  }, 1000);
  eventSubscription = subscribeToCaptureEvents(capture);
  void eventSubscription.hydrate().catch(() => {
    error.value = 'Could not restore captured streams.';
  });
});

onBeforeUnmount(() => {
  if (expiryTimer !== undefined) clearInterval(expiryTimer);
  eventSubscription?.dispose();
});
</script>

<template>
  <div class="panel-content">
    <div class="panel-heading"><div><p class="eyebrow">CAPTURE</p><h2>Detected streams</h2></div><span class="count-badge">{{ capture.streams.length }}</span>
    </div>
    <div v-if="!isCaptureActive" class="empty-state capture-empty"><p>Capture from Firefox or Zen</p><span>Generate a pairing code, then paste it into the temporary browser add on for the selected tab.</span><button class="capture-action" type="button" :disabled="loading" @click="start"><PlayIcon aria-hidden="true" />{{ loading ? 'Starting' : 'Generate pairing code' }}</button>
    </div>
    <div v-else class="capture-active">
      <div class="pairing-row"><input aria-label="Firefox pairing code" readonly :value="pairingCode" /><button class="icon-button compact" type="button" title="Copy pairing code" aria-label="Copy pairing code" @click="copyPairingCode"><ClipboardDocumentIcon aria-hidden="true" /></button></div>
      <p class="capture-help">Paste this pairing code into the add on, then play media in its active tab. {{ expiresText }}</p>
      <button class="capture-action" type="button" :disabled="loading" @click="start"><PlayIcon aria-hidden="true" />{{ loading ? 'Generating' : pairingExpired ? 'Generate fresh pairing code' : 'Refresh pairing code' }}</button>
      <ul v-if="capture.streams.length" class="stream-list"><li v-for="stream in capture.streams" :key="stream.id"><div class="stream-summary"><strong>{{ stream.name }}</strong><span>{{ stream.host }} · {{ stream.type }}</span></div><button class="icon-button compact" type="button" title="Download captured stream" :aria-label="`Download ${stream.name}`" @click="download(stream)"><ArrowDownTrayIcon aria-hidden="true" /></button></li>
      </ul>
      <div v-else class="empty-state capture-waiting"><p>Waiting for media</p><span>Only requests from the tab selected in the add on are captured.</span></div>
    </div>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
  </div>
</template>
