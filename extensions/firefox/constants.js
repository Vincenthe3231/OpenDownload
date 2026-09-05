const CaptureConstants = Object.freeze({
  allowedRequestHeaders: new Set([
    'accept',
    'accept-language',
    'authorization',
    'cookie',
    'origin',
    'referer',
    'user-agent',
  ]),
  fragmentExtensions: new Set(['cmfa', 'cmfv', 'm4f', 'm4s', 'ts']),
  directMediaExtensions: new Set(['aac', 'm4a', 'mkv', 'mov', 'mp3', 'mp4', 'opus', 'webm']),
  maxPendingRequests: 256,
});
