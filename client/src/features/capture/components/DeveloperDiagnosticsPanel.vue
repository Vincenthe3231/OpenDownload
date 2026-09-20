<script setup lang="ts">
import { ExclamationTriangleIcon } from '@heroicons/vue/24/outline';

interface Props {
  enabled: boolean;
  loading: boolean;
  hasStreams: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  (event: 'toggle', enabled: boolean): void;
}>();

function handleChange(event: Event): void {
  emit('toggle', (event.target as HTMLInputElement).checked);
}
</script>

<template>
  <section class="diagnostics-panel" aria-labelledby="diagnostics-title">
    <div class="diagnostics-heading">
      <div>
        <p class="eyebrow">DESKTOP ONLY</p>
        <h3 id="diagnostics-title">Developer diagnostics</h3>
      </div>
      <span class="diagnostic-state" :class="{ enabled }">{{ enabled ? 'Enabled' : 'Off' }}</span>
    </div>

    <label class="diagnostics-switch">
      <input type="checkbox" :checked="enabled" :disabled="loading" @change="handleChange" />
      <span class="switch-track" aria-hidden="true"><span class="switch-thumb"></span></span>
      <span class="switch-copy">
        <strong>{{ loading ? 'Updating diagnostics…' : enabled ? 'Technical detail is available' : 'Enable technical detail' }}</strong>
        <small>Enable before starting or reconnecting automatic capture. If you enable it during an active session, reconnect the extension before capturing a new request. Sensitive context stays in memory for this session only.</small>
      </span>
    </label>

    <p class="diagnostics-warning" role="note">
      <ExclamationTriangleIcon aria-hidden="true" />
      May reveal cookies, tokens, headers, and URLs. Turning this off clears visible technical content immediately.
    </p>
    <p v-if="enabled && !hasStreams" class="diagnostics-empty">Technical details will appear inside each captured stream after a request is detected.</p>
  </section>
</template>
