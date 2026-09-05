import { defineStore } from 'pinia';

export interface DownloadItem {
  id: string;
  name: string;
  progress: number;
  status: 'pending' | 'downloading' | 'completed' | 'failed';
}

export interface DetectedStream {
	id: string;
	url: string;
	host: string;
	name: string;
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
    setDetected(streams: DetectedStream[]) { this.detectedStreams = streams; },
    updateDownload(id: string, patch: Partial<DownloadItem>) {
      const download = this.activeDownloads.find((item) => item.id === id);
      if (download) Object.assign(download, patch);
    },
  },
});
