export interface Pairing {
  code: string;
  expiresAt: string;
}

export interface CaptureSessionSnapshot {
  active: boolean;
  paired: boolean;
  expiresAt: string;
}

// These fields are intentionally safe to render. Source URLs and request
// headers are never accepted into the client capture store.
export interface CaptureStreamSummary {
  id: string;
  name: string;
  host: string;
  type: string;
  capturedAt: string;
}
