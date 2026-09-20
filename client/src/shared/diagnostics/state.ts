export const developerDiagnosticsChangedEvent = 'opendownload:developer-diagnostics-changed';

export function publishDeveloperDiagnosticsChange(enabled: boolean): void {
  if (typeof window === 'undefined') return;

  window.dispatchEvent(new CustomEvent<boolean>(developerDiagnosticsChangedEvent, { detail: enabled }));
}

export function subscribeToDeveloperDiagnosticsChanges(handler: (enabled: boolean) => void): () => void {
  if (typeof window === 'undefined') return () => undefined;

  const listener = (event: Event) => {
    const enabled = (event as CustomEvent<boolean>).detail;
    if (typeof enabled === 'boolean') handler(enabled);
  };

  window.addEventListener(developerDiagnosticsChangedEvent, listener);
  return () => window.removeEventListener(developerDiagnosticsChangedEvent, listener);
}
