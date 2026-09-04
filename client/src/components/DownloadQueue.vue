<script setup lang="ts">
import { useQueueStore } from '../store/queue';
import DownloadItem from './DownloadItem.vue';

const queue = useQueueStore();
</script>

<template>
  <div class="space-y-3">
    <h2 class="text-lg font-semibold">Active Downloads</h2>
    <DownloadItem v-for="item in queue.activeDownloads" :key="item.id" :item="item" />
  </div>
</template>
