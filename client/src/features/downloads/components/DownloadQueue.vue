<script setup lang="ts">
import { ref } from 'vue';
import { cancelDownload } from '../api';
import { useDownloadStore } from '../store';
import DownloadItem from './DownloadItem.vue';

const downloads = useDownloadStore();
const error = ref('');

async function cancel(jobID: string): Promise<void> {
  error.value = '';
  try {
    downloads.apply(await cancelDownload(jobID));
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Could not cancel the download.';
  }
}
</script>

<template>
  <div class="panel-content">
    <div class="panel-heading"><div><p class="eyebrow">ACTIVITY</p><h2>Downloads</h2></div><span class="count-badge" :aria-label="`${downloads.jobs.length} downloads`">{{ downloads.jobs.length }}</span>
    </div>
    <div v-if="downloads.jobs.length === 0" class="empty-state"><p>No downloads yet</p><span>Your recent downloads will appear here.</span>
    </div>
    <DownloadItem v-for="item in downloads.jobs" :key="item.id" :item="item" @cancel="cancel" />
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
  </div>
</template>
