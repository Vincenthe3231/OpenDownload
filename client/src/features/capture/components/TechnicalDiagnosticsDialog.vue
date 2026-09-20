<script setup lang="ts">
import { computed } from 'vue';
import SharedTechnicalDiagnosticsDialog from '../../../shared/diagnostics/components/TechnicalDiagnosticsDialog.vue';
import type { TechnicalDiagnostic } from '../../../shared/diagnostics/technical';
import type { CaptureStreamSummary } from '../types';

interface Props {
  open: boolean;
  stream?: CaptureStreamSummary;
  detail?: TechnicalDiagnostic | null;
  loading: boolean;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  (event: 'close'): void;
}>();

const title = computed(() => props.stream?.name || 'Captured stream');
const subtitle = computed(() => `Full technical context for ${props.stream?.host || 'this captured request'}.`);
</script>

<template>
  <SharedTechnicalDiagnosticsDialog
    :open="open"
    :title="title"
    :subtitle="subtitle"
    :detail="detail"
    :loading="loading"
    empty-message="No technical context was retained for this stream. Enable developer diagnostics before starting or reconnecting capture, then capture a new request."
    @close="emit('close')"
  />
</template>
