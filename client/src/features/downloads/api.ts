import { getAppBridge } from '../../shared/wails/app';
import { parseTechnicalDiagnostic, type TechnicalDiagnostic } from '../../shared/diagnostics/technical';
import { DOWNLOAD_STATUSES } from './constants';
import type { DownloadDiagnostic, DownloadRequest, DownloadStatus, JobSnapshot } from './types';

interface DownloadBridge {
  QueueDownload(request: DownloadRequest): Promise<unknown>;
  CancelDownload(jobID: string): Promise<unknown>;
  ListDownloadJobs(): Promise<unknown>;
  GetDownloadTechnicalContext(diagnosticID: string): Promise<unknown>;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null ? value as Record<string, unknown> : null;
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function numberValue(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : 0;
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

function parseDownloadDiagnostic(value: unknown): DownloadDiagnostic | undefined {
  const raw = asRecord(value);
  if (!raw || typeof raw.code !== 'string' || typeof raw.stage !== 'string' || typeof raw.userMessage !== 'string' || typeof raw.diagnosticId !== 'string') {
    return undefined;
  }

  return {
    code: raw.code,
    stage: raw.stage,
    retryable: raw.retryable === true,
    userMessage: raw.userMessage,
    diagnosticId: raw.diagnosticId,
    occurredAt: stringValue(raw.occurredAt),
    safeContext: stringMapValue(raw.safeContext),
  };
}

function isDownloadStatus(value: unknown): value is DownloadStatus {
  return typeof value === 'string' && DOWNLOAD_STATUSES.includes(value as DownloadStatus);
}

export function parseJobSnapshot(value: unknown): JobSnapshot | null {
  const raw = asRecord(value);
  if (!raw || typeof raw.id !== 'string' || raw.id === '' || typeof raw.name !== 'string' || !isDownloadStatus(raw.status)) {
    return null;
  }

  return {
    id: raw.id,
    name: raw.name,
    outputPath: stringValue(raw.outputPath),
    status: raw.status,
    downloadedBytes: numberValue(raw.downloadedBytes),
    totalBytes: numberValue(raw.totalBytes),
    completedUnits: numberValue(raw.completedUnits),
    totalUnits: numberValue(raw.totalUnits),
    bytesPerSecond: numberValue(raw.bytesPerSecond),
    etaSeconds: numberValue(raw.etaSeconds),
    hasEta: raw.hasEta === true,
    activeConnections: numberValue(raw.activeConnections),
    message: stringValue(raw.message),
    diagnostic: parseDownloadDiagnostic(raw.diagnostic),
    version: numberValue(raw.version),
  };
}

function requiredSnapshot(value: unknown): JobSnapshot {
  const snapshot = parseJobSnapshot(value);
  if (!snapshot) {
    throw new Error('OpenDownload returned an invalid download update.');
  }

  return snapshot;
}

function bridge(): DownloadBridge {
  return getAppBridge<DownloadBridge>();
}

export async function queueDownload(request: DownloadRequest): Promise<JobSnapshot> {
  const snapshot = await bridge().QueueDownload(request);
  return requiredSnapshot(snapshot);
}

export async function cancelDownload(jobID: string): Promise<JobSnapshot> {
  const snapshot = await bridge().CancelDownload(jobID);
  return requiredSnapshot(snapshot);
}

export async function listDownloadJobs(): Promise<JobSnapshot[]> {
  const response = await bridge().ListDownloadJobs();
  if (!Array.isArray(response)) {
    throw new Error('OpenDownload returned an invalid download list.');
  }

  return response.map(requiredSnapshot);
}

export async function getDownloadTechnicalContext(diagnosticID: string): Promise<TechnicalDiagnostic | null> {
  return parseTechnicalDiagnostic(await bridge().GetDownloadTechnicalContext(diagnosticID));
}
