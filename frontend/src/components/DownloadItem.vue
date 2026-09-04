<script setup lang="ts">
import type { DownloadItem } from '../store/queue';
defineProps<{ item: DownloadItem }>();
</script>

<template>
  <div class="glass p-4 rounded-xl flex items-center justify-between">
    <div>
      <p class="font-medium text-sm truncate max-w-sm">{{ item.name }}</p>
      <div class="h-2 w-48 bg-gray-200 dark:bg-gray-700 rounded-full mt-2">
        <div class="h-2 bg-accent rounded-full" :style="{ width: item.progress + '%' }"></div>
      </div>
    </div>
    <span class="text-xs font-mono">{{ item.status }}</span>
  </div>
</template>
