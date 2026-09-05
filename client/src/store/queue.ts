import { defineStore } from 'pinia';

export interface DownloadItem {
  id: string;
  name: string;
  progress: number;
  status: 'queued' | 'downloading' | 'completed' | 'failed' | 'cancelled';
	downloadedBytes: number;
	totalBytes: number;
	completedUnits: number;
	totalUnits: number;
	bytesPerSecond: number;
	etaSeconds: number;
	hasEta: boolean;
	activeConnections: number;
	message: string;
	outputPath: string;
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
	activeConnections: number;
}

export interface DownloadStateEvent { id: string; state: DownloadItem['status']; message: string; }
export interface DownloadDestinationEvent { id: string; path: string; }

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
			status: 'queued',
			downloadedBytes: 0,
			totalBytes: 0,
			completedUnits: 0,
			totalUnits: 0,
			bytesPerSecond: 0,
			etaSeconds: 0,
			hasEta: false,
			activeConnections: 0,
			message: '',
			outputPath: '',
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
	updateState(update: DownloadStateEvent) {
		this.updateDownload(update.id, { status: update.state, message: update.message });
	},
	updateDestination(update: DownloadDestinationEvent) {
		this.updateDownload(update.id, { outputPath: update.path });
	},
  },
});
