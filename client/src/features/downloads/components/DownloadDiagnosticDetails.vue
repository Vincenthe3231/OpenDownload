<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { developerDiagnosticsEnabled } from '../../capture/api';
import { subscribeToDeveloperDiagnosticsChanges } from '../../../shared/diagnostics/state';
import type { TechnicalDiagnostic } from '../../../shared/diagnostics/technical';
import { getDownloadTechnicalContext } from '../api';
import type { DownloadDiagnostic } from '../types';
import TechnicalDiagnosticsDialog from '../../../shared/diagnostics/components/TechnicalDiagnosticsDialog.vue';

const props = defineProps<{
  diagnostic: DownloadDiagnostic;
  filename: string;
}>();

const diagnosticsEnabled = ref(false);
const safeContextEntries = computed(() => Object.entries(props.diagnostic.safeContext ?? {}));
const technicalDialogOpen = ref(false);
const technicalDetail = ref<TechnicalDiagnostic | null | undefined>();
const technicalLoading = ref(false);
let loadedDiagnosticID = '';
let requestVersion = 0;
let diagnosticsTimer: ReturnType<typeof setInterval> | undefined;
let unsubscribeDiagnostics: (() => void) | undefined;

function clearTechnicalContext(): void {
  requestVersion += 1;
  technicalDialogOpen.value = false;
  technicalDetail.value = null;
  technicalLoading.value = false;
  loadedDiagnosticID = '';
}

function applyDiagnosticsState(enabled: boolean): void {
  diagnosticsEnabled.value = enabled;
  if (!enabled) clearTechnicalContext();
}

async function refreshDiagnosticsState(): Promise<void> {
  try {
    applyDiagnosticsState(await developerDiagnosticsEnabled());
  } catch {
    applyDiagnosticsState(false);
  }
}

async function openTechnicalDetails(): Promise<void> {
  if (!diagnosticsEnabled.value) return;

  technicalDialogOpen.value = true;
  const diagnosticID = props.diagnostic.diagnosticId;
  if (loadedDiagnosticID === diagnosticID) return;

  loadedDiagnosticID = diagnosticID;
  technicalDetail.value = undefined;
  technicalLoading.value = true;
  const currentRequest = ++requestVersion;

  try {
    const detail = await getDownloadTechnicalContext(diagnosticID);
    if (currentRequest === requestVersion && diagnosticsEnabled.value) {
      technicalDetail.value = detail;
    }
  } catch {
    if (currentRequest === requestVersion && diagnosticsEnabled.value) {
      technicalDetail.value = null;
    }
  } finally {
    if (currentRequest === requestVersion) technicalLoading.value = false;
  }
}

watch(() => props.diagnostic.diagnosticId, clearTechnicalContext);

onMounted(() => {
  unsubscribeDiagnostics = subscribeToDeveloperDiagnosticsChanges(applyDiagnosticsState);
  void refreshDiagnosticsState();
  diagnosticsTimer = setInterval(() => {
    void refreshDiagnosticsState();
  }, 1500);
});

onBeforeUnmount(() => {
  if (diagnosticsTimer !== undefined) clearInterval(diagnosticsTimer);
  unsubscribeDiagnostics?.();
});
</script>

<template>
  <div class="technical-details" :aria-label="`Download diagnostic ${diagnostic.diagnosticId}`">
    <p class="diagnostics-warning" role="alert">{{ diagnostic.userMessage }}</p>
    <p class="progress-detail">Diagnostic ID: <code>{{ diagnostic.diagnosticId }}</code></p>

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

    <button
      v-if="diagnosticsEnabled"
      class="technical-toggle"
      type="button"
      aria-haspopup="dialog"
      :aria-expanded="technicalDialogOpen"
      :aria-label="`View technical details for ${filename}`"
      @click="void openTechnicalDetails()"
    >
      Technical details
    </button>

    <TechnicalDiagnosticsDialog
      :open="technicalDialogOpen"
      :title="filename"
      :subtitle="`In-memory failure context for ${filename}.`"
      :detail="technicalDetail"
      :loading="technicalLoading"
      empty-message="No technical context was retained for this download. Enable Developer diagnostics before retrying."
      @close="technicalDialogOpen = false"
    />
  </div>
</template>

<style scoped>
.technical-toggle {
  cursor: pointer;
  min-height: 44px;
  justify-content: center;
  padding: 0 12px;
  touch-action: manipulation;
}
</style>
