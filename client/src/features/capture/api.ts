import { getAppBridge } from '../../shared/wails/app';
import { parseJobSnapshot } from '../downloads/api';
import type { CapturedDownloadRequest, JobSnapshot } from '../downloads/types';
import type {
  AutomaticCaptureFailure,
  AutomaticCaptureStatus,
  CaptureDiagnostic,
  CaptureSessionSnapshot,
  CaptureStreamSummary,
  Pairing,
  TechnicalDiagnostic,
} from './types';

interface CaptureBridge {
  StartManualCapture(): Promise<unknown>;
  GetCaptureSession(): Promise<unknown>;
  GetAutomaticCaptureStatus(): Promise<unknown>;
  RepairAutomaticCapture(): Promise<unknown>;
  DeveloperDiagnosticsEnabled(): Promise<unknown>;
  SetDeveloperDiagnostics(enabled: boolean): Promise<unknown>;
  GetTechnicalContext(streamID: string): Promise<unknown>;
  ListCapturedStreams(): Promise<unknown>;
  QueueCapturedStream(request: CapturedDownloadRequest): Promise<unknown>;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null ? value as Record<string, unknown> : null;
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function booleanValue(value: unknown): boolean {
  return value === true;
}

function numberValue(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function parseCaptureDiagnostic(value: unknown): CaptureDiagnostic | undefined {
  const raw = asRecord(value);
  if (!raw || typeof raw.code !== 'string' || typeof raw.stage !== 'string' || typeof raw.userMessage !== 'string' || typeof raw.diagnosticId !== 'string') {
    return undefined;
  }

  return {
    code: raw.code,
    stage: raw.stage,
    retryable: booleanValue(raw.retryable),
    userMessage: raw.userMessage,
    diagnosticId: raw.diagnosticId,
    occurredAt: stringValue(raw.occurredAt),
    safeContext: stringMapValue(raw.safeContext),
  };
}

export function parseCaptureSession(value: unknown): CaptureSessionSnapshot | null {
  const raw = asRecord(value);
  if (!raw || typeof raw.active !== 'boolean' || typeof raw.paired !== 'boolean') {
    return null;
  }

  return {
    active: raw.active,
    paired: raw.paired,
    expiresAt: stringValue(raw.expiresAt),
    mode: raw.mode === 'automatic' ? 'automatic' : 'manual',
    nativeStatus: raw.nativeStatus === 'connected' || raw.nativeStatus === 'connecting' ? raw.nativeStatus : 'disconnected',
    browser: typeof raw.browser === 'string' ? raw.browser : undefined,
    tabId: numberValue(raw.tabId),
    diagnostic: parseCaptureDiagnostic(raw.diagnostic),
  };
}

function parseAutomaticCaptureFailure(value: unknown): AutomaticCaptureFailure | undefined {
  const raw = asRecord(value);
  if (!raw || typeof raw.code !== 'string' || typeof raw.userMessage !== 'string') {
    return undefined;
  }

  return {
    code: raw.code,
    userMessage: raw.userMessage,
    retryable: booleanValue(raw.retryable),
  };
}

export function parseAutomaticCaptureStatus(value: unknown): AutomaticCaptureStatus | null {
  const raw = asRecord(value);
  if (!raw || typeof raw.healthy !== 'boolean') {
    return null;
  }

  return {
    hostExecutableFound: booleanValue(raw.hostExecutableFound),
    manifestExists: booleanValue(raw.manifestExists),
    manifestPathMatchesInstall: booleanValue(raw.manifestPathMatchesInstall),
    manifestExtensionIdMatches: booleanValue(raw.manifestExtensionIdMatches),
    mozillaRegistryPointsToExpectedManifest: booleanValue(raw.mozillaRegistryPointsToExpectedManifest),
    healthy: raw.healthy,
    failure: parseAutomaticCaptureFailure(raw.failure),
  };
}

function stringMapValue(value: unknown): Record<string, string> | undefined {
  const raw = asRecord(value);
  if (!raw) return undefined;
  const result: Record<string, string> = {};
  for (const [key, entry] of Object.entries(raw)) {
    if (typeof entry === 'string') result[key] = entry;
  }
  return Object.keys(result).length ? result : undefined;
}

export function parseTechnicalDiagnostic(value: unknown): TechnicalDiagnostic | null {
  const raw = asRecord(value);
  if (!raw || typeof raw.diagnosticId !== 'string' || typeof raw.occurredAt !== 'string') {
    return null;
  }

  return {
    diagnosticId: raw.diagnosticId,
    occurredAt: raw.occurredAt,
    rawError: typeof raw.rawError === 'string' ? raw.rawError : undefined,
    requestId: typeof raw.requestId === 'string' ? raw.requestId : undefined,
    requestMethod: typeof raw.requestMethod === 'string' ? raw.requestMethod : undefined,
    requestType: typeof raw.requestType === 'string' ? raw.requestType : undefined,
    requestTimestamp: numberValue(raw.requestTimestamp),
    requestFrameId: numberValue(raw.requestFrameId),
    requestParentFrameId: numberValue(raw.requestParentFrameId),
    requestUrl: typeof raw.requestUrl === 'string' ? raw.requestUrl : undefined,
    requestDocumentUrl: typeof raw.requestDocumentUrl === 'string' ? raw.requestDocumentUrl : undefined,
    requestOriginUrl: typeof raw.requestOriginUrl === 'string' ? raw.requestOriginUrl : undefined,
    requestInitiator: typeof raw.requestInitiator === 'string' ? raw.requestInitiator : undefined,
    requestHeaders: stringMapValue(raw.requestHeaders),
    responseHeaders: stringMapValue(raw.responseHeaders),
    responseStatusLine: typeof raw.responseStatusLine === 'string' ? raw.responseStatusLine : undefined,
    responseFromCache: typeof raw.responseFromCache === 'boolean' ? raw.responseFromCache : undefined,
    responseIp: typeof raw.responseIp === 'string' ? raw.responseIp : undefined,
    nativeHostDetail: typeof raw.nativeHostDetail === 'string' ? raw.nativeHostDetail : undefined,
    pipeDetail: typeof raw.pipeDetail === 'string' ? raw.pipeDetail : undefined,
    httpStatus: typeof raw.httpStatus === 'number' ? raw.httpStatus : undefined,
    truncated: booleanValue(raw.truncated),
  };
}

export function parsePairing(value: unknown): Pairing | null {
  const raw = asRecord(value);
  if (!raw || typeof raw.code !== 'string' || raw.code === '' || typeof raw.expiresAt !== 'string') {
    return null;
  }

  return { code: raw.code, expiresAt: raw.expiresAt };
}

export function parseCaptureStreamSummary(value: unknown): CaptureStreamSummary | null {
  const raw = asRecord(value);
  if (!raw || typeof raw.id !== 'string' || raw.id === '' || typeof raw.name !== 'string' || typeof raw.host !== 'string' || typeof raw.type !== 'string') {
    return null;
  }

  return {
    id: raw.id,
    name: raw.name,
    host: raw.host,
    type: raw.type,
    capturedAt: stringValue(raw.capturedAt),
  };
}

function bridge(): CaptureBridge {
  return getAppBridge<CaptureBridge>();
}

export async function startBrowserCapture(): Promise<Pairing> {
  const pairing = parsePairing(await bridge().StartManualCapture());
  if (!pairing) {
    throw new Error('OpenDownload returned an invalid capture session.');
  }

  return pairing;
}

export async function getCaptureSession(): Promise<CaptureSessionSnapshot> {
  const session = parseCaptureSession(await bridge().GetCaptureSession());
  if (!session) {
    throw new Error('OpenDownload returned an invalid capture session.');
  }

  return session;
}

export async function getAutomaticCaptureStatus(): Promise<AutomaticCaptureStatus> {
  const status = parseAutomaticCaptureStatus(await bridge().GetAutomaticCaptureStatus());
  if (!status) {
    throw new Error('OpenDownload returned an invalid automatic capture status.');
  }

  return status;
}

export async function repairAutomaticCapture(): Promise<void> {
  await bridge().RepairAutomaticCapture();
}

export async function developerDiagnosticsEnabled(): Promise<boolean> {
  const enabled = await bridge().DeveloperDiagnosticsEnabled();
  if (typeof enabled !== 'boolean') {
    throw new Error('OpenDownload returned an invalid diagnostics state.');
  }

  return enabled;
}

export async function setDeveloperDiagnostics(enabled: boolean): Promise<void> {
  await bridge().SetDeveloperDiagnostics(enabled);
}

export async function getTechnicalContext(streamID: string): Promise<TechnicalDiagnostic | null> {
  return parseTechnicalDiagnostic(await bridge().GetTechnicalContext(streamID));
}

export async function listCapturedStreams(): Promise<CaptureStreamSummary[]> {
  const response = await bridge().ListCapturedStreams();
  if (!Array.isArray(response)) {
    throw new Error('OpenDownload returned an invalid capture stream list.');
  }

  const streams = response.map(parseCaptureStreamSummary);
  if (streams.some((stream) => stream === null)) {
    throw new Error('OpenDownload returned an invalid captured stream.');
  }

  return streams as CaptureStreamSummary[];
}

export async function queueCapturedStream(request: CapturedDownloadRequest): Promise<JobSnapshot> {
  const snapshot = parseJobSnapshot(await bridge().QueueCapturedStream(request));
  if (!snapshot) {
    throw new Error('OpenDownload returned an invalid download update.');
  }

  return snapshot;
}
