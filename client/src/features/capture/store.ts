import { defineStore } from 'pinia';
import { EMPTY_CAPTURE_SESSION } from './constants';
import type { CaptureSessionSnapshot, CaptureStreamSummary } from './types';

interface CaptureState {
  session: CaptureSessionSnapshot;
  streams: CaptureStreamSummary[];
}

export const useCaptureStore = defineStore('capture', {
  state: (): CaptureState => ({
    session: {
      ...EMPTY_CAPTURE_SESSION,
      mode: 'manual',
      nativeStatus: 'disconnected',
    },
    streams: [],
  }),
  actions: {
    setSession(session: CaptureSessionSnapshot): void {
      this.session = session;
      if (!session.active) {
        this.streams = [];
      }
    },
    setStreams(streams: CaptureStreamSummary[]): void {
      this.streams = streams;
    },
    addStream(stream: CaptureStreamSummary): void {
      const index = this.streams.findIndex((candidate) => candidate.id === stream.id);
      if (index === -1) {
        this.streams.push(stream);
        return;
      }

      this.streams.splice(index, 1, stream);
    },
  },
});

export type CaptureStore = ReturnType<typeof useCaptureStore>;
