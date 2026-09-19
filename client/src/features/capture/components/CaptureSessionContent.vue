<script setup lang="ts">
import {
  ArrowDownTrayIcon,
  ChevronDownIcon,
  ClipboardDocumentIcon,
  PlayIcon,
} from '@heroicons/vue/24/outline';
import type { CaptureSessionSnapshot, CaptureStreamSummary } from '../types';

interface Props {
  session: CaptureSessionSnapshot;
  pairingCode: string;
  expiresText: string;
  pairingExpired: boolean;
  loading: boolean;
  streams: CaptureStreamSummary[];
  diagnosticsEnabled: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  (event: 'start'): void;
  (event: 'copy-pairing-code'): void;
  (event: 'download', stream: CaptureStreamSummary): void;
  (event: 'technical-details', streamID: string): void;
}>();
</script>

<template>
  <div v-if="!session.active" class="empty-state capture-empty">
    <p>Capture from Firefox or Zen</p>
    <span>Open OpenDownload Capture in Firefox or Zen, then start capture. Manual pairing is available only as a recovery path.</span>
    <button class="capture-action" type="button" :disabled="loading" @click="emit('start')">
      <PlayIcon aria-hidden="true" />
      {{ loading ? 'Generating pairing code' : 'Use manual pairing' }}
    </button>
  </div>

  <div v-else class="capture-active">
    <div v-if="session.mode === 'automatic'" class="capture-help">
      Automatic capture · {{ session.nativeStatus }}
      <span v-if="session.browser"> · {{ session.browser }}</span>
      <span v-if="session.tabId"> · selected tab {{ session.tabId }}</span>
    </div>

    <div v-if="session.mode !== 'automatic'" class="pairing-row">
      <input aria-label="Browser pairing code" readonly :value="pairingCode" />
      <button
        class="icon-button compact"
        type="button"
        title="Copy pairing code"
        aria-label="Copy pairing code"
        @click="emit('copy-pairing-code')"
      >
        <ClipboardDocumentIcon aria-hidden="true" />
      </button>
    </div>

    <p v-if="session.mode !== 'automatic'" class="capture-help">
      Paste this pairing code into the Firefox or Zen extension, then play media in its active tab. {{ expiresText }}
    </p>

    <button
      v-if="session.mode !== 'automatic'"
      class="capture-action"
      type="button"
      :disabled="loading"
      @click="emit('start')"
    >
      <PlayIcon aria-hidden="true" />
      {{ loading ? 'Generating' : pairingExpired ? 'Generate fresh pairing code' : 'Refresh pairing code' }}
    </button>

    <ul v-if="streams.length" class="stream-list">
      <li v-for="stream in streams" :key="stream.id">
        <div class="stream-summary">
          <strong>{{ stream.name }}</strong>
          <span>{{ stream.host }} · {{ stream.type }}</span>
        </div>
        <div class="stream-actions">
          <button
            class="icon-button compact"
            type="button"
            title="Download captured stream"
            :aria-label="`Download ${stream.name}`"
            @click="emit('download', stream)"
          >
            <ArrowDownTrayIcon aria-hidden="true" />
          </button>
          <button
            v-if="diagnosticsEnabled"
            class="technical-toggle"
            type="button"
            aria-haspopup="dialog"
            @click="emit('technical-details', stream.id)"
          >
            Technical details
            <ChevronDownIcon aria-hidden="true" />
          </button>
        </div>
      </li>
    </ul>

    <div v-else class="empty-state capture-waiting">
      <p>Waiting for media</p>
      <span>Only requests from the tab selected in the Firefox or Zen extension are captured.</span>
    </div>
  </div>
</template>
