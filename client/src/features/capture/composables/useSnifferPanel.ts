import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import {
  developerDiagnosticsEnabled,
  getAutomaticCaptureStatus,
  getCaptureSession,
  getTechnicalContext,
  queueCapturedStream,
  repairAutomaticCapture as repairAutomaticCaptureApi,
  setDeveloperDiagnostics,
  startBrowserCapture,
} from '../api';
import { subscribeToCaptureEvents, type CaptureEventSubscription } from '../events';
import { useCaptureStore } from '../store';
import { useDownloadStore } from '../../downloads/store';
import type { TechnicalDiagnostic } from '../../../shared/diagnostics/technical';
import type {
  AutomaticCaptureStatus,
  CaptureStreamSummary,
} from '../types';
import { formatDateTime } from '../../../shared/formatters/dateTime';

export function useSnifferPanel() {
  const capture = useCaptureStore();
  const downloads = useDownloadStore();
  const pairingCode = ref('');
  const pairingExpiresAt = ref('');
  const currentTime = ref(Date.now());
  const error = ref('');
  const statusError = ref('');
  const loading = ref(false);
  const automaticStatusLoading = ref(true);
  const repairLoading = ref(false);
  const diagnosticsLoading = ref(false);
  const diagnosticsEnabled = ref(false);
  const automaticStatus = ref<AutomaticCaptureStatus | null>(null);
  const technicalDialogStreamID = ref('');
  const technicalContext = ref<Record<string, TechnicalDiagnostic | null>>({});
  const technicalLoading = ref<Record<string, boolean>>({});
  let eventSubscription: CaptureEventSubscription | undefined;
  let expiryTimer: ReturnType<typeof setInterval> | undefined;

  const pairingExpired = computed(() => {
    const expiresAt = Date.parse(pairingExpiresAt.value);
    return !Number.isNaN(expiresAt) && currentTime.value >= expiresAt;
  });
  const expiresText = computed(() => {
    if (!pairingExpiresAt.value) return '';
    return pairingExpired.value
      ? 'This pairing code has expired. Generate a fresh code below without restarting OpenDownload.'
      : `Pairing code expires at ${formatDateTime(pairingExpiresAt.value)}.`;
  });
  const appPipeUnavailable = computed(() => capture.session.diagnostic?.code === 'IPC_ACCESS_DENIED');
  const automaticNeedsRepair = computed(() => Boolean(automaticStatus.value && !automaticStatus.value.healthy && !appPipeUnavailable.value));
  const automaticStatusLabel = computed(() => {
    if (automaticStatusLoading.value) return 'Checking';
    if (appPipeUnavailable.value) return 'Offline';
    if (!automaticStatus.value) return 'Unavailable';
    if (!automaticStatus.value.healthy) return 'Needs repair';
    if (capture.session.mode === 'automatic') return 'Connected';
    return 'Ready';
  });
  const automaticStatusMessage = computed(() => {
    if (automaticStatusLoading.value) return 'Checking the Firefox native host and browser registration.';
    if (appPipeUnavailable.value) return 'Restart OpenDownload.';
    if (!automaticStatus.value) return 'Refresh status to inspect the Firefox browser capture host.';
    if (!automaticStatus.value.healthy) {
      return automaticStatus.value.failure?.code === 'NATIVE_HOST_NOT_REGISTERED'
        ? 'Repair browser capture.'
        : 'Restart OpenDownload.';
    }
    if (capture.session.mode === 'automatic') {
      const browser = capture.session.browser ? ` in ${capture.session.browser}` : '';
      return `Automatic capture is connected${browser}.`;
    }
    return 'Install or reopen the Gecko extension. Open OpenDownload Capture in Firefox or Zen, then start capture.';
  });
  const automaticStatusTone = computed(() => {
    if (appPipeUnavailable.value || (automaticStatus.value && !automaticStatus.value.healthy)) return 'danger';
    if (capture.session.mode === 'automatic') return 'success';
    return 'neutral';
  });
  const statusChecks = computed(() => {
    const status = automaticStatus.value;
    if (!status) return [];
    return [
      { label: 'Host', healthy: status.hostExecutableFound },
      { label: 'Manifest', healthy: status.manifestExists && status.manifestPathMatchesInstall },
      { label: 'Firefox ID', healthy: status.manifestExtensionIdMatches },
      { label: 'Registry', healthy: status.mozillaRegistryPointsToExpectedManifest },
    ];
  });
  const technicalDialogStream = computed(() => capture.streams.find((stream) => stream.id === technicalDialogStreamID.value));

  watch(
    () => capture.session.active,
    (active) => {
      if (!active) {
        pairingCode.value = '';
        pairingExpiresAt.value = '';
        technicalDialogStreamID.value = '';
        technicalContext.value = {};
      }
    },
  );

  async function refreshAutomaticStatus(): Promise<void> {
    statusError.value = '';
    automaticStatusLoading.value = true;
    try {
      automaticStatus.value = await getAutomaticCaptureStatus();
    } catch (reason) {
      statusError.value = reason instanceof Error ? reason.message : 'Could not inspect automatic capture.';
      automaticStatus.value = null;
    } finally {
      automaticStatusLoading.value = false;
    }
  }

  async function hydratePanel(): Promise<void> {
    statusError.value = '';
    automaticStatusLoading.value = true;
    try {
      const [session, status, diagnostics] = await Promise.all([
        getCaptureSession(),
        getAutomaticCaptureStatus(),
        developerDiagnosticsEnabled(),
      ]);
      capture.setSession(session);
      automaticStatus.value = status;
      diagnosticsEnabled.value = diagnostics;
    } catch (reason) {
      statusError.value = reason instanceof Error ? reason.message : 'Could not load capture status.';
    } finally {
      automaticStatusLoading.value = false;
    }

    try {
      await eventSubscription?.hydrate();
    } catch {
      error.value = 'Could not restore captured streams.';
    }
  }

  async function repairAutomaticCapture(): Promise<void> {
    statusError.value = '';
    repairLoading.value = true;
    try {
      await repairAutomaticCaptureApi();
      await refreshAutomaticStatus();
    } catch (reason) {
      statusError.value = reason instanceof Error ? reason.message : 'Could not repair browser capture.';
    } finally {
      repairLoading.value = false;
    }
  }

  async function start(): Promise<void> {
    error.value = '';
    loading.value = true;
    try {
      const pairing = await startBrowserCapture();
      pairingCode.value = pairing.code;
      pairingExpiresAt.value = pairing.expiresAt;
      capture.setSession({
        ...capture.session,
        active: true,
        paired: false,
        expiresAt: pairing.expiresAt,
        mode: 'manual',
        nativeStatus: 'disconnected',
        diagnostic: undefined,
      });
      await eventSubscription?.hydrate();
    } catch (reason) {
      error.value = reason instanceof Error ? reason.message : 'Could not start browser capture.';
    } finally {
      loading.value = false;
    }
  }

  async function copyPairingCode(): Promise<void> {
    try {
      await navigator.clipboard.writeText(pairingCode.value);
    } catch {
      error.value = 'Copy the pairing code directly from the field.';
    }
  }

  async function download(stream: CaptureStreamSummary): Promise<void> {
    error.value = '';
    const id = crypto.randomUUID();
    try {
      downloads.apply(await queueCapturedStream({ id, capturedStreamId: stream.id, outputDir: '' }));
    } catch (reason) {
      error.value = reason instanceof Error ? reason.message : 'Could not download the captured stream.';
    }
  }

  async function toggleDeveloperDiagnostics(enabled: boolean): Promise<void> {
    error.value = '';
    diagnosticsLoading.value = true;
    try {
      await setDeveloperDiagnostics(enabled);
      diagnosticsEnabled.value = enabled;
      if (!enabled) {
        technicalDialogStreamID.value = '';
        technicalContext.value = {};
      }
    } catch (reason) {
      error.value = reason instanceof Error ? reason.message : 'Could not change developer diagnostics.';
    } finally {
      diagnosticsLoading.value = false;
    }
  }

  async function openTechnicalContext(streamID: string): Promise<void> {
    if (!diagnosticsEnabled.value) return;
    technicalDialogStreamID.value = streamID;
    if (Object.prototype.hasOwnProperty.call(technicalContext.value, streamID)) return;
    technicalLoading.value[streamID] = true;
    try {
      technicalContext.value[streamID] = await getTechnicalContext(streamID);
    } catch {
      technicalContext.value[streamID] = null;
    } finally {
      technicalLoading.value[streamID] = false;
    }
  }

  function closeTechnicalContext(): void {
    technicalDialogStreamID.value = '';
  }

  onMounted(() => {
    expiryTimer = setInterval(() => {
      currentTime.value = Date.now();
    }, 1000);
    eventSubscription = subscribeToCaptureEvents(capture);
    void hydratePanel();
  });

  onBeforeUnmount(() => {
    if (expiryTimer !== undefined) clearInterval(expiryTimer);
    eventSubscription?.dispose();
  });

  return {
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
  };
}
