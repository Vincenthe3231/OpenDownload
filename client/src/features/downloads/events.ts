import { EventsOn } from '../../../wailsjs/wailsjs/runtime/runtime';
import { WAILS_EVENTS } from '../../shared/constants';
import { listDownloadJobs, parseJobSnapshot } from './api';
import type { DownloadStore } from './store';

type EventSubscriber = (eventName: string, callback: (payload: unknown) => void) => () => void;

export interface DownloadEventSubscription {
  dispose(): void;
  hydrate(): Promise<void>;
}

export function subscribeToDownloadEvents(
  store: DownloadStore,
  subscribe: EventSubscriber = EventsOn,
): DownloadEventSubscription {
  const unsubscribe = subscribe(WAILS_EVENTS.downloadChanged, (payload) => {
    const snapshot = parseJobSnapshot(payload);
    if (snapshot) {
      store.apply(snapshot);
    }
  });

  return {
    dispose: unsubscribe,
    async hydrate(): Promise<void> {
      store.hydrate(await listDownloadJobs());
    },
  };
}
