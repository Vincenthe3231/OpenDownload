<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useTheme } from './composables/useTheme';
import { ArrowDownTrayIcon, LinkIcon, MoonIcon, SunIcon } from '@heroicons/vue/24/outline';
import DownloadQueue from './components/DownloadQueue.vue';
import SnifferPanel from './components/SnifferPanel.vue';
import { useQueueStore, type DownloadDestinationEvent, type DownloadProgressEvent, type DownloadStateEvent } from './store/queue';
import { useDownloadEngine } from './composables/useDownload';
import { EventsOn } from '../wailsjs/wailsjs/runtime/runtime';

const { isDark, toggleTheme } = useTheme();
const queue = useQueueStore();
const { queueDownload } = useDownloadEngine();
const sourceUrl = ref('');
const outputPath = ref('');
const error = ref('');
const isSubmitting = ref(false);
const isReady = computed(() => sourceUrl.value.trim().length > 0 && !isSubmitting.value);

async function submitDownload() {
  error.value = '';
  const url = sourceUrl.value.trim();
  try { new URL(url); } catch { error.value = 'Paste a complete http or https media URL.'; return; }
  const id = crypto.randomUUID();
  const name = new URL(url).pathname.split('/').filter(Boolean).pop() || 'download';
  queue.addDownload({ id, name });
  isSubmitting.value = true;
  try {
    await queueDownload(id, url, outputPath.value.trim());
    sourceUrl.value = '';
  } catch (reason) {
    queue.updateDownload(id, { progress: 0, status: 'failed' });
    error.value = reason instanceof Error ? reason.message : 'The download could not be started.';
  } finally { isSubmitting.value = false; }
}

let stopProgressListener: (() => void) | undefined;
let stopStateListener: (() => void) | undefined;
let stopDestinationListener: (() => void) | undefined;
onMounted(() => {
  stopProgressListener = EventsOn('download:progress', (progress: DownloadProgressEvent) => queue.updateProgress(progress));
	stopStateListener = EventsOn('download:state', (state: DownloadStateEvent) => queue.updateState(state));
	stopDestinationListener = EventsOn('download:destination', (destination: DownloadDestinationEvent) => queue.updateDestination(destination));
});
onBeforeUnmount(() => { stopProgressListener?.(); stopStateListener?.(); stopDestinationListener?.(); });
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <div class="brand"><span class="brand-mark"><ArrowDownTrayIcon aria-hidden="true" /></span><div><p class="eyebrow">MEDIA UTILITY</p><h1>OpenDownload</h1></div></div>
      <button @click="toggleTheme" class="icon-button" :aria-label="isDark() ? 'Use light theme' : 'Use dark theme'">
        <SunIcon v-if="isDark()" aria-hidden="true" /><MoonIcon v-else aria-hidden="true" />
      </button>
    </header>

    <main class="workspace">
      <section class="download-card" aria-labelledby="download-title">
        <div class="section-heading"><div><p class="eyebrow">NEW DOWNLOAD</p><h2 id="download-title">Save a video from its source</h2></div><LinkIcon class="heading-icon" aria-hidden="true" /></div>
        <form @submit.prevent="submitDownload" novalidate>
          <label for="source-url">Video or stream URL</label>
          <div class="url-row"><input id="source-url" v-model="sourceUrl" type="url" inputmode="url" autocomplete="url" placeholder="https://example.com/video.m3u8" :disabled="isSubmitting" required /><button class="primary-button" type="submit" :disabled="!isReady"><ArrowDownTrayIcon aria-hidden="true" />{{ isSubmitting ? 'Downloading' : 'Download' }}</button></div>
          <label for="output-path">Save to folder <span>optional</span></label>
          <input id="output-path" v-model="outputPath" type="text" autocomplete="off" placeholder="Default: Windows Downloads" :disabled="isSubmitting" />
          <p v-if="error" class="form-error" role="alert">{{ error }}</p>
          <p class="form-help">Leaving this blank saves to your Windows Downloads folder. Supports direct files, HLS playlists, and DASH manifests. Only download media you have permission to save.</p>
        </form>
      </section>
      <section class="queue-card" aria-label="Download activity">
        <DownloadQueue />
      </section>
      <aside class="capture-card">
        <SnifferPanel />
      </aside>
    </main>
  </div>
</template>
