import { EventsOn } from '../../../wailsjs/wailsjs/runtime/runtime';
import { WAILS_EVENTS } from '../../shared/constants';
import { listCapturedStreams, parseCaptureSession, parseCaptureStreamSummary } from './api';
import type { CaptureStore } from './store';

type EventSubscriber = (eventName: string, callback: (payload: unknown) => void) => () => void;

export interface CaptureEventSubscription {
  dispose(): void;
  hydrate(): Promise<void>;
}

export function subscribeToCaptureEvents(
  store: CaptureStore,
  subscribe: EventSubscriber = EventsOn,
): CaptureEventSubscription {
  const unsubscribeSession = subscribe(WAILS_EVENTS.captureSessionChanged, (payload) => {
    const session = parseCaptureSession(payload);
    if (session) {
      store.setSession(session);
    }
  });
  const unsubscribeStream = subscribe(WAILS_EVENTS.captureStreamAdded, (payload) => {
    const stream = parseCaptureStreamSummary(payload);
    if (stream) {
      store.addStream(stream);
    }
  });

  return {
    dispose(): void {
      unsubscribeSession();
      unsubscribeStream();
    },
    async hydrate(): Promise<void> {
      store.setStreams(await listCapturedStreams());
    },
  };
}
