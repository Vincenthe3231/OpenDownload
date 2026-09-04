<script setup lang="ts">
import { useQueueStore } from '../store/queue';
const queue = useQueueStore();
</script>

<template>
  <div class="glass p-4 rounded-xl">
    <h2 class="text-lg font-semibold mb-3">Sniffer</h2>
    <ul class="space-y-2">
      <li v-for="s in queue.detectedStreams" :key="s.url" class="text-xs truncate">
        {{ s.url }}
      </li>
    </ul>
  </div>
</template>
