<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { ArrowDownTrayIcon, ClipboardDocumentIcon, PlayIcon, StopIcon } from '@heroicons/vue/24/outline';
import { useQueueStore } from '../store/queue';
import { useCapture } from '../composables/useCapture';

const queue = useQueueStore();
const { startCapture, stopCapture, refreshStreams, downloadCaptured } = useCapture();
const pairingCode = ref('');
const error = ref('');
const loading = ref(false);

async function refresh() {
  try {
    queue.setDetected(await refreshStreams());
  } catch {
    error.value = 'Could not refresh captured streams.';
  }
}

async function start() {
  error.value = '';
  loading.value = true;
  try {
    const pairing = await startCapture();
    pairingCode.value = pairing.code;
    await refresh();
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Could not start Firefox capture.';
  } finally {
    loading.value = false;
  }
}

async function stop() {
  await stopCapture();
  pairingCode.value = '';
  queue.setDetected([]);
}

async function copyPairingCode() {
  try {
    await navigator.clipboard.writeText(pairingCode.value);
  } catch {
    error.value = 'Copy the pairing code directly from the field.';
  }
}

async function download(stream: typeof queue.detectedStreams[number]) {
  error.value = '';
  const id = crypto.randomUUID();
  queue.addDownload({ id, name: stream.name });
  try {
    await downloadCaptured(id, stream.id, '');
    queue.updateDownload(id, { progress: 100, status: 'completed' });
  } catch (reason) {
    queue.updateDownload(id, { progress: 0, status: 'failed' });
    error.value = reason instanceof Error ? reason.message : 'Could not download the captured stream.';
  }
}

let poll: number | undefined;
onMounted(() => {
  poll = window.setInterval(() => {
    if (pairingCode.value) refresh();
  }, 1500);
});
onBeforeUnmount(() => {
  if (poll !== undefined) window.clearInterval(poll);
});
</script>

<template>
  <div class="panel-content">
    <div class="panel-heading"><div><p class="eyebrow">CAPTURE</p><h2>Detected streams</h2></div><span class="count-badge">{{ queue.detectedStreams.length }}</span>
    </div>
    <div v-if="!pairingCode" class="empty-state capture-empty"><p>Capture from Firefox or Zen</p><span>Select Generate Pair Address. OpenDownload will show a address here to copy into the temporary browser add on.</span><button class="capture-action" type="button" :disabled="loading" @click="start"><PlayIcon aria-hidden="true" />{{ loading ? 'Starting' : 'Generate Pair Address' }}</button>
    </div>
    <div v-else class="capture-active">
      <div class="pairing-row"><input aria-label="Firefox pairing code" readonly :value="pairingCode" /><button class="icon-button compact" type="button" title="Copy pairing code" aria-label="Copy pairing code" @click="copyPairingCode"><ClipboardDocumentIcon aria-hidden="true" /></button><button class="icon-button compact" type="button" title="Stop capture" aria-label="Stop capture" @click="stop"><StopIcon aria-hidden="true" /></button></div>
      <p class="capture-help">Paste this URL into the add on, then play media in its active tab.</p>
      <ul class="stream-list" v-if="queue.detectedStreams.length"><li v-for="stream in queue.detectedStreams" :key="stream.id"><div class="stream-summary"><strong>{{ stream.name }}</strong><span>{{ stream.host }} · {{ stream.type }}</span></div><button class="icon-button compact" type="button" title="Download captured stream" :aria-label="`Download ${stream.name}`" @click="download(stream)"><ArrowDownTrayIcon aria-hidden="true" /></button></li>
      </ul>
      <div v-else class="empty-state capture-waiting"><p>Waiting for media</p><span>Only requests from the tab selected in the add on are captured.</span></div>
    </div>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
  </div>
</template>
