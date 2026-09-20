<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { developerDiagnosticsEnabled } from '../../capture/api';
import type { DownloadDiagnostic } from '../types';

const props = defineProps<{
  diagnostic: DownloadDiagnostic;
}>();

const diagnosticsEnabled = ref(false);
const safeContextEntries = computed(() => Object.entries(props.diagnostic.safeContext ?? {}));
let diagnosticsTimer: ReturnType<typeof setInterval> | undefined;

async function refreshDiagnosticsState(): Promise<void> {
  try {
    diagnosticsEnabled.value = await developerDiagnosticsEnabled();
  } catch {
    diagnosticsEnabled.value = false;
  }
}

onMounted(() => {
  void refreshDiagnosticsState();
  diagnosticsTimer = setInterval(() => {
    void refreshDiagnosticsState();
  }, 1500);
});

onBeforeUnmount(() => {
  if (diagnosticsTimer !== undefined) clearInterval(diagnosticsTimer);
});
</script>

<template>
  <div class="technical-details" :aria-label="`Download diagnostic ${diagnostic.diagnosticId}`">
    <p class="diagnostics-warning" role="alert">{{ diagnostic.userMessage }}</p>
    <p class="progress-detail">Diagnostic ID: <code>{{ diagnostic.diagnosticId }}</code></p>

    <details v-if="diagnosticsEnabled">
      <summary class="technical-toggle">Technical details</summary>
      <dl class="technical-fields">
        <div>
          <dt>Code</dt>
          <dd><code>{{ diagnostic.code }}</code></dd>
        </div>
        <div>
          <dt>Stage</dt>
          <dd><code>{{ diagnostic.stage }}</code></dd>
        </div>
        <div>
          <dt>Retryable</dt>
          <dd>{{ diagnostic.retryable ? 'Yes' : 'No' }}</dd>
        </div>
        <div>
          <dt>Occurred</dt>
          <dd><code>{{ diagnostic.occurredAt }}</code></dd>
        </div>
        <template v-for="[key, value] in safeContextEntries" :key="key">
          <div>
            <dt>{{ key }}</dt>
            <dd><code>{{ value }}</code></dd>
          </div>
        </template>
      </dl>
    </details>
  </div>
</template>

<style scoped>
.technical-toggle {
  cursor: pointer;
}
</style>
