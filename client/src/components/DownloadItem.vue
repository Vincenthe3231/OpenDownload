<script setup lang="ts">
import { computed } from 'vue';
import type { DownloadItem } from '../store/queue';

const props = defineProps<{ item: DownloadItem }>();
defineEmits<{ cancel: [id: string] }>();

const hasKnownProgress = computed(() => props.item.totalBytes > 0 || props.item.totalUnits > 0);
const progressPercent = computed(() => {
  if (props.item.totalBytes > 0) return Math.min(100, props.item.downloadedBytes / props.item.totalBytes * 100);
  if (props.item.totalUnits > 0) return Math.min(100, props.item.completedUnits / props.item.totalUnits * 100);
  return 0;
});
const statusText = computed(() => props.item.status[0].toUpperCase() + props.item.status.slice(1));
const progressDetail = computed(() => {
  const item = props.item;
  if (item.status === 'queued') return 'Waiting for an available transfer connection';
  if (item.status === 'completed') return item.totalUnits > 0 ? `${item.totalUnits} segments saved` : `${formatBytes(item.downloadedBytes)} saved`;
  if (item.status === 'cancelled') return 'Download cancelled';
  if (item.status === 'failed') return item.message || (item.downloadedBytes > 0 ? `Stopped after ${formatBytes(item.downloadedBytes)}` : 'Download failed');

  const detail: string[] = [];
  if (item.totalBytes > 0) detail.push(`${formatBytes(item.downloadedBytes)} of ${formatBytes(item.totalBytes)}`);
  else if (item.totalUnits > 0) detail.push(`${item.completedUnits} of ${item.totalUnits} segments`);
  else detail.push(`${formatBytes(item.downloadedBytes)} received`);
  if (item.bytesPerSecond > 0) detail.push(`${formatBytes(item.bytesPerSecond)}/s`);
  if (item.activeConnections > 0) detail.push(`${item.activeConnections} active connections`);
  if (item.hasEta) detail.push(`${formatDuration(item.etaSeconds)} remaining`);
  return detail.join(' · ');
});

function formatBytes(value: number) {
  if (value < 1024) return `${Math.round(value)} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)) - 1, units.length - 1);
  return `${(value / 1024 ** (index + 1)).toFixed(value >= 1024 ** (index + 2) ? 1 : 0)} ${units[index]}`;
}

function formatDuration(value: number) {
  const seconds = Math.max(0, Math.ceil(value));
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ${seconds % 60}s`;
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
}
</script>

<template>
  <div class="download-item">
    <div class="download-summary">
      <p>{{ item.name }}</p>
      <div class="progress-track" role="progressbar" :aria-label="`${statusText}. ${progressDetail}`" :aria-valuemin="hasKnownProgress ? 0 : undefined" :aria-valuemax="hasKnownProgress ? 100 : undefined" :aria-valuenow="hasKnownProgress ? Math.round(progressPercent) : undefined">
        <div class="progress-value" :class="[item.status, { indeterminate: item.status === 'downloading' && !hasKnownProgress }]" :style="{ width: hasKnownProgress ? `${progressPercent}%` : undefined }"></div>
      </div>
      <p class="progress-detail">{{ progressDetail }}</p>
      <p v-if="item.outputPath" class="progress-detail output-path" :title="item.outputPath">Saved to {{ item.outputPath }}</p>
    </div>
    <div class="download-actions"><button v-if="item.status === 'queued' || item.status === 'downloading'" class="icon-button compact" type="button" @click="$emit('cancel', item.id)">Cancel</button><span class="status-pill" :class="item.status" aria-live="polite">{{ statusText }}</span></div>
  </div>
</template>
