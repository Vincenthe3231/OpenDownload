import { defineStore } from 'pinia';

export interface DownloadItem {
  id: string;
  name: string;
  progress: number;
  status: 'pending' | 'downloading' | 'completed' | 'failed';
	downloadedBytes: number;
	totalBytes: number;
	completedUnits: number;
	totalUnits: number;
	bytesPerSecond: number;
	etaSeconds: number;
	hasEta: boolean;
}

export interface DownloadProgressEvent {
	id: string;
	downloadedBytes: number;
	totalBytes: number;
	completedUnits: number;
	totalUnits: number;
	bytesPerSecond: number;
	etaSeconds: number;
	hasEta: boolean;
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
	addDownload(download: Pick<DownloadItem, 'id' | 'name'>) {
		this.activeDownloads.push({
			...download,
			progress: 0,
			status: 'downloading',
			downloadedBytes: 0,
			totalBytes: 0,
			completedUnits: 0,
			totalUnits: 0,
			bytesPerSecond: 0,
			etaSeconds: 0,
			hasEta: false,
		});
	},
    addDetected(stream: DetectedStream) { this.detectedStreams.push(stream); },
    setDetected(streams: DetectedStream[]) { this.detectedStreams = streams; },
    updateDownload(id: string, patch: Partial<DownloadItem>) {
      const download = this.activeDownloads.find((item) => item.id === id);
      if (download) Object.assign(download, patch);
    },
	updateProgress(update: DownloadProgressEvent) {
		const download = this.activeDownloads.find((item) => item.id === update.id);
		if (!download) return;
		Object.assign(download, update, {
			progress: update.totalBytes > 0
				? Math.min(100, update.downloadedBytes / update.totalBytes * 100)
				: update.totalUnits > 0
					? Math.min(100, update.completedUnits / update.totalUnits * 100)
					: 0,
		});
	},
  },
});
