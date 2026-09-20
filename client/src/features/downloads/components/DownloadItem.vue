<script setup lang="ts">
import { computed } from 'vue';
import { formatDownloadStatus, formatProgressDetail } from '../formatters';
import { hasKnownProgress, progressPercentage } from '../progress';
import type { JobSnapshot } from '../types';
import DownloadDiagnosticDetails from './DownloadDiagnosticDetails.vue';

const props = defineProps<{ item: JobSnapshot }>();
const emit = defineEmits<{ cancel: [id: string] }>();

const hasProgress = computed(() => hasKnownProgress(props.item));
const progressPercent = computed(() => progressPercentage(props.item));
const statusText = computed(() => formatDownloadStatus(props.item.status));
const progressDetail = computed(() => formatProgressDetail(props.item));

function requestCancel(): void {
  emit('cancel', props.item.id);
}
</script>

<template>
  <div class="download-item">
    <div class="download-summary">
      <p>{{ item.name }}</p>
      <div class="progress-track" role="progressbar" :aria-label="`${statusText}. ${progressDetail}`" :aria-valuemin="hasProgress ? 0 : undefined" :aria-valuemax="hasProgress ? 100 : undefined" :aria-valuenow="hasProgress ? Math.round(progressPercent) : undefined">
        <div class="progress-value" :class="[item.status, { indeterminate: item.status === 'downloading' && !hasProgress }]" :style="{ width: hasProgress ? `${progressPercent}%` : undefined }"></div>
      </div>
      <p class="progress-detail">{{ progressDetail }}</p>
      <p v-if="item.status === 'completed' && item.outputPath" class="progress-detail output-path" :title="item.outputPath">Saved to {{ item.outputPath }}</p>
      <DownloadDiagnosticDetails v-if="item.status === 'failed' && item.diagnostic" :diagnostic="item.diagnostic" :filename="item.name" />
    </div>
    <div class="download-actions"><button v-if="item.status === 'queued' || item.status === 'downloading'" class="icon-button compact" type="button" @click="requestCancel">Cancel</button><span class="status-pill" :class="item.status" aria-live="polite">{{ statusText }}</span></div>
  </div>
</template>
