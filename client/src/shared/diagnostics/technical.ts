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

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null ? value as Record<string, unknown> : null;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

function numberValue(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function stringMapValue(value: unknown): Record<string, string> | undefined {
  const raw = asRecord(value);
  if (!raw) return undefined;

  const result: Record<string, string> = {};
  for (const [key, entry] of Object.entries(raw)) {
    if (typeof entry === 'string') result[key] = entry;
  }
  return Object.keys(result).length ? result : undefined;
}

export function parseTechnicalDiagnostic(value: unknown): TechnicalDiagnostic | null {
  const raw = asRecord(value);
  if (!raw || typeof raw.diagnosticId !== 'string' || typeof raw.occurredAt !== 'string') {
    return null;
  }

  return {
    diagnosticId: raw.diagnosticId,
    occurredAt: raw.occurredAt,
    rawError: stringValue(raw.rawError),
    requestId: stringValue(raw.requestId),
    requestMethod: stringValue(raw.requestMethod),
    requestType: stringValue(raw.requestType),
    requestTimestamp: numberValue(raw.requestTimestamp),
    requestFrameId: numberValue(raw.requestFrameId),
    requestParentFrameId: numberValue(raw.requestParentFrameId),
    requestUrl: stringValue(raw.requestUrl),
    requestDocumentUrl: stringValue(raw.requestDocumentUrl),
    requestOriginUrl: stringValue(raw.requestOriginUrl),
    requestInitiator: stringValue(raw.requestInitiator),
    requestHeaders: stringMapValue(raw.requestHeaders),
    responseHeaders: stringMapValue(raw.responseHeaders),
    responseStatusLine: stringValue(raw.responseStatusLine),
    responseFromCache: typeof raw.responseFromCache === 'boolean' ? raw.responseFromCache : undefined,
    responseIp: stringValue(raw.responseIp),
    nativeHostDetail: stringValue(raw.nativeHostDetail),
    pipeDetail: stringValue(raw.pipeDetail),
    httpStatus: typeof raw.httpStatus === 'number' ? raw.httpStatus : undefined,
    truncated: raw.truncated === true,
  };
}
