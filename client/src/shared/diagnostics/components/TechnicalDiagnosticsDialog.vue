<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';
import { ExclamationTriangleIcon, XMarkIcon } from '@heroicons/vue/24/outline';
import type { TechnicalDiagnostic } from '../technical';

interface Props {
  open: boolean;
  title: string;
  subtitle: string;
  detail?: TechnicalDiagnostic | null;
  loading: boolean;
  emptyMessage?: string;
}

const props = withDefaults(defineProps<Props>(), {
  emptyMessage: 'No technical context was retained.',
});

const emit = defineEmits<{
  (event: 'close'): void;
}>();

const closeButton = ref<HTMLButtonElement>();
const titleID = `technical-dialog-title-${Math.random().toString(36).slice(2)}`;

watch(
  () => props.open,
  async (open) => {
    if (!open) return;
    await nextTick();
    closeButton.value?.focus();
  },
);

function formatHeaders(headers?: Record<string, string>): string {
  if (!headers || Object.keys(headers).length === 0) return '';
  return Object.entries(headers).map(([key, value]) => `${key}: ${value}`).join('\n');
}

interface TechnicalRow {
  label: string;
  value: string;
}

function technicalRows(detail: TechnicalDiagnostic): TechnicalRow[] {
  const rows: TechnicalRow[] = [];
  const add = (label: string, value: unknown) => {
    if (value !== undefined && value !== null && value !== '') rows.push({ label, value: String(value) });
  };
  add('Diagnostic ID', detail.diagnosticId);
  add('Occurred at', detail.occurredAt);
  add('Raw error', detail.rawError);
  add('Request ID', detail.requestId);
  add('Request method', detail.requestMethod);
  add('Request type', detail.requestType);
  add('Request timestamp', detail.requestTimestamp);
  add('Request frame', detail.requestFrameId);
  add('Parent frame', detail.requestParentFrameId);
  add('Request URL', detail.requestUrl);
  add('Document URL', detail.requestDocumentUrl);
  add('Origin URL', detail.requestOriginUrl);
  add('Initiator', detail.requestInitiator);
  add('Request headers', formatHeaders(detail.requestHeaders));
  add('Response headers', formatHeaders(detail.responseHeaders));
  add('Response status', detail.responseStatusLine);
  add('Response from cache', detail.responseFromCache);
  add('Response IP', detail.responseIp);
  add('Native host detail', detail.nativeHostDetail);
  add('Pipe detail', detail.pipeDetail);
  add('HTTP status', detail.httpStatus);
  return rows;
}
</script>

<template>
  <dialog
    v-if="open"
    class="technical-dialog"
    open
    aria-modal="true"
    :aria-labelledby="titleID"
    @click.self="emit('close')"
    @keydown.esc.prevent="emit('close')"
  >
    <div class="technical-dialog-shell">
      <header class="technical-dialog-header">
        <div>
          <p class="eyebrow">IN-MEMORY REPORT</p>
          <h3 :id="titleID">{{ title }}</h3>
          <p class="technical-dialog-subtitle">{{ subtitle }}</p>
        </div>
        <button
          ref="closeButton"
          class="icon-button"
          type="button"
          aria-label="Close technical details"
          title="Close technical details"
          @click="emit('close')"
        >
          <XMarkIcon aria-hidden="true" />
        </button>
      </header>

      <div class="technical-dialog-body">
        <p v-if="loading" class="technical-loading">Loading in-memory technical context...</p>
        <p v-else-if="detail === null" class="technical-empty">{{ emptyMessage }}</p>
        <template v-else-if="detail">
          <p class="diagnostics-warning technical-dialog-warning" role="note">
            <ExclamationTriangleIcon aria-hidden="true" />
            This report may contain cookies, tokens, headers, URLs, and local transport details. It is not copied or persisted.
          </p>
          <dl class="technical-fields">
            <template v-for="row in technicalRows(detail)" :key="row.label">
              <div class="technical-field">
                <dt>{{ row.label }}</dt>
                <dd><pre>{{ row.value }}</pre></dd>
              </div>
            </template>
          </dl>
          <p v-if="detail.truncated" class="technical-truncated">Some values were truncated to stay within the in-memory diagnostic limit.</p>
        </template>
        <p v-else class="technical-empty">Technical context is not available yet.</p>
      </div>
    </div>
  </dialog>
</template>
