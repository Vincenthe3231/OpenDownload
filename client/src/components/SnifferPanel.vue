<script setup lang="ts">
import { useQueueStore } from '../store/queue';
const queue = useQueueStore();
</script>

<template>
  <div class="panel-content">
    <div class="panel-heading"><div><p class="eyebrow">CAPTURE</p><h2>Detected streams</h2></div><span class="count-badge">{{ queue.detectedStreams.length }}</span>
    </div>
    <ul class="stream-list" v-if="queue.detectedStreams.length"><li v-for="s in queue.detectedStreams" :key="s.url">
        {{ s.url }}
      </li>
    </ul>
    <div v-else class="empty-state capture-empty"><p>Capture is available from the CLI</p><span>Run <code>opendownload sniff --port 9000</code>, then paste a detected URL above.</span>
    </div>
  </div>
</template>
