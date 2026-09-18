import { EventsOn } from '../../../wailsjs/wailsjs/runtime/runtime';
import { WAILS_EVENTS } from '../../shared/constants';
import { listCapturedStreams, parseCaptureStreamSummary } from './api';
import type { CaptureStore } from './store';
import type { CaptureSessionSnapshot } from './types';

type EventSubscriber = (eventName: string, callback: (payload: unknown) => void) => () => void;

export interface CaptureEventSubscription {
  dispose(): void;
  hydrate(): Promise<void>;
}

function parseCaptureSession(value: unknown): CaptureSessionSnapshot | null {
  if (typeof value !== 'object' || value === null) {
    return null;
  }

  const raw = value as Record<string, unknown>;
  if (typeof raw.active !== 'boolean' || typeof raw.paired !== 'boolean') {
    return null;
  }

	return {
		active: raw.active,
		paired: raw.paired,
		expiresAt: typeof raw.expiresAt === 'string' ? raw.expiresAt : '',
		mode: raw.mode === 'automatic' ? 'automatic' : 'manual',
		nativeStatus: raw.nativeStatus === 'connected' || raw.nativeStatus === 'connecting' ? raw.nativeStatus : 'disconnected',
		browser: typeof raw.browser === 'string' ? raw.browser : undefined,
		tabId: typeof raw.tabId === 'number' ? raw.tabId : undefined,
		diagnostic: typeof raw.diagnostic === 'object' && raw.diagnostic !== null ? raw.diagnostic as CaptureSessionSnapshot['diagnostic'] : undefined,
	};
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
