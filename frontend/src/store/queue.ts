import { defineStore } from 'pinia';

export interface DownloadItem {
  id: string;
  name: string;
  progress: number;
  status: 'pending' | 'downloading' | 'completed' | 'failed';
}

export interface DetectedStream {
  url: string;
  type: string;
}

export const useQueueStore = defineStore('queue', {
  state: () => ({
    activeDownloads: [] as DownloadItem[],
    detectedStreams: [] as DetectedStream[],
  }),
  actions: {
    addDownload(download: DownloadItem) { this.activeDownloads.push(download); },
    addDetected(stream: DetectedStream) { this.detectedStreams.push(stream); },
  },
});
