import type { JobSnapshot } from './types';

export function hasKnownProgress(job: JobSnapshot): boolean {
  return job.totalBytes > 0 || job.totalUnits > 0;
}

export function progressPercentage(job: JobSnapshot): number {
  if (job.totalBytes > 0) {
    return Math.min(100, (job.downloadedBytes / job.totalBytes) * 100);
  }

  if (job.totalUnits > 0) {
    return Math.min(100, (job.completedUnits / job.totalUnits) * 100);
  }

  return 0;
}
