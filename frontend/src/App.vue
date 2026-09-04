<script setup lang="ts">
import { useTheme } from './composables/useTheme';
import { SunIcon, MoonIcon } from '@heroicons/vue/24/outline';
import DownloadQueue from './components/DownloadQueue.vue';
import SnifferPanel from './components/SnifferPanel.vue';

const { isDark, toggleDark } = useTheme();
</script>

<template>
  <div class="min-h-screen p-6 max-w-6xl mx-auto">
    <header class="glass rounded-2xl p-4 flex justify-between items-center mb-6">
      <h1 class="text-2xl font-bold">OpenDownload</h1>
      <button @click="toggleDark" class="p-2 rounded-full glass">
        <SunIcon v-if="isDark" class="w-5 h-5" />
        <MoonIcon v-else class="w-5 h-5" />
      </button>
    </header>
    
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <main class="md:col-span-2 glass rounded-2xl p-6">
        <DownloadQueue />
      </main>
      <aside>
        <SnifferPanel />
      </aside>
    </div>
  </div>
</template>
