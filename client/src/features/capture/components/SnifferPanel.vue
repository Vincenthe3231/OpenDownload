<script setup lang="ts">
import AutomaticCaptureStatusCard from './AutomaticCaptureStatusCard.vue';
import CaptureSessionContent from './CaptureSessionContent.vue';
import DeveloperDiagnosticsPanel from './DeveloperDiagnosticsPanel.vue';
import TechnicalDiagnosticsDialog from './TechnicalDiagnosticsDialog.vue';
import { useSnifferPanel } from '../composables/useSnifferPanel';

const {
  capture,
  pairingCode,
  expiresText,
  pairingExpired,
  loading,
  automaticStatusLoading,
  repairLoading,
  diagnosticsLoading,
  diagnosticsEnabled,
  automaticNeedsRepair,
  automaticStatusLabel,
  automaticStatusMessage,
  automaticStatusTone,
  statusChecks,
  technicalDialogStreamID,
  technicalDialogStream,
  technicalContext,
  technicalLoading,
  error,
  statusError,
  refreshAutomaticStatus,
  repairAutomaticCapture,
  start,
  copyPairingCode,
  download,
  toggleDeveloperDiagnostics,
  openTechnicalContext,
  closeTechnicalContext,
} = useSnifferPanel();
</script>

<template>
  <div class="panel-content">
    <div class="panel-heading">
      <div><p class="eyebrow">CAPTURE</p><h2>Detected streams</h2></div>
      <span class="count-badge">{{ capture.streams.length }}</span>
    </div>

    <AutomaticCaptureStatusCard
      :tone="automaticStatusTone"
      :label="automaticStatusLabel"
      :message="automaticStatusMessage"
      :loading="automaticStatusLoading"
      :needs-repair="automaticNeedsRepair"
      :repair-loading="repairLoading"
      :checks="statusChecks"
      @repair="repairAutomaticCapture"
      @refresh="refreshAutomaticStatus"
    />

    <CaptureSessionContent
      :session="capture.session"
      :pairing-code="pairingCode"
      :expires-text="expiresText"
      :pairing-expired="pairingExpired"
      :loading="loading"
      :streams="capture.streams"
      :diagnostics-enabled="diagnosticsEnabled"
      @start="start"
      @copy-pairing-code="copyPairingCode"
      @download="download"
      @technical-details="openTechnicalContext"
    />

    <TechnicalDiagnosticsDialog
      :open="Boolean(technicalDialogStreamID)"
      :stream="technicalDialogStream"
      :detail="technicalDialogStreamID ? technicalContext[technicalDialogStreamID] : undefined"
      :loading="technicalDialogStreamID ? Boolean(technicalLoading[technicalDialogStreamID]) : false"
      @close="closeTechnicalContext"
    />

    <DeveloperDiagnosticsPanel
      :enabled="diagnosticsEnabled"
      :loading="diagnosticsLoading"
      :has-streams="capture.streams.length > 0"
      @toggle="toggleDeveloperDiagnostics"
    />

    <p v-if="capture.session.diagnostic" class="form-error" role="alert">
      {{ capture.session.diagnostic.userMessage }}
      <span v-if="capture.session.diagnostic.code">({{ capture.session.diagnostic.code }})</span>
    </p>
    <p v-else-if="statusError || error" class="form-error" role="alert">{{ statusError || error }}</p>
  </div>
</template>
