import { defineStore } from 'pinia';
import type { JobSnapshot } from './types';

interface DownloadState {
  jobs: JobSnapshot[];
}

export const useDownloadStore = defineStore('downloads', {
  state: (): DownloadState => ({
    jobs: [],
  }),
  actions: {
    apply(snapshot: JobSnapshot): void {
      const index = this.jobs.findIndex((job) => job.id === snapshot.id);
      if (index === -1) {
        this.jobs.push(snapshot);
        return;
      }

      if (snapshot.version >= this.jobs[index].version) {
        this.jobs.splice(index, 1, snapshot);
      }
    },
    hydrate(snapshots: JobSnapshot[]): void {
      for (const snapshot of snapshots) {
        this.apply(snapshot);
      }
    },
  },
});

export type DownloadStore = ReturnType<typeof useDownloadStore>;
