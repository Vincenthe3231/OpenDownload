import { getAppBridge } from '../../shared/wails/app';
import { parseJobSnapshot } from '../downloads/api';
import type { CapturedDownloadRequest, JobSnapshot } from '../downloads/types';
import type { CaptureStreamSummary, Pairing } from './types';

interface CaptureBridge {
  StartFirefoxCapture(): Promise<unknown>;
  ListCapturedStreams(): Promise<unknown>;
  QueueCapturedStream(request: CapturedDownloadRequest): Promise<unknown>;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null ? value as Record<string, unknown> : null;
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
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

export async function startFirefoxCapture(): Promise<Pairing> {
  const pairing = parsePairing(await bridge().StartFirefoxCapture());
  if (!pairing) {
    throw new Error('OpenDownload returned an invalid capture session.');
  }

  return pairing;
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
