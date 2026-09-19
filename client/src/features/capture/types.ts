export interface Pairing {
  code: string;
  expiresAt: string;
}

export interface CaptureSessionSnapshot {
  active: boolean;
  paired: boolean;
  expiresAt: string;
  mode: 'automatic' | 'manual';
  nativeStatus: 'disconnected' | 'connecting' | 'connected';
  browser?: string;
  tabId?: number;
  diagnostic?: CaptureDiagnostic;
}

export interface CaptureDiagnostic {
  code: string;
  stage: 'native_connection' | 'capture' | 'download' | string;
  retryable: boolean;
  userMessage: string;
  diagnosticId: string;
  occurredAt: string;
  technicalDetail?: string;
  safeContext?: Record<string, string>;
}

export interface AutomaticCaptureFailure {
  code: string;
  userMessage: string;
  retryable: boolean;
}

export interface AutomaticCaptureStatus {
  hostExecutableFound: boolean;
  manifestExists: boolean;
  manifestPathMatchesInstall: boolean;
  manifestExtensionIdMatches: boolean;
  mozillaRegistryPointsToExpectedManifest: boolean;
  healthy: boolean;
  failure?: AutomaticCaptureFailure;
}

export interface TechnicalDiagnostic {
  diagnosticId: string;
  occurredAt: string;
  rawError?: string;
  requestId?: string;
  requestMethod?: string;
  requestType?: string;
  requestTimestamp?: number;
  requestFrameId?: number;
  requestParentFrameId?: number;
  requestUrl?: string;
  requestDocumentUrl?: string;
  requestOriginUrl?: string;
  requestInitiator?: string;
  requestHeaders?: Record<string, string>;
  responseHeaders?: Record<string, string>;
  responseStatusLine?: string;
  responseFromCache?: boolean;
  responseIp?: string;
  nativeHostDetail?: string;
  pipeDetail?: string;
  httpStatus?: number;
  truncated: boolean;
}

// These fields are intentionally safe to render. Source URLs and request
// headers are never accepted into the client capture store.
export interface CaptureStreamSummary {
  id: string;
  name: string;
  host: string;
  type: string;
  capturedAt: string;
  errorCode?: string;
  diagnosticId?: string;
}
