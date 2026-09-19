<script setup lang="ts">
import {
  ArrowPathIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  WrenchScrewdriverIcon,
} from '@heroicons/vue/24/outline';

type StatusTone = 'danger' | 'success' | 'neutral';

interface StatusCheck {
  label: string;
  healthy: boolean;
}

interface Props {
  tone: StatusTone;
  label: string;
  message: string;
  loading: boolean;
  needsRepair: boolean;
  repairLoading: boolean;
  checks: StatusCheck[];
}

defineProps<Props>();

const emit = defineEmits<{
  (event: 'repair'): void;
  (event: 'refresh'): void;
}>();
</script>

<template>
  <section class="automatic-status" :class="`automatic-status-${tone}`" aria-labelledby="automatic-capture-title">
    <div class="automatic-status-heading">
      <div class="status-orb" aria-hidden="true">
        <CheckCircleIcon v-if="tone === 'success'" />
        <ExclamationTriangleIcon v-else-if="tone === 'danger'" />
        <ArrowPathIcon v-else :class="{ 'is-spinning': loading }" />
      </div>
      <div class="automatic-status-copy">
        <div class="status-title-row">
          <h3 id="automatic-capture-title">Automatic capture</h3>
          <span class="status-pill">{{ label }}</span>
        </div>
        <p>{{ message }}</p>
      </div>
    </div>

    <div v-if="checks.length" class="health-checks" aria-label="Automatic capture checks">
      <span v-for="check in checks" :key="check.label" class="health-check" :class="{ healthy: check.healthy }">
        <span class="health-dot" aria-hidden="true"></span>{{ check.label }}
      </span>
    </div>

    <div class="status-actions">
      <button
        v-if="needsRepair"
        class="repair-button"
        type="button"
        :disabled="repairLoading"
        @click="emit('repair')"
      >
        <WrenchScrewdriverIcon aria-hidden="true" />
        {{ repairLoading ? 'Repairing' : 'Repair browser capture' }}
      </button>
      <button
        class="status-refresh"
        type="button"
        :disabled="loading || repairLoading"
        @click="emit('refresh')"
      >
        <ArrowPathIcon aria-hidden="true" />
        Refresh status
      </button>
    </div>
  </section>
</template>
