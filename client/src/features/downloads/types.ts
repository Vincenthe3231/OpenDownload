import { DOWNLOAD_STATUSES } from './constants';

export type DownloadStatus = (typeof DOWNLOAD_STATUSES)[number];

export interface DownloadRequest {
  id: string;
  url: string;
  outputDir: string;
}

export interface CapturedDownloadRequest {
  id: string;
  capturedStreamId: string;
  outputDir: string;
}

export interface DownloadDiagnostic {
  code: string;
  stage: string;
  retryable: boolean;
  userMessage: string;
  diagnosticId: string;
  occurredAt: string;
  safeContext?: Record<string, string>;
}

export interface JobSnapshot {
  id: string;
  name: string;
  outputPath: string;
  status: DownloadStatus;
  downloadedBytes: number;
  totalBytes: number;
  completedUnits: number;
  totalUnits: number;
  bytesPerSecond: number;
  etaSeconds: number;
  hasEta: boolean;
  activeConnections: number;
  message: string;
  diagnostic?: DownloadDiagnostic;
  version: number;
}
