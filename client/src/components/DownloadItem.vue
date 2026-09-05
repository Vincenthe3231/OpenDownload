<script setup lang="ts">
import type { DownloadItem } from '../store/queue';
defineProps<{ item: DownloadItem }>();
</script>

<template>
  <div class="download-item">
    <div class="download-summary"><p>{{ item.name }}</p><div class="progress-track" aria-hidden="true"><div class="progress-value" :class="item.status" :style="{ width: item.progress + '%' }"></div></div>
      </div>
    <span class="status-pill" :class="item.status">{{ item.status }}</span>
  </div>
</template>
