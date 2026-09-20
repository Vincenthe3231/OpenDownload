import type { JobSnapshot } from './types';

export function formatBytes(value: number): string {
  if (value < 1024) {
    return `${Math.round(value)} B`;
  }

  const units = ['KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)) - 1, units.length - 1);
  const divisor = 1024 ** (index + 1);
  const precision = value >= 1024 ** (index + 2) ? 1 : 0;
  return `${(value / divisor).toFixed(precision)} ${units[index]}`;
}

export function formatDuration(value: number): string {
  const seconds = Math.max(0, Math.ceil(value));
  if (seconds < 60) {
    return `${seconds}s`;
  }

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m ${seconds % 60}s`;
  }

  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
}

export function formatDownloadStatus(status: JobSnapshot['status']): string {
  return `${status[0].toUpperCase()}${status.slice(1)}`;
}

export function formatProgressDetail(job: JobSnapshot): string {
  if (job.status === 'queued') {
    return 'Waiting for an available transfer connection';
  }

  if (job.status === 'completed') {
    return job.totalUnits > 0 ? `${job.totalUnits} segments saved` : `${formatBytes(job.downloadedBytes)} saved`;
  }

  if (job.status === 'cancelled') {
    return 'Download cancelled';
  }

  if (job.status === 'failed') {
    if (job.diagnostic) {
      return job.downloadedBytes > 0
        ? `Download failed after ${formatBytes(job.downloadedBytes)}. See diagnostic details below.`
        : 'Download failed. See diagnostic details below.';
    }
    return job.message || (job.downloadedBytes > 0 ? `Stopped after ${formatBytes(job.downloadedBytes)}` : 'Download failed');
  }

  const detail: string[] = [];
  if (job.totalBytes > 0) {
    detail.push(`${formatBytes(job.downloadedBytes)} of ${formatBytes(job.totalBytes)}`);
  } else if (job.totalUnits > 0) {
    detail.push(`${job.completedUnits} of ${job.totalUnits} segments`);
  } else {
    detail.push(`${formatBytes(job.downloadedBytes)} received`);
  }

  if (job.bytesPerSecond > 0) {
    detail.push(`${formatBytes(job.bytesPerSecond)}/s`);
  }
  if (job.activeConnections > 0) {
    detail.push(`${job.activeConnections} active connections`);
  }
  if (job.hasEta) {
    detail.push(`${formatDuration(job.etaSeconds)} remaining`);
  }

  return detail.join(' · ');
}
