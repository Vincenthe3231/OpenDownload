<script setup lang="ts">
import { useQueueStore } from '../store/queue';
import DownloadItem from './DownloadItem.vue';

const queue = useQueueStore();
</script>

<template>
  <div class="panel-content">
    <div class="panel-heading"><div><p class="eyebrow">ACTIVITY</p><h2>Downloads</h2></div><span class="count-badge" :aria-label="`${queue.activeDownloads.length} downloads`">{{ queue.activeDownloads.length }}</span>
    </div>
    <div v-if="queue.activeDownloads.length === 0" class="empty-state"><p>No downloads yet</p><span>Your recent downloads will appear here.</span>
    </div>
    <DownloadItem v-for="item in queue.activeDownloads" :key="item.id" :item="item" />
  </div>
</template>
